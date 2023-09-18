package vault_raft_snapshot_agent

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/app/vault_raft_snapshot_agent/config"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/app/vault_raft_snapshot_agent/upload"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/app/vault_raft_snapshot_agent/vault"
	"go.uber.org/multierr"
)

type SnapshotterConfig struct {
	Vault     vault.ClientConfig
	Snapshots SnapshotConfig
	Uploaders upload.UploadersConfig
}

func (c SnapshotterConfig) HasUploaders() bool {
	return !(c.Uploaders.AWS.Empty && c.Uploaders.Azure.Empty && c.Uploaders.GCP.Empty && c.Uploaders.Local.Empty)
}

type SnapshotConfig struct {
	Frequency       time.Duration `default:"1h"`
	Retain          int
	Timeout         time.Duration `default:"60s"`
	NamePrefix      string        `default:"raft-snapshot-"`
	NameSuffix      string        `default:".snap"`
	TimestampFormat string        `default:"2006-01-02T15-04-05Z-0700"`
}

type SnapshotterOptions struct {
	ConfigFileName        string
	ConfigFileSearchPaths []string
	ConfigFilePath        string
	EnvPrefix             string
}

type Snapshotter struct {
	lock          sync.Mutex
	client        snapshotterVaultAPI
	uploaders     []upload.Uploader
	config        SnapshotConfig
	lastSnapshot  time.Time
	snapshotTimer *time.Timer
}

type snapshotterVaultAPI interface {
	TakeSnapshot(ctx context.Context, writer io.Writer) error
}

func CreateSnapshotter(options SnapshotterOptions) (*Snapshotter, error) {
	data := SnapshotterConfig{}
	parser := config.NewParser[*SnapshotterConfig](options.ConfigFileName, options.EnvPrefix, options.ConfigFileSearchPaths...)

	if err := parser.ReadConfig(&data, options.ConfigFilePath); err != nil {
		return nil, err
	}

	snapshotter, err := createSnapshotter(data)
	if err != nil {
		return nil, err
	}

	parser.OnConfigChange(
		&SnapshotterConfig{},
		func(config *SnapshotterConfig) error {
			if err := snapshotter.reconfigure(*config); err != nil {
				log.Printf("could not reconfigure snapshotter: %s\n", err)
				return err
			}
			return nil
		},
	)

	return snapshotter, nil
}

func createSnapshotter(config SnapshotterConfig) (*Snapshotter, error) {
	snapshotter := &Snapshotter{}

	err := snapshotter.reconfigure(config)
	return snapshotter, err
}

func (s *Snapshotter) reconfigure(config SnapshotterConfig) error {
	client, err := vault.CreateClient(config.Vault)
	if err != nil {
		return err
	}

	s.Configure(config.Snapshots, client, upload.CreateUploaders(config.Uploaders))
	return nil
}

func (s *Snapshotter) Configure(config SnapshotConfig, client snapshotterVaultAPI, uploaders []upload.Uploader) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.client = client
	s.uploaders = uploaders
	s.config = config
	s.updateTimer(config.Frequency)
}

func (s *Snapshotter) TakeSnapshot(ctx context.Context) (*time.Timer, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.resetTimer()

	snapshot, err := os.CreateTemp("", "snapshot")
	if err != nil {
		return s.snapshotTimer, err
	}

	defer func() {
		if err := snapshot.Close(); err != nil {
			fmt.Printf("could not close snapshot-temp-file %s: %s\n", snapshot.Name(), err)
		} else if err := os.Remove(snapshot.Name()); err != nil {
			fmt.Printf("could not remove snapshot-temp-file %s: %s\n", snapshot.Name(), err)
		}
	}()

	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()

	err = s.client.TakeSnapshot(ctx, snapshot)
	if err != nil {
		return s.snapshotTimer, err
	}

	_, err = snapshot.Seek(0, io.SeekStart)
	if err != nil {
		return s.snapshotTimer, err
	}

	return s.snapshotTimer, s.uploadSnapshot(ctx, snapshot, time.Now().Format(s.config.TimestampFormat))
}

func (s *Snapshotter) uploadSnapshot(ctx context.Context, snapshot io.Reader, timestamp string) error {
	var errs error
	for _, uploader := range s.uploaders {
		err := uploader.Upload(ctx, snapshot, s.config.NamePrefix, timestamp, s.config.NameSuffix, s.config.Retain)
		if err != nil {
			errs = multierr.Append(errs, fmt.Errorf("unable to upload snapshot: %s", err))
		} else {
			log.Printf("Successfully uploaded snapshot to %s\n", uploader.Destination())
		}
	}

	return errs
}

func (s *Snapshotter) resetTimer() {
	s.lastSnapshot = time.Now()
	s.snapshotTimer = time.NewTimer(s.config.Frequency)
}

func (s *Snapshotter) updateTimer(frequency time.Duration) {
	if s.snapshotTimer != nil {
		if !s.snapshotTimer.Stop() {
			<-s.snapshotTimer.C
		}

		now := time.Now()
		timeout := time.Duration(0)

		nextSnapshot := s.lastSnapshot.Add(frequency)
		if nextSnapshot.After(now) {
			timeout = nextSnapshot.Sub(now)
		}

		s.snapshotTimer.Reset(timeout)
	}
}

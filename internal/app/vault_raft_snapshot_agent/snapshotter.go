package vault_raft_snapshot_agent

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/app/vault_raft_snapshot_agent/upload"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/app/vault_raft_snapshot_agent/vault"
	"go.uber.org/multierr"
)

type Snapshotter struct {
	lock            sync.Mutex
	client          *vault.VaultClient
	uploaders       []upload.Uploader
	frequency       time.Duration
	snapshotTimeout time.Duration
	retainSnapshots int
}


func CreateSnapshotter(config SnapshotterConfig) (*Snapshotter, error) {
	snapshotter := &Snapshotter{}

	err := snapshotter.Reconfigure(config)
	return snapshotter, err
}

func (s *Snapshotter) Reconfigure(config SnapshotterConfig) error {
	client, err := vault.CreateClient(config.Vault)
	if err != nil {
		return err
	}

	uploaders, err := upload.CreateUploaders(config.Uploaders)
	if err != nil {
		return err
	}

	s.Configure(config.Snapshots, client, uploaders)
	return nil
}

func (s *Snapshotter) Configure(config SnapshotConfig, client *vault.VaultClient, uploaders []upload.Uploader) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.frequency = config.Frequency
	s.client = client
	s.uploaders = uploaders
	s.snapshotTimeout = config.Timeout
	s.retainSnapshots = config.Retain
}

func (s *Snapshotter) TakeSnapshot(ctx context.Context) (time.Duration, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	snapshot, err := os.CreateTemp("", "snapshot")
	if err != nil {
		return s.frequency, err
	}

	defer os.Remove(snapshot.Name())

	ctx, cancel := context.WithTimeout(ctx, s.snapshotTimeout)
	defer cancel()

	err = s.client.TakeSnapshot(ctx, snapshot)
	if err != nil {
		return s.frequency, err
	}

	_, err = snapshot.Seek(0, io.SeekStart)
	if err != nil {
		return s.frequency, err
	}

	return s.frequency, s.uploadSnapshot(ctx, snapshot, time.Now())
}

func (s *Snapshotter) uploadSnapshot(ctx context.Context, snapshot io.Reader, time time.Time) error {
	var errs error
	for _, uploader := range s.uploaders {
		if err := uploader.Upload(ctx, snapshot, time, s.retainSnapshots); err != nil {
			errs = multierr.Append(errs, fmt.Errorf("unable to upload snapshot: %s", err))
		} else {
			log.Printf("Successfully uploaded snapshot to %s\n", uploader.Destination())		
		}
	}

	return errs
}

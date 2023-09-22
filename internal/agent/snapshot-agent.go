package agent

import (
	"context"
	"io"
	"os"
	"sync"
	"time"

	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/config"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/logging"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/storage"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/vault"
)

// SnapshotAgentConfig is the root of the agent-configuration
type SnapshotAgentConfig struct {
	Vault     vault.ClientConfig
	Snapshots SnapshotsConfig
}

// SnapshotsConfig configures where snapshots get stored and how often snapshots are made etc.
type SnapshotsConfig struct {
	storage.StorageConfigDefaults `mapstructure:",squash"`
	Storages                      storage.StoragesConfig
}

// SnapshotAgentOptions is a Parameter Object containing all parameters required by CreateSnapshotAgent
type SnapshotAgentOptions struct {
	ConfigFileName        string
	ConfigFileSearchPaths []string
	ConfigFilePath        string
	EnvPrefix             string
}

// SnapshotAgent implements the taking of snapshots from vault and uploading them to the storages
type SnapshotAgent struct {
	lock                  sync.Mutex
	client                snapshotAgentVaultAPI
	manager               snapshotManager
	tempDir               string
	storageConfigDefaults storage.StorageConfigDefaults
	lastSnapshotTime      time.Time
	snapshotTimer         *time.Timer
}

type snapshotAgentVaultAPI interface {
	TakeSnapshot(ctx context.Context, writer io.Writer) error
}

type snapshotManager interface {
	ScheduleSnapshot(ctx context.Context, lastSnapshot time.Time, defaults storage.StorageConfigDefaults) time.Time
	UploadSnapshot(ctx context.Context, snapshot io.ReadSeeker, timestamp time.Time, defaults storage.StorageConfigDefaults) time.Time
}

func (c SnapshotAgentConfig) HasStorages() bool {
	return c.Snapshots.HasStorages()
}

func (c SnapshotsConfig) HasStorages() bool {
	return !(c.Storages.AWS.Empty && c.Storages.Azure.Empty && c.Storages.GCP.Empty && c.Storages.Local.Empty)
}

func CreateSnapshotAgent(ctx context.Context, options SnapshotAgentOptions) (*SnapshotAgent, error) {
	data := SnapshotAgentConfig{}
	parser := config.NewParser[*SnapshotAgentConfig](options.EnvPrefix, options.ConfigFileName, options.ConfigFileSearchPaths...)

	if err := parser.ReadConfig(&data, options.ConfigFilePath); err != nil {
		return nil, err
	}

	agent, err := createSnapshotAgent(ctx, data)
	if err != nil {
		return nil, err
	}

	parser.OnConfigChange(
		&SnapshotAgentConfig{},
		func(config *SnapshotAgentConfig) error {
			if err := agent.reconfigure(ctx, *config); err != nil {
				logging.Warn("Could not reconfigure agent", "error", err)
				return err
			}
			return nil
		},
	)

	return agent, nil
}

func createSnapshotAgent(ctx context.Context, config SnapshotAgentConfig) (*SnapshotAgent, error) {
	agent := &SnapshotAgent{}

	err := agent.reconfigure(ctx, config)
	return agent, err
}

func (a *SnapshotAgent) reconfigure(ctx context.Context, config SnapshotAgentConfig) error {
	client, err := vault.CreateClient(config.Vault)
	if err != nil {
		return err
	}

	manager, err := storage.CreateManager(ctx, config.Snapshots.Storages)
	if err != nil {
		return err
	}

	a.update(ctx, client, manager, config.Snapshots.StorageConfigDefaults)
	return nil
}

func (a *SnapshotAgent) update(ctx context.Context, client snapshotAgentVaultAPI, manager snapshotManager, defaults storage.StorageConfigDefaults) {
	a.lock.Lock()
	defer a.lock.Unlock()

	a.client = client
	a.manager = manager
	a.storageConfigDefaults = defaults
	a.snapshotTimer = time.NewTimer(0)
	a.updateTimer(manager.ScheduleSnapshot(ctx, a.lastSnapshotTime, a.storageConfigDefaults))
}

func (a *SnapshotAgent) TakeSnapshot(ctx context.Context) *time.Timer {
	a.lock.Lock()
	defer a.lock.Unlock()

	a.lastSnapshotTime = time.Now()
	// ensure that we do not hammer on vault in case of errors
	a.updateTimer(a.lastSnapshotTime.Add(a.storageConfigDefaults.Frequency))

	snapshot, err := os.CreateTemp(a.tempDir, "snapshot")
	if err != nil {
		logging.Warn("Could not create snapshot-temp-file", "file", "error", err)
		return a.snapshotTimer
	}

	defer func() {
		if err := snapshot.Close(); err != nil {
			logging.Warn("Could not close snapshot-temp-file", "file", snapshot.Name(), "error", err)
		} else if err := os.Remove(snapshot.Name()); err != nil {
			logging.Warn("Could not remove snapshot-temp-file %a: %a", "file", snapshot.Name(), "error", err)
		}
	}()

	err = a.client.TakeSnapshot(ctx, snapshot)
	if err != nil {
		logging.Error("Could not take snapshot of vault", "error", err)
		return a.snapshotTimer
	}

	info, err := snapshot.Stat()
	if err != nil {
		logging.Error("Could not stat snapshot-temp-file", "file", snapshot.Name(), "error", err)
		return a.snapshotTimer
	}

	if info.Size() < 1 {
		logging.Warn("Ignoring empty snapshot", "file", snapshot.Name())
		return a.snapshotTimer
	}

	nextSnapshot := a.manager.UploadSnapshot(ctx, snapshot, a.lastSnapshotTime, a.storageConfigDefaults)
	return a.updateTimer(nextSnapshot)
}

func (a *SnapshotAgent) updateTimer(nextSnapshot time.Time) *time.Timer {
	if !nextSnapshot.IsZero() {
		now := time.Now()
		timeout := time.Duration(0)

		if nextSnapshot.After(now) {
			timeout = nextSnapshot.Sub(now)
		}

		if !a.snapshotTimer.Stop() {
			<-a.snapshotTimer.C
		}

		a.snapshotTimer.Reset(timeout)
	}
	return a.snapshotTimer
}

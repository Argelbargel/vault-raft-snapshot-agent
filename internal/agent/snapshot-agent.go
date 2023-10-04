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
	Vault     vault.VaultClientConfig
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
	snapshotTicker        *time.Ticker
}

type snapshotAgentVaultAPI interface {
	TakeSnapshot(ctx context.Context, writer io.Writer) error
}

type snapshotManager interface {
	ScheduleSnapshot(ctx context.Context, lastSnapshot time.Time, defaults storage.StorageConfigDefaults) time.Time
	UploadSnapshot(ctx context.Context, snapshot io.ReadSeeker, snapshotSize int64, timestamp time.Time, defaults storage.StorageConfigDefaults) time.Time
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
	agent := newSnapshotAgent("")
	err := agent.reconfigure(ctx, config)
	return agent, err
}

func newSnapshotAgent(tempDir string) *SnapshotAgent {
	return &SnapshotAgent{
		snapshotTicker: time.NewTicker(time.Hour),
		tempDir:        tempDir,
	}
}

func (a *SnapshotAgent) reconfigure(ctx context.Context, config SnapshotAgentConfig) error {
	client, err := vault.CreateClient(config.Vault)
	if err != nil {
		return err
	}

	a.update(ctx, client, storage.CreateManager(config.Snapshots.Storages), config.Snapshots.StorageConfigDefaults)
	return nil
}

func (a *SnapshotAgent) update(ctx context.Context, client snapshotAgentVaultAPI, manager snapshotManager, defaults storage.StorageConfigDefaults) {
	a.lock.Lock()
	defer a.lock.Unlock()

	a.client = client
	a.manager = manager
	a.storageConfigDefaults = defaults
	nextSnapshot := manager.ScheduleSnapshot(ctx, a.lastSnapshotTime, a.storageConfigDefaults)
	a.updateTicker(nextSnapshot)
	logging.Debug("Successfully updated configuration", "nextSnapshot", nextSnapshot)
}

func (a *SnapshotAgent) TakeSnapshot(ctx context.Context) *time.Ticker {
	a.lock.Lock()
	defer a.lock.Unlock()

	a.lastSnapshotTime = time.Now()
	// ensure that we do not hammer on vault in case of errors
	nextSnapshot := a.lastSnapshotTime.Add(a.storageConfigDefaults.Frequency)
	a.updateTicker(nextSnapshot)

	snapshot, err := os.CreateTemp(a.tempDir, "snapshot")
	if err != nil {
		logging.Warn("Could not create snapshot-temp-file", "nextSnapshot", nextSnapshot, "error", err)
		return a.snapshotTicker
	}

	defer func() {
		if err := snapshot.Close(); err != nil {
			logging.Warn("Could not close snapshot-temp-file", "file", snapshot.Name(), "nextSnapshot", nextSnapshot, "error", err)
		} else if err := os.Remove(snapshot.Name()); err != nil {
			logging.Warn("Could not remove snapshot-temp-file %a: %a", "file", snapshot.Name(), "nextSnapshot", nextSnapshot, "error", err)
		}
	}()

	err = a.client.TakeSnapshot(ctx, snapshot)
	if err != nil {
		logging.Error("Could not take snapshot of vault", "nextSnapshot", nextSnapshot, "error", err)
		return a.snapshotTicker
	}

	info, err := snapshot.Stat()
	if err != nil {
		logging.Error("Could not stat snapshot-temp-file", "file", snapshot.Name(), "nextSnapshot", nextSnapshot, "error", err)
		return a.snapshotTicker
	}

	if info.Size() < 1 {
		logging.Warn("Ignoring empty snapshot", "file", snapshot.Name(), "nextSnapshot", nextSnapshot)
		return a.snapshotTicker
	}

	nextSnapshot = a.manager.UploadSnapshot(ctx, snapshot, 0, a.lastSnapshotTime, a.storageConfigDefaults)
	return a.updateTicker(nextSnapshot)
}

func (a *SnapshotAgent) updateTicker(nextSnapshot time.Time) *time.Ticker {
	if !nextSnapshot.IsZero() {
		now := time.Now()

		if nextSnapshot.After(now) {
			timeout := nextSnapshot.Sub(now)
			a.snapshotTicker.Reset(timeout)
		}
	}
	return a.snapshotTicker
}

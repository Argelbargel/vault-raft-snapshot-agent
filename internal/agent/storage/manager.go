package storage

import (
	"context"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/logging"
	"go.uber.org/multierr"
	"io"
	"time"
)

// Manager manages the upload of a snapshot to one or multiple storage-locations
// using StorageController-instances configured by StoragesConfig
type Manager struct {
	factories []StorageControllerFactory
}

type StorageControllerFactory interface {
	// CreateController creates a new controller connection to a storage location
	CreateController(context.Context) (StorageController, error)
	// Destination returns information about the location of the controlled storage
	Destination() string
}

// StorageController defines the interface required by the Manager to communicate with storageControllerImpl-instances
type StorageController interface {
	// ScheduleSnapshot schedules the next upload to the controlled storage.
	// If the time of the last upload can not be determined by the controller,
	// it may use the time of the last snapshot given as fallback
	// For the case that the StorageControllerConfig of the controller does not specify one of its fields,
	// StorageConfigDefaults is passed.
	ScheduleSnapshot(ctx context.Context, lastSnapshot time.Time, defaults StorageConfigDefaults) (time.Time, error)
	// UploadSnapshot uploads the given snapshot to the controlled storage, if the timestamp of the snapshot
	// corresponds with its scheduled upload-date.
	// For the case that the StorageControllerConfig of the controller does not specify one of its fields,
	// StorageConfigDefaults is passed.
	UploadSnapshot(ctx context.Context, snapshot io.Reader, snapshotSize int64, timestamp time.Time, defaults StorageConfigDefaults) (bool, time.Time, error)
	DeleteObsoleteSnapshots(ctx context.Context, defaults StorageConfigDefaults) (int, error)
}

// CreateManager creates a Manager controlling the StorageController-instances
// configured according to the given StoragesConfig and StorageConfigDefaults
func CreateManager(storageConfig StoragesConfig) *Manager {
	manager := &Manager{}

	if storageConfig.AWS != nil {
		manager.AddStorageFactory(storageConfig.AWS)
	}
	if storageConfig.Azure != nil {
		manager.AddStorageFactory(storageConfig.Azure)
	}
	if storageConfig.GCP != nil {
		manager.AddStorageFactory(storageConfig.GCP)
	}
	if storageConfig.Local != nil {
		manager.AddStorageFactory(storageConfig.Local)
	}
	if storageConfig.Swift != nil {
		manager.AddStorageFactory(storageConfig.Swift)
	}
	if storageConfig.S3 != nil {
		manager.AddStorageFactory(storageConfig.S3)
	}

	return manager
}

// AddStorageFactory adds a StorageController to the manager
// Allows adding of StorageController-implementations for testing
func (m *Manager) AddStorageFactory(factory StorageControllerFactory) {
	m.factories = append(m.factories, factory)
}

// ScheduleSnapshot schedules the next snapshot.
// Scheduling of snapshot is delegated to the StorageController-instances; the earliest time calculated by all
// factories is returned. The given time when the last snapshot was taken is passed on to the factories as fallback
// if the time of the last upload cannot be determined
func (m *Manager) ScheduleSnapshot(ctx context.Context, lastSnapshotTime time.Time, defaults StorageConfigDefaults) time.Time {
	nextSnapshot := time.Time{}

	for _, factory := range m.factories {
		controller, err := factory.CreateController(ctx)
		if err != nil {
			logging.Warn("Could not create controller", "destination", factory.Destination(), "error", err)
		} else {
			candidate, err := controller.ScheduleSnapshot(ctx, lastSnapshotTime, defaults)
			if err != nil {
				logging.Warn("Could not schedule snapshot", "destination", factory.Destination(), "error", err)
			} else if nextSnapshot.IsZero() || candidate.Before(nextSnapshot) {
				nextSnapshot = candidate
			}
		}
	}

	return nextSnapshot
}

// UploadSnapshot uploads the given snapshot to all storages controlled by the StorageController-instances
// and returns the time the next snapshot should be taken.
// Whether the snapshot is actually uploaded to a storage is controlled by the StorageController based
// on the upload-frequency in its StoragesConfig
func (m *Manager) UploadSnapshot(ctx context.Context, snapshot io.ReadSeeker, snapshotSize int64, timestamp time.Time, defaults StorageConfigDefaults) time.Time {
	var (
		nextSnapshot time.Time
		errs         error
	)

	for _, factory := range m.factories {
		if _, err := snapshot.Seek(0, io.SeekStart); err != nil {
			logging.Error("Could not reset snapshot before uploading", "error", err)
			return timestamp.Add(defaults.Frequency)
		}

		controller, err := factory.CreateController(ctx)
		if err != nil {
			logging.Warn("Could not create storage-controller", "destination", factory.Destination(), "error", err)
			errs = multierr.Append(errs, err)
		} else {
			uploaded, candidate, err := controller.UploadSnapshot(ctx, snapshot, snapshotSize, timestamp, defaults)
			if nextSnapshot.IsZero() || candidate.Before(nextSnapshot) {
				nextSnapshot = candidate
			}

			if err != nil {
				logging.Warn("Could not upload snapshot", "destination", factory.Destination(), "error", err, "nextSnapshot", candidate)
				errs = multierr.Append(errs, err)
			} else if !uploaded {
				logging.Debug("Skipped upload of snapshot", "destination", factory.Destination(), "nextSnapshot", candidate)
			} else {
				logging.Debug("Successfully uploaded snapshot", "destination", factory.Destination(), "nextSnapshot", candidate)

				deleted, err := controller.DeleteObsoleteSnapshots(ctx, defaults)
				if err != nil {
					logging.Warn("Could not delete obsolete snapshots", "destination", factory.Destination(), "error", err)
				} else if deleted > 0 {
					logging.Debug("Deleted obsolete snapshots", "destination", factory.Destination(), "deleted", deleted)
				}
			}
		}
	}

	if errs == nil {
		logging.Info("Successfully uploaded snapshot to all scheduled destinations", "nextSnapshot", nextSnapshot)
	}

	return nextSnapshot
}

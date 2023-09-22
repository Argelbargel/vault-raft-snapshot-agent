package storage

import (
	"context"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/logging"
	"go.uber.org/multierr"
	"io"
	"time"
)

// Manager manages the upload of a snapshot to one or multiple storage-locations
// using storageController-instances configured by StoragesConfig
type Manager struct {
	controllers []storageController
}

// storageController defines the interface required by the Manager to communicate with storageControllerImpl-instances
type storageController interface {
	// Destination returns information about the location of the controlled storage
	Destination() string
	// ScheduleSnapshot schedules the next upload to the controlled storage.
	// If the time of the last upload can not be determined by the controller,
	// it may use the time of the last snapshot given as fallback
	// For the case that the storageConfig of the controller does not specify one of its fields,
	// StorageConfigDefaults is passed.
	ScheduleSnapshot(ctx context.Context, lastSnapshot time.Time, defaults StorageConfigDefaults) time.Time
	// UploadSnapshot uploads the given snapshot to the controlled storage, if the timestamp of the snapshot
	// corresponds with its scheduled upload-date.
	// For the case that the storageConfig of the controller does not specify one of its fields,
	// StorageConfigDefaults is passed.
	UploadSnapshot(ctx context.Context, snapshot io.Reader, timestamp time.Time, defaults StorageConfigDefaults) (bool, time.Time, error)
	DeleteObsoleteSnapshots(ctx context.Context, defaults StorageConfigDefaults) (int, error)
}

// CreateManager creates a Manager controlling the storageController-instances
// configured according to the given StoragesConfig and StorageConfigDefaults
func CreateManager(ctx context.Context, storageConfig StoragesConfig) (*Manager, error) {
	manager := &Manager{}

	if !storageConfig.AWS.Empty {
		aws, err := createAWSStorageController(ctx, storageConfig.AWS)
		if err != nil {
			return nil, err
		}
		manager.AddStorage(aws)
	}
	if !storageConfig.Azure.Empty {
		azure, err := createAzureStorageController(ctx, storageConfig.Azure)
		if err != nil {
			return nil, err
		}
		manager.AddStorage(azure)
	}
	if !storageConfig.GCP.Empty {
		gcp, err := createGCPStorageController(ctx, storageConfig.GCP)
		if err != nil {
			return nil, err
		}
		manager.AddStorage(gcp)
	}
	if !storageConfig.Local.Empty {
		local, err := createLocalStorageController(ctx, storageConfig.Local)
		if err != nil {
			return nil, err
		}

		manager.AddStorage(local)
	}
	if !storageConfig.Swift.Empty {
		swift, err := createSwiftStorageController(ctx, storageConfig.Swift)
		if err != nil {
			return nil, err
		}
		manager.AddStorage(swift)
	}

	return manager, nil
}

// AddStorage adds a storageController to the manager
// Allows adding of storageController-implementations for testing
func (m *Manager) AddStorage(controller storageController) {
	m.controllers = append(m.controllers, controller)
}

// ScheduleSnapshot schedules the next snapshot.
// Scheduling of snapshot is delegated to the storageController-instances; the earliest time calculated by all
// controllers is returned. The given time when the last snapshot was taken is passed on to the controllers as fallback
// if the time of the last upload cannot be determined
func (m *Manager) ScheduleSnapshot(ctx context.Context, lastSnapshotTime time.Time, defaults StorageConfigDefaults) time.Time {
	nextSnapshot := time.Time{}

	for _, uploader := range m.controllers {
		candidate := uploader.ScheduleSnapshot(ctx, lastSnapshotTime, defaults)
		if nextSnapshot.IsZero() || candidate.Before(nextSnapshot) {
			nextSnapshot = candidate
		}
	}

	return nextSnapshot
}

// UploadSnapshot uploads the given snapshot to all storages controlled by the storageController-instances
// and returns the time the next snapshot should be taken.
// Whether the snapshot is actually uploaded to a storage is controlled by the storageController based
// on the upload-frequency in its StoragesConfig
func (m *Manager) UploadSnapshot(ctx context.Context, snapshot io.ReadSeeker, timestamp time.Time, defaults StorageConfigDefaults) time.Time {
	var (
		nextSnapshot time.Time
		errs         error
	)

	for _, controller := range m.controllers {
		if _, err := snapshot.Seek(0, io.SeekStart); err != nil {
			logging.Error("Could not reset snapshot before uploading", "error", err)
			return timestamp.Add(defaults.Frequency)
		}

		uploaded, candidate, err := controller.UploadSnapshot(ctx, snapshot, timestamp, defaults)
		if nextSnapshot.IsZero() || candidate.Before(nextSnapshot) {
			nextSnapshot = candidate
		}

		if err != nil {
			logging.Warn("Could not upload snapshot", "destination", controller.Destination(), "error", err, "nextSnapshot", candidate)
			errs = multierr.Append(errs, err)
		} else if !uploaded {
			logging.Debug("Skipped upload of snapshot", "destination", controller.Destination(), "nextSnapshot", candidate)
		} else {
			logging.Debug("Successfully uploaded snapshot", "destination", controller.Destination(), "nextSnapshot", candidate)

			deleted, err := controller.DeleteObsoleteSnapshots(ctx, defaults)
			if err != nil {
				logging.Warn("Could not delete obsolete snapshots", "destination", controller.Destination(), "error", err)
			} else if deleted > 0 {
				logging.Debug("Deleted obsolete snapshots", "destination", controller.Destination(), "deleted", deleted)
			}
		}
	}

	if errs == nil {
		logging.Info("Successfully uploaded snapshot to all scheduled destinations", "nextSnapshot", nextSnapshot)
	}

	return nextSnapshot
}

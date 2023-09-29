package storage

import (
	"context"
	"errors"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/logging"
	"io"
	"slices"
	"strings"
	"time"
)

// storageControllerImpl implements StorageController.
// Access to the storage-location is delegated to the given storage.
// Options like upload-frequency, snapshot-naming are configured by the given storageControllerConfig.
type storageControllerImpl[S any] struct {
	config     storageControllerConfig
	storage    storage[S]
	lastUpload time.Time
}

// storage defines the interface used by storageControllerImpl to access a storage-location
type storage[S any] interface {
	uploadSnapshot(ctx context.Context, name string, data io.Reader, size int64) error
	deleteSnapshot(ctx context.Context, snapshot S) error
	listSnapshots(ctx context.Context, prefix string, suffix string) ([]S, error)
	getLastModifiedTime(snapshot S) time.Time
}

// storageControllerConfig defines the interface used by storageControllerImpl to determine its required configuration
// options either from the storageConfig for the controlled storage or the global StorageConfigDefaults for all storages
type storageControllerConfig interface {
	frequencyOrDefault(StorageConfigDefaults) time.Duration
	retainOrDefault(StorageConfigDefaults) int
	timeoutOrDefault(StorageConfigDefaults) time.Duration
	namePrefixOrDefault(StorageConfigDefaults) string
	nameSuffixOrDefault(StorageConfigDefaults) string
	timestampFormatOrDefault(StorageConfigDefaults) string
}

// newStorageController creates a new storageControllerImpl uploading snapshots to the
// given storage configured according to the given storageControllerConfig
func newStorageController[S any](config storageControllerConfig, storage storage[S]) *storageControllerImpl[S] {
	return &storageControllerImpl[S]{
		config:  config,
		storage: storage,
	}
}

func (u *storageControllerImpl[S]) ScheduleSnapshot(ctx context.Context, lastSnapshotTime time.Time, defaults StorageConfigDefaults) (time.Time, error) {
	if err := u.ensureLastUploadTime(ctx, lastSnapshotTime, defaults); err != nil {
		return time.Time{}, err
	}

	return u.lastUpload.Add(u.config.frequencyOrDefault(defaults)), nil
}

func (u *storageControllerImpl[S]) UploadSnapshot(ctx context.Context, snapshot io.Reader, snapshotSize int64, timestamp time.Time, defaults StorageConfigDefaults) (bool, time.Time, error) {
	frequency := u.config.frequencyOrDefault(defaults)

	if timestamp.Before(u.lastUpload.Add(frequency)) {
		nextSnapshot, err := u.ScheduleSnapshot(ctx, timestamp, defaults)
		return false, nextSnapshot, err
	}

	ctx, cancel := context.WithTimeout(ctx, u.config.timeoutOrDefault(defaults))
	defer cancel()

	nextSnapshot := timestamp.Add(frequency)

	prefix := u.config.namePrefixOrDefault(defaults)
	suffix := u.config.nameSuffixOrDefault(defaults)
	ts := timestamp.Format(u.config.timestampFormatOrDefault(defaults))
	snapshotName := strings.Join([]string{prefix, ts, suffix}, "")

	if err := u.storage.uploadSnapshot(ctx, snapshotName, snapshot, snapshotSize); err != nil {
		return false, nextSnapshot, err
	}

	u.lastUpload = timestamp

	return true, nextSnapshot, nil
}

func (u *storageControllerImpl[S]) DeleteObsoleteSnapshots(ctx context.Context, defaults StorageConfigDefaults) (int, error) {
	retain := u.config.retainOrDefault(defaults)
	if retain < 1 {
		return 0, nil
	}

	ctx, cancel := context.WithTimeout(ctx, u.config.timeoutOrDefault(defaults))
	defer cancel()

	snapshots, err := u.listSnapshots(ctx, u.config.namePrefixOrDefault(defaults), u.config.nameSuffixOrDefault(defaults))
	if err != nil {
		return 0, err
	}

	if len(snapshots) <= retain {
		return 0, err
	}

	deleted := 0
	for _, s := range snapshots[retain:] {
		if err := u.storage.deleteSnapshot(ctx, s); err != nil {
			logging.Warn("Could not delete snapshot", "snapshot", s, "error", err)
		} else {
			deleted++
		}
	}

	return deleted, nil
}

func (u *storageControllerImpl[S]) listSnapshots(ctx context.Context, prefix string, suffix string) ([]S, error) {
	snapshots, err := u.storage.listSnapshots(ctx, prefix, suffix)
	if err != nil {
		return nil, err
	}

	slices.SortFunc(snapshots, func(a, b S) int {
		return u.storage.getLastModifiedTime(a).Compare(u.storage.getLastModifiedTime(b))
	})

	return snapshots, nil
}

func (u *storageControllerImpl[S]) ensureLastUploadTime(ctx context.Context, lastSnapshotTime time.Time, defaults StorageConfigDefaults) error {
	if u.lastUpload.IsZero() {
		if !lastSnapshotTime.IsZero() {
			u.lastUpload = lastSnapshotTime
		} else {
			storageLastModified, err := u.getLastModificationTime(ctx, defaults)
			if err != nil {
				return err
			}
			u.lastUpload = storageLastModified
		}
	}
	return nil
}

func (u *storageControllerImpl[S]) getLastModificationTime(ctx context.Context, defaults StorageConfigDefaults) (time.Time, error) {
	ctx, cancel := context.WithTimeout(ctx, u.config.timeoutOrDefault(defaults))
	defer cancel()

	snapshots, err := u.storage.listSnapshots(ctx, u.config.namePrefixOrDefault(defaults), u.config.nameSuffixOrDefault(defaults))
	if err != nil {
		return u.lastUpload, err
	}

	if len(snapshots) < 1 {
		return u.lastUpload, errors.New("storage does not contain any snapshots")
	}

	u.lastUpload = u.storage.getLastModifiedTime(snapshots[0])
	return u.lastUpload, nil
}

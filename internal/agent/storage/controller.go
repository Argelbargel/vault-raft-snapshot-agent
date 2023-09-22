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

// storageControllerImpl implements storageController.
// Access to the storage-location is delegated to the given storage.
// Options like upload-frequency, snapshot-naming are configured by the given storageControllerConfig.
type storageControllerImpl[S any] struct {
	config      storageControllerConfig
	destination string
	storage     storage[S]
	lastUpload  time.Time
}

// storage defines the interface used by storageControllerImpl to access a storage-location
type storage[S any] interface {
	UploadSnapshot(ctx context.Context, name string, data io.Reader) error
	DeleteSnapshot(ctx context.Context, snapshot S) error
	ListSnapshots(ctx context.Context, prefix string, suffix string) ([]S, error)
	GetLastModifiedTime(snapshot S) time.Time
}

// storageControllerConfig defines the interface used by storageControllerImpl to determine its required configuration
// options either from the storageConfig for the controlled storage or the global StorageConfigDefaults for all storages
type storageControllerConfig interface {
	FrequencyOrDefault(StorageConfigDefaults) time.Duration
	RetainOrDefault(StorageConfigDefaults) int
	TimeoutOrDefault(StorageConfigDefaults) time.Duration
	NamePrefixOrDefault(StorageConfigDefaults) string
	NameSuffixOrDefault(StorageConfigDefaults) string
	TimestampFormatOrDefault(StorageConfigDefaults) string
}

// newStorageController creates a new storageControllerImpl uploading snapshots to the
// given storage configured according to the given storageControllerConfig
func newStorageController[S any](config storageControllerConfig, destination string, storage storage[S]) *storageControllerImpl[S] {
	return &storageControllerImpl[S]{
		config:      config,
		destination: destination,
		storage:     storage,
	}
}

func (u *storageControllerImpl[S]) Destination() string {
	return u.destination
}

func (u *storageControllerImpl[S]) ScheduleSnapshot(ctx context.Context, lastSnapshotTime time.Time, defaults StorageConfigDefaults) time.Time {
	if err := u.ensureLastUploadTime(ctx, lastSnapshotTime, defaults); err != nil {
		logging.Warn("Could not schedule snapshot", "destination", u.destination, "error", err)
		return time.Time{}
	}

	return u.lastUpload.Add(u.config.FrequencyOrDefault(defaults))
}

func (u *storageControllerImpl[S]) UploadSnapshot(ctx context.Context, snapshot io.Reader, timestamp time.Time, defaults StorageConfigDefaults) (bool, time.Time, error) {
	frequency := u.config.FrequencyOrDefault(defaults)

	if timestamp.Before(u.lastUpload.Add(frequency)) {
		return false, u.ScheduleSnapshot(ctx, timestamp, defaults), nil
	}

	ctx, cancel := context.WithTimeout(ctx, u.config.TimeoutOrDefault(defaults))
	defer cancel()

	nextSnapshot := timestamp.Add(frequency)

	prefix := u.config.NamePrefixOrDefault(defaults)
	suffix := u.config.NameSuffixOrDefault(defaults)
	ts := timestamp.Format(u.config.TimestampFormatOrDefault(defaults))
	snapshotName := strings.Join([]string{prefix, ts, suffix}, "")

	if err := u.storage.UploadSnapshot(ctx, snapshotName, snapshot); err != nil {
		return false, nextSnapshot, err
	}

	u.lastUpload = timestamp

	return true, nextSnapshot, nil
}

func (u *storageControllerImpl[S]) DeleteObsoleteSnapshots(ctx context.Context, defaults StorageConfigDefaults) (int, error) {
	retain := u.config.RetainOrDefault(defaults)
	if retain < 1 {
		return 0, nil
	}

	ctx, cancel := context.WithTimeout(ctx, u.config.TimeoutOrDefault(defaults))
	defer cancel()

	snapshots, err := u.listSnapshots(ctx, u.config.NamePrefixOrDefault(defaults), u.config.NameSuffixOrDefault(defaults))
	if err != nil {
		return 0, err
	}

	if len(snapshots) <= retain {
		return 0, err
	}

	deleted := 0
	for _, s := range snapshots[retain:] {
		if err := u.storage.DeleteSnapshot(ctx, s); err != nil {
			logging.Warn("Could not delete snapshot", "snapshot", s, "destination", u.destination, "error", err)
		} else {
			deleted++
		}
	}

	return deleted, nil
}

func (u *storageControllerImpl[S]) listSnapshots(ctx context.Context, prefix string, suffix string) ([]S, error) {
	snapshots, err := u.storage.ListSnapshots(ctx, prefix, suffix)
	if err != nil {
		return nil, err
	}

	slices.SortFunc(snapshots, func(a, b S) int {
		return u.storage.GetLastModifiedTime(a).Compare(u.storage.GetLastModifiedTime(b))
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
	ctx, cancel := context.WithTimeout(ctx, u.config.TimeoutOrDefault(defaults))
	defer cancel()

	snapshots, err := u.storage.ListSnapshots(ctx, u.config.NamePrefixOrDefault(defaults), u.config.NameSuffixOrDefault(defaults))
	if err != nil {
		return u.lastUpload, err
	}

	if len(snapshots) < 1 {
		return u.lastUpload, errors.New("storage does not contain any snapshots")
	}

	u.lastUpload = u.storage.GetLastModifiedTime(snapshots[0])
	return u.lastUpload, nil
}

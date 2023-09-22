package storage

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"io"
	"strings"
	"testing"
	"time"
)

func TestDestination(t *testing.T) {
	controller := &storageControllerImpl[time.Time]{
		destination: "test",
	}

	assert.Equal(t, controller.destination, controller.Destination())
}

func TestScheduleSnapshotPrefersLastUploadTimeAndStorageConfig(t *testing.T) {
	lastUploadTime := time.Now()
	config := storageConfigStub{
		storageConfig{
			Frequency: time.Minute,
		},
	}

	controller := &storageControllerImpl[time.Time]{
		config:     config,
		lastUpload: lastUploadTime,
	}

	assert.Equal(t, lastUploadTime.Add(config.Frequency), controller.ScheduleSnapshot(context.Background(), time.Time{}, StorageConfigDefaults{}))
}

func TestScheduleSnapshotFallsBackOnStorageConfigDefaults(t *testing.T) {
	lastUploadTime := time.Now()
	defaults := StorageConfigDefaults{
		Frequency: time.Minute,
	}

	controller := &storageControllerImpl[time.Time]{
		config:     storageConfigStub{},
		lastUpload: lastUploadTime,
	}

	assert.Equal(t, lastUploadTime.Add(defaults.Frequency), controller.ScheduleSnapshot(context.Background(), time.Time{}, defaults))
}

func TestScheduleSnapshotFallsBackOnLastSnapshotTime(t *testing.T) {
	lastSnapshotTime := time.Now()
	config := storageConfigStub{
		storageConfig{
			Frequency: time.Minute,
		},
	}

	controller := &storageControllerImpl[time.Time]{
		config: config,
	}

	assert.Equal(t, lastSnapshotTime.Add(config.Frequency), controller.ScheduleSnapshot(context.Background(), lastSnapshotTime, StorageConfigDefaults{}))
}

func TestScheduleSnapshotFallsBackOnStorageLastModifiedTime(t *testing.T) {
	storageLastModifiedTime := time.Now()
	config := storageConfigStub{
		storageConfig{
			Frequency: time.Minute,
		},
	}

	storage := &storageStub{
		snapshots: []time.Time{storageLastModifiedTime},
	}

	controller := &storageControllerImpl[time.Time]{
		config:  config,
		storage: storage,
	}

	assert.Equal(t, storageLastModifiedTime.Add(config.Frequency), controller.ScheduleSnapshot(context.Background(), time.Time{}, StorageConfigDefaults{}))
}

func TestScheduleSnapshotReturnsZeroIfNoFallbackPossible(t *testing.T) {
	config := storageConfigStub{
		storageConfig{
			Frequency: time.Minute,
		},
	}

	storage := &storageStub{}

	controller := &storageControllerImpl[time.Time]{
		config:  config,
		storage: storage,
	}

	assert.Zero(t, controller.ScheduleSnapshot(context.Background(), time.Time{}, StorageConfigDefaults{}))
}

func TestUploadSnapshotUploadsToStorage(t *testing.T) {
	config := storageConfigStub{
		storageConfig{
			Frequency:       time.Minute,
			NamePrefix:      "test",
			NameSuffix:      ".test",
			TimestampFormat: "2006-01-02T15-04-05Z",
			Timeout:         time.Millisecond * 500,
		},
	}

	storage := &storageStub{}
	controller := &storageControllerImpl[time.Time]{
		config:  config,
		storage: storage,
	}

	ctx := context.Background()
	data := "test"
	timestamp := time.Now()
	start := time.Now()
	uploaded, nextSnapshot, err := controller.UploadSnapshot(
		ctx,
		strings.NewReader(data),
		timestamp,
		StorageConfigDefaults{},
	)
	assert.NoError(t, err, "UploadSnapshot failed unexpectedly")

	assert.True(t, uploaded)
	assert.Equal(t, timestamp.Add(config.Frequency), nextSnapshot)

	expectedDeadline, _ := storage.uploadContext.Deadline()
	expectedName := strings.Join([]string{config.NamePrefix, timestamp.Format(config.TimestampFormat), config.NameSuffix}, "")

	assert.Equal(t, start.Add(config.Timeout), expectedDeadline)
	assert.Equal(t, expectedName, storage.uploadName)
	assert.Equal(t, data, storage.uploadData)
}

func TestDeletesObsoleteSnapshots(t *testing.T) {
	config := storageConfigStub{
		storageConfig{
			Retain:     2,
			NamePrefix: "test",
			NameSuffix: ".text",
		},
	}

	now := time.Now()
	storage := &storageStub{
		snapshots: []time.Time{now.Add(time.Minute), now.Add(time.Second), now.Add(time.Hour), now.Add(time.Second * 2)},
	}
	controller := &storageControllerImpl[time.Time]{
		config:  config,
		storage: storage,
	}

	deleted, err := controller.DeleteObsoleteSnapshots(context.Background(), StorageConfigDefaults{})
	assert.NoError(t, err, "UploadSnapshot failed unexpectedly")

	assert.Equal(t, 2, deleted)
	assert.Equal(t, []time.Time{now.Add(time.Second), now.Add(time.Second * 2)}, storage.snapshots)
	assert.Equal(t, config.NamePrefix, storage.listPrefix)
	assert.Equal(t, config.NameSuffix, storage.listSuffix)
}

func TestDeleteObsoleteSnapshotsIgnoresFailures(t *testing.T) {
	config := storageConfigStub{
		storageConfig{
			Retain: 2,
		},
	}

	now := time.Now()
	storage := &storageStub{
		snapshots:      []time.Time{now.Add(time.Minute), now.Add(time.Second), now.Add(time.Hour), now.Add(time.Second * 2)},
		deleteFailures: []time.Time{now.Add(time.Hour)},
	}
	controller := &storageControllerImpl[time.Time]{
		config:  config,
		storage: storage,
	}

	deleted, err := controller.DeleteObsoleteSnapshots(context.Background(), StorageConfigDefaults{})
	assert.NoError(t, err, "UploadSnapshot failed unexpectedly")

	assert.Equal(t, 1, deleted)
	assert.Equal(t, []time.Time{now.Add(time.Second), now.Add(time.Second * 2), now.Add(time.Hour)}, storage.snapshots)
}

func TestDeleteObsoleteSnapshotsSkipsWhenNothingToRetain(t *testing.T) {
	config := storageConfigStub{
		storageConfig{
			Retain:     0,
			NamePrefix: "test",
		},
	}

	now := time.Now()
	storage := &storageStub{snapshots: []time.Time{now.Add(time.Minute)}}
	controller := &storageControllerImpl[time.Time]{
		config:  config,
		storage: storage,
	}

	deleted, err := controller.DeleteObsoleteSnapshots(context.Background(), StorageConfigDefaults{})
	assert.NoError(t, err, "UploadSnapshot failed unexpectedly")

	assert.Equal(t, 0, deleted)
	assert.Zero(t, storage.listPrefix)
}

func TestUploadSnapshotSkipsUploadBeforeScheduledTime(t *testing.T) {
	config := storageConfigStub{
		storageConfig{
			Frequency: time.Minute,
		},
	}

	storage := &storageStub{}
	controller := &storageControllerImpl[time.Time]{
		config:     config,
		lastUpload: time.Now(),
		storage:    storage,
	}

	uploaded, nextSnapshot, err := controller.UploadSnapshot(
		context.Background(),
		strings.NewReader("test"),
		controller.lastUpload.Add(time.Second),
		StorageConfigDefaults{},
	)
	assert.NoError(t, err, "UploadSnapshot failed unexpectedly")

	assert.False(t, uploaded)
	assert.Equal(t, controller.lastUpload.Add(config.Frequency), nextSnapshot)
	assert.Zero(t, storage.uploadContext)
}

type storageConfigStub struct {
	storageConfig
}

type storageStub struct {
	snapshots      []time.Time
	uploadContext  context.Context
	uploadName     string
	uploadData     string
	deleteFailures []time.Time
	listPrefix     string
	listSuffix     string
	deleted        bool
}

// nolint:unused
// implements interface storage
func (stub *storageStub) UploadSnapshot(ctx context.Context, name string, data io.Reader) error {
	stub.uploadContext = ctx
	stub.uploadName = name
	upload, err := io.ReadAll(data)
	if err != nil {
		return err
	}
	stub.uploadData = string(upload)
	return nil
}

// nolint:unused
// implements interface storage
func (stub *storageStub) DeleteSnapshot(_ context.Context, snapshot time.Time) error {
	stub.deleted = false
	for _, s := range stub.deleteFailures {
		if s == snapshot {
			return fmt.Errorf("could not delete snapshot %s", s)
		}
	}

	n := 0
	for _, s := range stub.snapshots {
		if s != snapshot {
			stub.snapshots[n] = s
			n++
		}
	}
	stub.snapshots = stub.snapshots[:n]
	return nil
}

// nolint:unused
// implements interface storage
func (stub *storageStub) ListSnapshots(_ context.Context, prefix string, suffix string) ([]time.Time, error) {
	stub.listPrefix = prefix
	stub.listSuffix = suffix

	return stub.snapshots, nil
}

// nolint:unused
// implements interface storage
func (stub *storageStub) GetLastModifiedTime(snapshot time.Time) time.Time {
	return snapshot
}

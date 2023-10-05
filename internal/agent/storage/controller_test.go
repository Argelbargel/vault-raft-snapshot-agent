package storage

import (
	"context"
	"errors"
	"fmt"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/test"
	"github.com/stretchr/testify/assert"
	"io"
	"strings"
	"testing"
	"time"
)

func TestScheduleSnapshotPrefersLastUploadTimeAndStorageConfig(t *testing.T) {
	lastUploadTime := time.Now()
	config := StorageControllerConfig{
		Frequency: time.Minute,
	}

	controller := &storageControllerImpl[time.Time]{
		config:     config,
		lastUpload: lastUploadTime,
	}

	nextSnapshot, err := controller.ScheduleSnapshot(context.Background(), time.Time{}, StorageConfigDefaults{})
	assert.NoError(t, err, "ScheduleSnapshot failed unexpectedly")
	assert.Equal(t, lastUploadTime.Add(config.Frequency), nextSnapshot)
}

func TestScheduleSnapshotFallsBackOnStorageConfigDefaults(t *testing.T) {
	lastUploadTime := time.Now()
	defaults := StorageConfigDefaults{
		Frequency: time.Minute,
	}

	controller := &storageControllerImpl[time.Time]{
		config:     StorageControllerConfig{},
		lastUpload: lastUploadTime,
	}

	nextSnapshot, err := controller.ScheduleSnapshot(context.Background(), time.Time{}, defaults)
	assert.NoError(t, err, "ScheduleSnapshot failed unexpectedly")
	assert.Equal(t, lastUploadTime.Add(defaults.Frequency), nextSnapshot)
}

func TestScheduleSnapshotFallsBackOnLastSnapshotTime(t *testing.T) {
	lastSnapshotTime := time.Now()
	config := StorageControllerConfig{
		Frequency: time.Minute,
	}

	controller := &storageControllerImpl[time.Time]{
		config: config,
	}

	nextSnapshot, err := controller.ScheduleSnapshot(context.Background(), lastSnapshotTime, StorageConfigDefaults{})
	assert.NoError(t, err, "ScheduleSnapshot failed unexpectedly")
	assert.Equal(t, lastSnapshotTime.Add(config.Frequency), nextSnapshot)
}

func TestScheduleSnapshotFallsBackOnStorageLastModifiedTime(t *testing.T) {
	storageLastModifiedTime := time.Now()
	config := StorageControllerConfig{
		Frequency: time.Minute,
	}

	storage := &storageStub{
		snapshots: []time.Time{storageLastModifiedTime},
	}

	controller := &storageControllerImpl[time.Time]{
		config:  config,
		storage: storage,
	}

	nextSnapshot, err := controller.ScheduleSnapshot(context.Background(), time.Time{}, StorageConfigDefaults{})
	assert.NoError(t, err, "ScheduleSnapshot failed unexpectedly")
	assert.Equal(t, storageLastModifiedTime.Add(config.Frequency), nextSnapshot)
}

func TestScheduleSnapshotReturnsZeroIfNoFallbackPossible(t *testing.T) {
	config := StorageControllerConfig{
		Frequency: time.Minute,
	}

	storage := &storageStub{}

	controller := &storageControllerImpl[time.Time]{
		config:  config,
		storage: storage,
	}

	nextSnapshot, err := controller.ScheduleSnapshot(context.Background(), time.Time{}, StorageConfigDefaults{})
	assert.Error(t, err)
	assert.Zero(t, nextSnapshot)
}

func TestUploadSnapshotUploadsToStorage(t *testing.T) {
	config := StorageControllerConfig{
		Frequency:       time.Minute,
		NamePrefix:      "test",
		NameSuffix:      ".test",
		TimestampFormat: "2006-01-02T15-04-05Z",
		Timeout:         time.Millisecond * 500,
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
	uploaded, nextSnapshot, err := controller.UploadSnapshot(ctx, strings.NewReader(data), 0, timestamp, StorageConfigDefaults{})
	assert.NoError(t, err, "uploadSnapshot failed unexpectedly")

	assert.True(t, uploaded)
	assert.Equal(t, timestamp.Add(config.Frequency), nextSnapshot)

	expectedDeadline, _ := storage.uploadContext.Deadline()
	expectedName := strings.Join([]string{config.NamePrefix, timestamp.Format(config.TimestampFormat), config.NameSuffix}, "")

	assert.GreaterOrEqual(t, expectedDeadline, start.Add(config.Timeout))
	assert.Equal(t, expectedName, storage.uploadName)
	assert.Equal(t, data, storage.uploadData)
}

func TestUploadSnapshotHandlesStorageFailure(t *testing.T) {
	config := StorageControllerConfig{
		Frequency: time.Minute,
	}

	storage := &storageStub{uploadFails: true}
	controller := &storageControllerImpl[time.Time]{
		config:  config,
		storage: storage,
	}

	ctx := context.Background()
	timestamp := time.Now()
	uploaded, nextSnapshot, err := controller.UploadSnapshot(ctx, strings.NewReader("test"), 0, timestamp, StorageConfigDefaults{})

	assert.False(t, uploaded)
	assert.Error(t, err, "uploadSnapshot should return error if storage fails")
	assert.Equal(t, timestamp.Add(config.Frequency), nextSnapshot)
}

func TestDeletesObsoleteSnapshots(t *testing.T) {
	config := StorageControllerConfig{
		Retain:     test.PtrTo(2),
		NamePrefix: "test",
		NameSuffix: ".text",
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
	assert.NoError(t, err, "DeleteObsoleteSnapshots failed unexpectedly")

	assert.Equal(t, 2, deleted)
	assert.Equal(t, []time.Time{now.Add(time.Hour), now.Add(time.Minute)}, storage.snapshots)
	assert.Equal(t, config.NamePrefix, storage.listPrefix)
	assert.Equal(t, config.NameSuffix, storage.listSuffix)
}

func TestDeleteObsoleteSnapshotsIgnoresFailures(t *testing.T) {
	config := StorageControllerConfig{
		Retain: test.PtrTo(2),
	}

	now := time.Now()
	storage := &storageStub{
		snapshots:      []time.Time{now.Add(time.Minute), now.Add(time.Second), now.Add(time.Hour), now.Add(time.Second * 2)},
		deleteFailures: []time.Time{now.Add(time.Second)},
	}
	controller := &storageControllerImpl[time.Time]{
		config:  config,
		storage: storage,
	}

	deleted, err := controller.DeleteObsoleteSnapshots(context.Background(), StorageConfigDefaults{})
	assert.NoError(t, err, "DeleteObsoleteSnapshots failed unexpectedly")

	assert.Equal(t, 1, deleted)
	assert.Equal(t, []time.Time{now.Add(time.Hour), now.Add(time.Minute), now.Add(time.Second)}, storage.snapshots)
}

func TestDeleteObsoleteSnapshotsSkipsWhenNothingToRetain(t *testing.T) {
	config := StorageControllerConfig{
		Retain: test.PtrTo(0),
	}

	now := time.Now()
	storage := &storageStub{snapshots: []time.Time{now.Add(time.Minute)}}
	controller := &storageControllerImpl[time.Time]{
		config:  config,
		storage: storage,
	}

	deleted, err := controller.DeleteObsoleteSnapshots(context.Background(), StorageConfigDefaults{})
	assert.NoError(t, err, "DeleteObsoleteSnapshots failed unexpectedly")

	assert.Equal(t, 0, deleted)
	assert.Zero(t, storage.listPrefix)
}

func TestDeleteObsoleteSnapshotsHandlesStorageFailure(t *testing.T) {
	config := StorageControllerConfig{
		Retain: test.PtrTo(1),
	}

	storage := &storageStub{listFails: true}
	controller := &storageControllerImpl[time.Time]{
		config:  config,
		storage: storage,
	}

	deleted, err := controller.DeleteObsoleteSnapshots(context.Background(), StorageConfigDefaults{})
	assert.Equal(t, 0, deleted)
	assert.Error(t, err, "DeleteObsoleteSnapshots should fail if storage fails")
}

func TestDeleteObsoleteSnapshotsSkipsWhenNothingToDelete(t *testing.T) {
	config := StorageControllerConfig{
		Retain:     test.PtrTo(1),
		NamePrefix: "test",
	}

	storage := &storageStub{snapshots: []time.Time{time.Now().Add(time.Minute)}}
	controller := &storageControllerImpl[time.Time]{
		config:  config,
		storage: storage,
	}

	deleted, err := controller.DeleteObsoleteSnapshots(context.Background(), StorageConfigDefaults{})
	assert.NoError(t, err, "DeleteObsoleteSnapshots failed unexpectedly")

	assert.Equal(t, 0, deleted)
	assert.Equal(t, config.NamePrefix, storage.listPrefix)
	assert.False(t, storage.deleted)
}

func TestUploadSnapshotSkipsUploadBeforeScheduledTime(t *testing.T) {
	config := StorageControllerConfig{
		Frequency: time.Minute,
	}

	storage := &storageStub{}
	controller := &storageControllerImpl[time.Time]{
		config:     config,
		lastUpload: time.Now(),
		storage:    storage,
	}

	uploaded, nextSnapshot, err := controller.UploadSnapshot(context.Background(), strings.NewReader("test"), 0, controller.lastUpload.Add(time.Second), StorageConfigDefaults{})
	assert.NoError(t, err, "uploadSnapshot failed unexpectedly")

	assert.False(t, uploaded)
	assert.Equal(t, controller.lastUpload.Add(config.Frequency), nextSnapshot)
	assert.Zero(t, storage.uploadContext)
}

type storageStub struct {
	snapshots      []time.Time
	uploadContext  context.Context
	uploadFails    bool
	uploadName     string
	uploadData     string
	deleteFailures []time.Time
	listFails      bool
	listPrefix     string
	listSuffix     string
	deleted        bool
}

// nolint:unused
// implements interface storage
func (stub *storageStub) uploadSnapshot(ctx context.Context, name string, data io.Reader, _ int64) error {
	stub.uploadContext = ctx
	stub.uploadName = name
	upload, err := io.ReadAll(data)
	if err != nil {
		return err
	}
	stub.uploadData = string(upload)
	if stub.uploadFails {
		return errors.New("upload failed")
	}
	return nil
}

// nolint:unused
// implements interface storage
func (stub *storageStub) deleteSnapshot(_ context.Context, snapshot time.Time) error {
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
func (stub *storageStub) listSnapshots(_ context.Context, prefix string, suffix string) ([]time.Time, error) {
	stub.listPrefix = prefix
	stub.listSuffix = suffix

	if stub.listFails {
		return nil, errors.New("listing failed")
	}

	return stub.snapshots, nil
}

// nolint:unused
// implements interface storage
func (stub *storageStub) getLastModifiedTime(snapshot time.Time) time.Time {
	return snapshot
}

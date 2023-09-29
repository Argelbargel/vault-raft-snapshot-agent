package storage

import (
	"context"
	"errors"
	"github.com/stretchr/testify/assert"
	"io"
	"strings"
	"testing"
	"time"
)

func TestManagerSchedulesEarliestNextSnapshot(t *testing.T) {
	controller1 := &storageControllerStub{nextSnapshot: time.Now().Add(time.Millisecond * 2)}
	controller2 := &storageControllerStub{nextSnapshot: time.Now().Add(time.Millisecond)}
	manager := Manager{
		[]StorageControllerFactory{
			storageControllerFactoryStub{controller: controller1},
			storageControllerFactoryStub{controller: controller2},
		},
	}

	assert.Equal(t, controller2.nextSnapshot, manager.ScheduleSnapshot(context.Background(), controller1.nextSnapshot, StorageConfigDefaults{}))
}

func TestScheduleSnapshotIgnoresFactoryAndControllerFailure(t *testing.T) {
	controller1 := &storageControllerStub{scheduleFails: true}
	controller2 := &storageControllerStub{nextSnapshot: time.Now().Add(time.Millisecond)}
	manager := Manager{
		[]StorageControllerFactory{
			storageControllerFactoryStub{createFails: true},
			storageControllerFactoryStub{controller: controller1},
			storageControllerFactoryStub{controller: controller2},
		},
	}

	defaults := StorageConfigDefaults{}
	nextSnapshot := manager.ScheduleSnapshot(context.Background(), time.Time{}, defaults)

	assert.Equal(t, controller2.nextSnapshot, nextSnapshot)
}

func TestManagerUploadsToAllControllers(t *testing.T) {
	controller1 := &storageControllerStub{nextSnapshot: time.Now().Add(time.Millisecond * 2)}
	controller2 := &storageControllerStub{nextSnapshot: time.Now().Add(time.Millisecond)}
	manager := Manager{
		[]StorageControllerFactory{
			storageControllerFactoryStub{controller: controller1},
			storageControllerFactoryStub{controller: controller2},
		},
	}

	data := "test"
	nextSnapshot := manager.UploadSnapshot(context.Background(), strings.NewReader(data), 0, controller1.nextSnapshot, StorageConfigDefaults{})

	assert.Equal(t, data, controller1.uploadData)
	assert.Equal(t, controller1.nextSnapshot, controller1.snapshotTimestamp)
	assert.Equal(t, data, controller2.uploadData)
	assert.Equal(t, controller1.nextSnapshot, controller2.snapshotTimestamp)
	assert.Equal(t, controller2.nextSnapshot, nextSnapshot)
}

func TestManagerDeletesObsoleteSnapshotsWithAllControllers(t *testing.T) {
	controller1 := &storageControllerStub{}
	controller2 := &storageControllerStub{}
	manager := Manager{
		[]StorageControllerFactory{
			storageControllerFactoryStub{controller: controller1},
			storageControllerFactoryStub{controller: controller2},
		},
	}

	defaults := StorageConfigDefaults{Retain: 2}
	_ = manager.UploadSnapshot(context.Background(), strings.NewReader("test"), 0, controller1.nextSnapshot, defaults)

	assert.Equal(t, defaults, controller1.deleteDefaults)
	assert.Equal(t, defaults, controller2.deleteDefaults)
}

func TestManagerIgnoresFactoryAndControllerFailure(t *testing.T) {
	controller1 := &storageControllerStub{uploadFails: true, nextSnapshot: time.Now().Add(time.Millisecond)}
	controller2 := &storageControllerStub{deleteFails: true, nextSnapshot: time.Now().Add(time.Millisecond * 2)}
	controller3 := &storageControllerStub{nextSnapshot: time.Now().Add(time.Millisecond * 3)}
	manager := Manager{
		[]StorageControllerFactory{
			storageControllerFactoryStub{createFails: true},
			storageControllerFactoryStub{controller: controller1},
			storageControllerFactoryStub{controller: controller2},
			storageControllerFactoryStub{controller: controller3},
		},
	}

	data := "test"
	defaults := StorageConfigDefaults{}
	nextSnapshot := manager.UploadSnapshot(context.Background(), strings.NewReader(data), 0, controller3.nextSnapshot, defaults)

	assert.Equal(t, data, controller3.uploadData)
	assert.Equal(t, controller3.nextSnapshot, controller3.snapshotTimestamp)
	assert.Equal(t, data, controller2.uploadData)
	assert.Equal(t, controller3.nextSnapshot, controller2.snapshotTimestamp)
	assert.Equal(t, controller1.nextSnapshot, nextSnapshot)
	assert.Equal(t, defaults, controller1.deleteDefaults)
	assert.Equal(t, defaults, controller2.deleteDefaults)
	assert.Equal(t, defaults, controller3.deleteDefaults)
}

func TestManagerIgnoresSkippedControllers(t *testing.T) {
	controller1 := &storageControllerStub{nextSnapshot: time.Now().Add(time.Millisecond * 2)}
	controller2 := &storageControllerStub{nextSnapshot: time.Now().Add(time.Millisecond)}
	manager := Manager{
		[]StorageControllerFactory{
			storageControllerFactoryStub{controller: controller1},
			storageControllerFactoryStub{controller: controller2},
		},
	}

	data := "test"
	nextSnapshot := manager.UploadSnapshot(context.Background(), strings.NewReader(data), 0, controller2.nextSnapshot, StorageConfigDefaults{})

	assert.Equal(t, data, controller2.uploadData)
	assert.Equal(t, controller2.nextSnapshot, controller2.snapshotTimestamp)
	assert.Equal(t, controller2.nextSnapshot, nextSnapshot)
}

func TestManagerFailsIfSnapshotCannotBeReset(t *testing.T) {
	controller := &storageControllerStub{}
	manager := Manager{
		[]StorageControllerFactory{
			storageControllerFactoryStub{controller: controller},
		},
	}

	defaults := StorageConfigDefaults{Frequency: time.Second}
	timestamp := time.Now()
	nextSnapshot := manager.UploadSnapshot(context.Background(), ReadSeekerStub{}, 0, timestamp, defaults)

	assert.Equal(t, timestamp.Add(defaults.Frequency), nextSnapshot)
	assert.Zero(t, controller.uploadData)
}

type storageControllerFactoryStub struct {
	createFails bool
	controller  *storageControllerStub
}

func (stub storageControllerFactoryStub) Destination() string {
	return ""
}

func (stub storageControllerFactoryStub) CreateController(context.Context) (StorageController, error) {
	if stub.createFails {
		return nil, errors.New("create failed")
	}
	return stub.controller, nil
}

type storageControllerStub struct {
	uploadDefaults    StorageConfigDefaults
	scheduleFails     bool
	uploadData        string
	uploadFails       bool
	deleteFails       bool
	deleteDefaults    StorageConfigDefaults
	snapshotTimestamp time.Time
	nextSnapshot      time.Time
}

func (stub *storageControllerStub) ScheduleSnapshot(context.Context, time.Time, StorageConfigDefaults) (time.Time, error) {
	if stub.scheduleFails {
		return time.Time{}, errors.New("scheduling failed")
	}
	return stub.nextSnapshot, nil
}

func (stub *storageControllerStub) UploadSnapshot(_ context.Context, snapshot io.Reader, _ int64, timestamp time.Time, defaults StorageConfigDefaults) (bool, time.Time, error) {
	stub.snapshotTimestamp = timestamp
	stub.uploadDefaults = defaults
	if stub.uploadFails {
		return false, stub.nextSnapshot, errors.New("upload failed")
	}

	if timestamp.Before(stub.nextSnapshot) {
		return false, stub.nextSnapshot, nil
	}

	data, err := io.ReadAll(snapshot)
	if err != nil {
		return false, stub.nextSnapshot, err
	}

	stub.uploadData = string(data)
	return true, stub.nextSnapshot, nil
}

func (stub *storageControllerStub) DeleteObsoleteSnapshots(_ context.Context, defaults StorageConfigDefaults) (int, error) {
	stub.deleteDefaults = defaults
	if stub.deleteFails {
		return 0, errors.New("delete failed")
	}
	return 1, nil
}

type ReadSeekerStub struct{}

func (stub ReadSeekerStub) Seek(int64, int) (int64, error) {
	return 0, errors.New("seek failed")
}

func (stub ReadSeekerStub) Read([]byte) (n int, err error) {
	return 0, nil
}

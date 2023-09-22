package agent

import (
	"context"
	"errors"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/storage"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/vault"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTakeSnapshotUploadsSnapshot(t *testing.T) {
	clientVaultAPI := &clientVaultAPIStub{
		leader:       true,
		snapshotData: "test",
	}

	defaults := storage.StorageConfigDefaults{
		Frequency: time.Millisecond,
	}

	controller := &storageControllerStub{
		nextSnapshot: time.Now().Add(time.Millisecond * 250),
	}

	manager := &storage.Manager{}
	manager.AddStorage(controller)

	ctx := context.Background()

	agent := SnapshotAgent{}
	agent.update(ctx, newClient(clientVaultAPI), manager, defaults)

	start := time.Now()
	timer := agent.TakeSnapshot(ctx)
	<-timer.C

	assert.True(t, clientVaultAPI.tookSnapshot)
	assert.Equal(t, clientVaultAPI.snapshotData, controller.uploadData)
	assert.Equal(t, defaults, controller.defaults)
	assert.WithinRange(t, controller.snapshotTimestamp, start, start.Add(50*time.Millisecond))
	assert.GreaterOrEqual(t, time.Now(), controller.nextSnapshot)
}

func TestTakeSnapshotLocksTakeSnapshot(t *testing.T) {
	clientVaultAPI := &clientVaultAPIStub{
		leader:          true,
		snapshotRuntime: time.Millisecond * 500,
	}

	ctx := context.Background()

	agent := SnapshotAgent{}
	agent.update(ctx, newClient(clientVaultAPI), &storage.Manager{}, storage.StorageConfigDefaults{})

	start := time.Now()

	done := make(chan bool, 1)
	go func() {
		_ = agent.TakeSnapshot(ctx)
		done <- true
	}()

	go func() {
		_ = agent.TakeSnapshot(ctx)
		done <- true
	}()

	for i := 0; i < 2; i++ {
		<-done
	}

	assert.GreaterOrEqual(t, time.Since(start), clientVaultAPI.snapshotRuntime*2, "TakeSnapshot did not prevent synchronous snapshots")
}

func TestTakeSnapshotLocksUpdate(t *testing.T) {
	clientVaultAPI := &clientVaultAPIStub{
		leader:          true,
		snapshotRuntime: time.Millisecond * 500,
	}

	ctx := context.Background()

	agent := SnapshotAgent{}
	agent.update(ctx, newClient(clientVaultAPI), &storage.Manager{}, storage.StorageConfigDefaults{})

	start := time.Now()

	running := make(chan bool, 1)
	done := make(chan bool, 1)
	go func() {
		running <- true
		_ = agent.TakeSnapshot(ctx)
		done <- true
	}()

	go func() {
		<-running
		agent.update(ctx, newClient(clientVaultAPI), &storage.Manager{}, storage.StorageConfigDefaults{})
		done <- true
	}()

	for i := 0; i < 2; i++ {
		<-done
	}

	assert.GreaterOrEqual(t, time.Since(start), clientVaultAPI.snapshotRuntime+250, "TakeSnapshot did not prevent re-configuration during snapshots")
}

func TestTakeSnapshotFailsWhenTempFileCannotBeCreated(t *testing.T) {
	clientVaultAPI := &clientVaultAPIStub{
		leader: true,
	}

	defaults := storage.StorageConfigDefaults{
		Frequency: time.Millisecond * 150,
	}

	controller := &storageControllerStub{
		nextSnapshot: time.Now().Add(defaults.Frequency * 4),
	}

	manager := &storage.Manager{}
	manager.AddStorage(controller)

	ctx := context.Background()

	agent := SnapshotAgent{tempDir: "./missing"}
	agent.update(ctx, newClient(clientVaultAPI), manager, defaults)

	start := time.Now()
	timer := agent.TakeSnapshot(ctx)
	<-timer.C

	assert.False(t, clientVaultAPI.tookSnapshot)
	assert.WithinRange(t, time.Now(), start.Add(defaults.Frequency), start.Add(defaults.Frequency*2))
}

func TestTakeSnapshotFailsWhenSnapshottingFails(t *testing.T) {
	clientVaultAPI := &clientVaultAPIStub{
		leader:        true,
		snapshotFails: true,
	}

	defaults := storage.StorageConfigDefaults{
		Frequency: time.Millisecond * 150,
	}

	controller := &storageControllerStub{
		nextSnapshot: time.Now().Add(defaults.Frequency * 4),
	}

	manager := &storage.Manager{}
	manager.AddStorage(controller)

	ctx := context.Background()

	agent := SnapshotAgent{}
	agent.update(ctx, newClient(clientVaultAPI), manager, defaults)

	start := time.Now()
	timer := agent.TakeSnapshot(ctx)
	<-timer.C

	assert.True(t, clientVaultAPI.tookSnapshot)
	assert.WithinRange(t, time.Now(), start.Add(defaults.Frequency), start.Add(defaults.Frequency*2))
}

func TestTakeSnapshotIgnoresEmptySnapshot(t *testing.T) {
	clientVaultAPI := &clientVaultAPIStub{
		leader: true,
	}

	defaults := storage.StorageConfigDefaults{
		Frequency: time.Millisecond * 150,
	}

	controller := &storageControllerStub{
		nextSnapshot: time.Now().Add(defaults.Frequency * 4),
	}

	manager := &storage.Manager{}
	manager.AddStorage(controller)

	ctx := context.Background()

	agent := SnapshotAgent{}
	agent.update(ctx, newClient(clientVaultAPI), manager, defaults)

	start := time.Now()
	timer := agent.TakeSnapshot(ctx)
	<-timer.C

	assert.True(t, clientVaultAPI.tookSnapshot)
	assert.WithinRange(t, time.Now(), start.Add(defaults.Frequency), start.Add(defaults.Frequency*2))
}

func TestIgnoresZeroTimeForScheduling(t *testing.T) {
	clientVaultAPI := &clientVaultAPIStub{
		leader:       true,
		snapshotData: "test",
	}

	defaults := storage.StorageConfigDefaults{
		Frequency: time.Millisecond * 150,
	}

	controller := &storageControllerStub{
		nextSnapshot: time.Time{},
	}

	manager := &storage.Manager{}
	manager.AddStorage(controller)

	ctx := context.Background()

	agent := SnapshotAgent{}
	agent.update(ctx, newClient(clientVaultAPI), manager, defaults)

	start := time.Now()
	timer := agent.TakeSnapshot(ctx)
	<-timer.C

	assert.True(t, clientVaultAPI.tookSnapshot)
	assert.Equal(t, clientVaultAPI.snapshotData, controller.uploadData)
	assert.WithinRange(t, time.Now(), start.Add(defaults.Frequency), start.Add(defaults.Frequency*2))
}

func newClient(api *clientVaultAPIStub) *vault.Client[any, clientVaultAPIAuthStub] {
	return vault.NewClient[any, clientVaultAPIAuthStub](api, clientVaultAPIAuthStub{}, time.Time{})
}

type clientVaultAPIStub struct {
	snapshotFails   bool
	tookSnapshot    bool
	leader          bool
	snapshotRuntime time.Duration
	snapshotData    string
}

func (stub *clientVaultAPIStub) Address() string {
	return "test"
}

func (stub *clientVaultAPIStub) TakeSnapshot(ctx context.Context, writer io.Writer) error {
	stub.tookSnapshot = true
	if stub.snapshotFails {
		return errors.New("TakeSnapshot failed")
	}

	if stub.snapshotData != "" {
		if _, err := writer.Write([]byte(stub.snapshotData)); err != nil {
			return err
		}
	}

	select {
	case <-ctx.Done():
	case <-time.After(stub.snapshotRuntime):
	}

	return nil
}

func (stub *clientVaultAPIStub) IsLeader() (bool, error) {
	return stub.leader, nil
}

func (stub *clientVaultAPIStub) RefreshAuth(ctx context.Context, auth clientVaultAPIAuthStub) (time.Duration, error) {
	return auth.Login(ctx, nil)
}

type clientVaultAPIAuthStub struct{}

func (stub clientVaultAPIAuthStub) Login(_ context.Context, _ any) (time.Duration, error) {
	return 0, nil
}

type storageControllerStub struct {
	defaults          storage.StorageConfigDefaults
	uploadData        string
	uploadFails       bool
	snapshotTimestamp time.Time
	nextSnapshot      time.Time
}

func (stub *storageControllerStub) Destination() string {
	return ""
}

func (stub *storageControllerStub) ScheduleSnapshot(_ context.Context, _ time.Time, _ storage.StorageConfigDefaults) time.Time {
	return stub.nextSnapshot
}

func (stub *storageControllerStub) DeleteObsoleteSnapshots(_ context.Context, _ storage.StorageConfigDefaults) (int, error) {
	return 0, nil
}

func (stub *storageControllerStub) UploadSnapshot(_ context.Context, snapshot io.Reader, timestamp time.Time, defaults storage.StorageConfigDefaults) (bool, time.Time, error) {
	stub.snapshotTimestamp = timestamp
	stub.defaults = defaults
	if stub.uploadFails {
		return false, stub.nextSnapshot, errors.New("upload failed")
	}
	data, err := io.ReadAll(snapshot)
	if err != nil {
		return false, time.Now(), err
	}
	stub.uploadData = string(data)
	return true, stub.nextSnapshot, nil
}

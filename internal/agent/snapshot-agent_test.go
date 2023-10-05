package agent

import (
	"context"
	"errors"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/storage"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/vault"
	"github.com/hashicorp/vault/api"
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

	factory := &storageControllerFactoryStub{nextSnapshot: time.Now().Add(time.Millisecond * 250)}

	manager := &storage.Manager{}
	manager.AddStorageFactory(factory)

	ctx := context.Background()

	agent := newSnapshotAgent(t.TempDir())
	agent.update(ctx, newClient(clientVaultAPI), manager, defaults)

	start := time.Now()
	ticker := agent.TakeSnapshot(ctx)
	<-ticker.C

	assert.True(t, clientVaultAPI.tookSnapshot)
	assert.Equal(t, clientVaultAPI.snapshotData, factory.uploadData)
	assert.Equal(t, defaults, factory.defaults)
	assert.WithinRange(t, factory.snapshotTimestamp, start, start.Add(50*time.Millisecond))
	assert.GreaterOrEqual(t, time.Now(), factory.nextSnapshot)
}

func TestTakeSnapshotLocksTakeSnapshot(t *testing.T) {
	clientVaultAPI := &clientVaultAPIStub{
		leader:          true,
		snapshotRuntime: time.Millisecond * 500,
	}

	ctx := context.Background()

	agent := newSnapshotAgent(t.TempDir())
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

	agent := newSnapshotAgent(t.TempDir())
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

	factory := &storageControllerFactoryStub{
		nextSnapshot: time.Now().Add(defaults.Frequency * 4),
	}

	manager := &storage.Manager{}
	manager.AddStorageFactory(factory)

	ctx := context.Background()

	agent := newSnapshotAgent("./missing")
	agent.update(ctx, newClient(clientVaultAPI), manager, defaults)

	ticker := agent.TakeSnapshot(ctx)
	<-ticker.C

	assert.False(t, clientVaultAPI.tookSnapshot)
	assert.Less(t, time.Now(), factory.nextSnapshot.Add(-defaults.Frequency))
}

func TestTakeSnapshotFailsWhenSnapshottingFails(t *testing.T) {
	clientVaultAPI := &clientVaultAPIStub{
		leader:        true,
		snapshotFails: true,
	}

	defaults := storage.StorageConfigDefaults{
		Frequency: time.Millisecond * 150,
	}

	factory := &storageControllerFactoryStub{
		nextSnapshot: time.Now().Add(defaults.Frequency * 4),
	}

	manager := &storage.Manager{}
	manager.AddStorageFactory(factory)

	ctx := context.Background()

	agent := newSnapshotAgent(t.TempDir())
	agent.update(ctx, newClient(clientVaultAPI), manager, defaults)

	ticker := agent.TakeSnapshot(ctx)
	<-ticker.C

	assert.True(t, clientVaultAPI.tookSnapshot)
	assert.Less(t, time.Now(), factory.nextSnapshot.Add(-defaults.Frequency))
}

func TestTakeSnapshotIgnoresEmptySnapshot(t *testing.T) {
	clientVaultAPI := &clientVaultAPIStub{
		leader: true,
	}

	defaults := storage.StorageConfigDefaults{
		Frequency: time.Millisecond * 150,
	}

	factory := &storageControllerFactoryStub{
		nextSnapshot: time.Now().Add(defaults.Frequency * 4),
	}

	manager := &storage.Manager{}
	manager.AddStorageFactory(factory)

	ctx := context.Background()

	agent := newSnapshotAgent(t.TempDir())
	agent.update(ctx, newClient(clientVaultAPI), manager, defaults)

	ticker := agent.TakeSnapshot(ctx)
	<-ticker.C

	assert.True(t, clientVaultAPI.tookSnapshot)
	assert.Less(t, time.Now(), factory.nextSnapshot.Add(-defaults.Frequency))
}

func TestIgnoresZeroTimeForScheduling(t *testing.T) {
	clientVaultAPI := &clientVaultAPIStub{
		leader:       true,
		snapshotData: "test",
	}

	defaults := storage.StorageConfigDefaults{
		Frequency: time.Millisecond * 150,
	}

	factory := &storageControllerFactoryStub{
		nextSnapshot: time.Time{},
	}

	manager := &storage.Manager{}
	manager.AddStorageFactory(factory)

	ctx := context.Background()

	agent := newSnapshotAgent(t.TempDir())
	agent.update(ctx, newClient(clientVaultAPI), manager, defaults)

	start := time.Now()
	ticker := agent.TakeSnapshot(ctx)
	<-ticker.C

	assert.True(t, clientVaultAPI.tookSnapshot)
	assert.Equal(t, clientVaultAPI.snapshotData, factory.uploadData)
	assert.GreaterOrEqual(t, time.Now(), start.Add(defaults.Frequency))
}

func TestUpdateReschedulesSnapshots(t *testing.T) {
	clientVaultAPI := &clientVaultAPIStub{
		leader:       true,
		snapshotData: "test",
	}

	manager := &storage.Manager{}
	factory := &storageControllerFactoryStub{nextSnapshot: time.Now().Add(time.Millisecond * 250)}
	manager.AddStorageFactory(factory)

	newFactory := &storageControllerFactoryStub{nextSnapshot: time.Now().Add(time.Millisecond * 500)}
	newManager := &storage.Manager{}
	newManager.AddStorageFactory(newFactory)

	ctx := context.Background()
	agent := newSnapshotAgent(t.TempDir())
	client := newClient(clientVaultAPI)
	agent.update(ctx, client, manager, storage.StorageConfigDefaults{})
	ticker := agent.TakeSnapshot(ctx)

	updated := make(chan bool, 1)
	go func() {
		agent.update(ctx, client, newManager, storage.StorageConfigDefaults{})
		updated <- true
	}()

	<-updated
	<-ticker.C

	assert.GreaterOrEqual(t, time.Now(), newFactory.nextSnapshot)
	assert.Equal(t, newManager, agent.manager)
}

func newClient(api *clientVaultAPIStub) *vault.VaultClient {
	return vault.NewClient(api, clientVaultAPIAuthStub{}, time.Time{})
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

func (stub *clientVaultAPIStub) RefreshAuth(ctx context.Context, auth api.AuthMethod) (time.Duration, error) {
	_, err := auth.Login(ctx, nil)
	return 0, err
}

type clientVaultAPIAuthStub struct{}

func (stub clientVaultAPIAuthStub) Login(_ context.Context, _ *api.Client) (*api.Secret, error) {
	return nil, nil
}

type storageControllerFactoryStub struct {
	defaults          storage.StorageConfigDefaults
	uploadData        string
	uploadFails       bool
	snapshotTimestamp time.Time
	nextSnapshot      time.Time
}

func (stub *storageControllerFactoryStub) Destination() string {
	return ""
}

func (stub *storageControllerFactoryStub) CreateController(context.Context) (storage.StorageController, error) {
	return storageControllerStub{stub}, nil
}

type storageControllerStub struct {
	factory *storageControllerFactoryStub
}

func (stub storageControllerStub) ScheduleSnapshot(_ context.Context, _ time.Time, _ storage.StorageConfigDefaults) (time.Time, error) {
	return stub.factory.nextSnapshot, nil
}

func (stub storageControllerStub) DeleteObsoleteSnapshots(_ context.Context, _ storage.StorageConfigDefaults) (int, error) {
	return 0, nil
}

func (stub storageControllerStub) UploadSnapshot(_ context.Context, snapshot io.Reader, _ int64, timestamp time.Time, defaults storage.StorageConfigDefaults) (bool, time.Time, error) {
	stub.factory.snapshotTimestamp = timestamp
	stub.factory.defaults = defaults
	if stub.factory.uploadFails {
		return false, stub.factory.nextSnapshot, errors.New("upload failed")
	}
	data, err := io.ReadAll(snapshot)
	if err != nil {
		return false, time.Now(), err
	}
	stub.factory.uploadData = string(data)
	return true, stub.factory.nextSnapshot, nil
}

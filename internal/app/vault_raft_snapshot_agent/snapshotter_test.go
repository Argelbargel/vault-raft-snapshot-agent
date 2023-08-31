package vault_raft_snapshot_agent

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/app/vault_raft_snapshot_agent/upload"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/app/vault_raft_snapshot_agent/vault"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/app/vault_raft_snapshot_agent/vault/auth"
)

func TestSnapshotterLocksTakeSnapshot(t *testing.T) {
	clientAPIStub := snapshotterVaultClientAPIStub{
		leader:          true,
		snapshotRuntime: time.Millisecond * 500,
	}
	uploaderStub := uploaderStub{}
	config := SnapshotConfig{
		Timeout: clientAPIStub.snapshotRuntime * 3,
	}

	snapshotter := Snapshotter{}
	snapshotter.Configure(config, vault.NewClient("http://127.0.0.1:8200", &clientAPIStub, nil), []upload.Uploader{&uploaderStub})

	start := time.Now()

	errs := make(chan error, 1)
	go func() {
		_, err := snapshotter.TakeSnapshot(context.Background())
		errs <- err
	}()

	go func() {
		_, err := snapshotter.TakeSnapshot(context.Background())
		errs <- err
	}()

	for i := 0; i < 2; i++ {
		err := <-errs
		if err != nil {
			t.Fatalf("TakeSnapshot failed unexpectedly: %s", err)
		}
	}

	runtime := time.Now().Sub(start)
	expectedRuntime := clientAPIStub.snapshotRuntime * 2
	if runtime < expectedRuntime {
		t.Fatalf("TakeSnapshot did not prevent synchronous snapshots - expected runtime: %d, was: %d", expectedRuntime, runtime)
	}
}

func TestSnapshotterLocksConfigure(t *testing.T) {
	clientAPIStub := snapshotterVaultClientAPIStub{
		leader:          true,
		snapshotRuntime: time.Millisecond * 500,
	}
	uploaderStub := uploaderStub{}
	config := SnapshotConfig{
		Timeout: clientAPIStub.snapshotRuntime * 3,
	}

	newConfig := SnapshotConfig{
		Frequency: time.Minute,
		Timeout:   time.Millisecond,
	}

	snapshotter := Snapshotter{}
	snapshotter.Configure(config, vault.NewClient("http://127.0.0.1:8200", &clientAPIStub, nil), []upload.Uploader{&uploaderStub})

	start := time.Now()

	errs := make(chan error, 1)
	go func() {
		_, err := snapshotter.TakeSnapshot(context.Background())
		errs <- err
	}()
	go func() {
		time.Sleep(50) // wait for TakeSnapshot to start
		snapshotter.Configure(newConfig, vault.NewClient("http://127.0.0.1:8200", &clientAPIStub, nil), []upload.Uploader{&uploaderStub})
		errs <- nil
	}()

	for i := 0; i < 2; i++ {
		err := <-errs
		if err != nil {
			t.Fatalf("TakeSnapshot failed unexpectedly: %s", err)
		}
	}

	runtime := time.Now().Sub(start)
	expectedRuntime := clientAPIStub.snapshotRuntime + 250
	if runtime < expectedRuntime {
		t.Fatalf("TakeSnapshot did not prevent re-configuration during snapshots - expected runtime: %d, was: %d", expectedRuntime, runtime)
	}

	frequency, err := snapshotter.TakeSnapshot(context.Background())
	if err != nil {
		t.Fatalf("TakeSnapshot failed unexpectedly: %s", err)
	}

	if frequency != newConfig.Frequency {
		t.Fatalf("Snaphotter did not re-configure propertly - expected frequency: %v, got: %v", newConfig.Frequency, frequency)
	}
}

func TestSnapshotterAbortsAfterTimeout(t *testing.T) {
	clientAPIStub := snapshotterVaultClientAPIStub{
		leader:          true,
		snapshotRuntime: time.Second * 5,
	}
	uploaderStub := uploaderStub{}
	config := SnapshotConfig{
		Timeout: time.Second,
	}

	snapshotter := Snapshotter{}
	snapshotter.Configure(config, vault.NewClient("http://127.0.0.1:8200", &clientAPIStub, nil), []upload.Uploader{&uploaderStub})

	start := time.Now()

	errs := make(chan error, 1)
	go func() {
		_, err := snapshotter.TakeSnapshot(context.Background())
		errs <- err
	}()

	err := <-errs
	if err != nil {
		t.Fatalf("TakeSnapshot failed unexpectedly: %s", err)
	}

	runtime := time.Now().Sub(start)
	expectedRuntime := config.Timeout * 2 // quite less than runtime more enough to not flicker
	if runtime > expectedRuntime {
		t.Fatalf("TakeSnapshot did not abort at timeout - expected runtime: %d, was: %d", expectedRuntime, runtime)
	}
}

func TestSnapshotterFailsIfSnapshottingFails(t *testing.T) {
	clientAPIStub := snapshotterVaultClientAPIStub{
		leader: false,
	}
	uploaderStub := uploaderStub{}
	config := SnapshotConfig{
		Timeout: time.Second,
	}

	snapshotter := Snapshotter{}
	snapshotter.Configure(config, vault.NewClient("http://127.0.0.1:8200", &clientAPIStub, nil), []upload.Uploader{&uploaderStub})

	_, err := snapshotter.TakeSnapshot(context.Background())
	if err == nil {
		t.Fatalf("TakeSnaphot did not fail although snapshotting failed")
	}

	if uploaderStub.uploaded {
		t.Fatalf("TakeSnapshot uploaded although snapshotting failed")
	}
}

func TestSnapshotterUploadsDataFromSnapshot(t *testing.T) {
	clientAPIStub := snapshotterVaultClientAPIStub{
		leader:       true,
		snapshotData: "test-snapshot",
	}
	uploaderStub := uploaderStub{}
	config := SnapshotConfig{
		Timeout: time.Second,
	}

	snapshotter := Snapshotter{}
	snapshotter.Configure(config, vault.NewClient("http://127.0.0.1:8200", &clientAPIStub, nil), []upload.Uploader{&uploaderStub})

	_, err := snapshotter.TakeSnapshot(context.Background())
	if err != nil {
		t.Fatalf("TakeSnaphot failed unexpectedly: %v", err)
	}

	if !uploaderStub.uploaded || uploaderStub.uploadData != clientAPIStub.snapshotData {
		t.Fatalf("TakeSnapshot did not upload or uploaded false data - uploaded %v, expected data: %s, got data: %s", uploaderStub.uploaded, uploaderStub.uploadData, clientAPIStub.snapshotData)
	}
}

func TestSnapshotterContinuesUploadingIfUploadFails(t *testing.T) {
	clientAPIStub := snapshotterVaultClientAPIStub{
		leader:       true,
		snapshotData: "test-snapshot",
	}
	uploaderStub1 := uploaderStub{
		uploadFails: true,
	}
	uploaderStub2 := uploaderStub{
		uploadFails: false,
	}

	config := SnapshotConfig{
		Timeout: time.Second,
	}

	snapshotter := Snapshotter{}
	snapshotter.Configure(config, vault.NewClient("http://127.0.0.1:8200", &clientAPIStub, nil), []upload.Uploader{&uploaderStub1, &uploaderStub2})

	_, err := snapshotter.TakeSnapshot(context.Background())
	if err == nil {
		t.Fatalf("TakeSnaphot did not fail although one of the uploaders failed")
	}

	if !uploaderStub1.uploaded || !uploaderStub2.uploaded {
		t.Fatalf("TakeSnapshot did not upload upload to all uploaders: uploader1 %v, uploader2: %v", uploaderStub1.uploaded, uploaderStub2.uploaded)
	}
}

func TestSnapshotterReturnsFrequency(t *testing.T) {
	clientAPIStub := snapshotterVaultClientAPIStub{}
	uploaderStub := uploaderStub{}

	config := SnapshotConfig{
		Frequency: time.Minute,
	}

	snapshotter := Snapshotter{}
	snapshotter.Configure(config, vault.NewClient("http://127.0.0.1:8200", &clientAPIStub, nil), []upload.Uploader{&uploaderStub})

	frequency, _ := snapshotter.TakeSnapshot(context.Background())

	if frequency != config.Frequency {
		t.Fatalf("TakeSnapshot did return wron frequency: expected %v, got: %v", config.Frequency, frequency)
	}
}

type snapshotterVaultClientAPIStub struct {
	leader          bool
	snapshotRuntime time.Duration
	snapshotData    string
}

func (stub *snapshotterVaultClientAPIStub) TakeSnapshot(ctx context.Context, writer io.Writer) error {
	if stub.snapshotData != "" {
		_, err := writer.Write([]byte(stub.snapshotData))
		if err != nil {
			return err
		}
	}

	select {
	case <-ctx.Done():
	case <-time.After(stub.snapshotRuntime):
	}

	return nil
}

func (stub *snapshotterVaultClientAPIStub) IsLeader() (bool, error) {
	return stub.leader, nil
}

func (stub *snapshotterVaultClientAPIStub) AuthAPI() auth.VaultAuthAPI {
	return nil
}

type uploaderStub struct {
	uploaded    bool
	uploadData  string
	uploadFails bool
}

func (stub *uploaderStub) Upload(ctx context.Context, reader io.Reader, currentTs int64, retain int) error {
	stub.uploaded = true
	if stub.uploadFails {
		return errors.New("upload failed")
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	stub.uploadData = string(data)
	return nil
}

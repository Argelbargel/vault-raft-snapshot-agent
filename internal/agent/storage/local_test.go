package storage

import (
	"bytes"
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/thoas/go-funk"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalDestination(t *testing.T) {
	config := LocalStorageConfig{Path: "/test"}
	impl, _ := createLocalStorageController(context.Background(), config)

	assert.Equal(t, "local path /test", impl.Destination())
}

func TestLocalUploadSnapshotFailsIfFileCannotBeCreated(t *testing.T) {
	impl := localStorageImpl{"./does/not/exist"}

	err := impl.UploadSnapshot(context.Background(), "test", &bytes.Buffer{})

	assert.Error(t, err, "UploadSnapshot() should fail if file could not be created!")
}

func TestLocalUploadeSnapshotCreatesFile(t *testing.T) {
	impl := localStorageImpl{t.TempDir()}
	snapshotData := []byte("test")

	err := impl.UploadSnapshot(context.Background(), "test.snap", bytes.NewReader(snapshotData))

	assert.NoError(t, err, "UploadSnapshot() failed unexpectedly!")

	backupData, err := os.ReadFile(fmt.Sprintf("%s/test.snap", impl.path))

	assert.NoError(t, err, "UploadSnapshot() failed unexpectedly!")
	assert.Equal(t, snapshotData, backupData)
}

func TestLocalDeleteSnapshot(t *testing.T) {
	impl := localStorageImpl{t.TempDir()}
	snapshotData := []byte("test")

	defer func() {
		_ = os.RemoveAll(filepath.Dir(impl.path))
	}()

	err := impl.UploadSnapshot(context.Background(), "test.snap", bytes.NewReader(snapshotData))
	assert.NoError(t, err, "UploadSnapshot() failed unexpectedly!")

	info, err := os.Stat(fmt.Sprintf("%s/test.snap", impl.path))
	assert.NoError(t, err, "could not get info for snapshot: %v", err)

	err = impl.DeleteSnapshot(context.Background(), info)
	assert.NoError(t, err, "DeleteSnapshot() failed unexpectedly!")

	_, err = os.Stat(fmt.Sprintf("%s/test.snap", impl.path))
	assert.Error(t, err)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestLocalListSnapshots(t *testing.T) {
	impl := localStorageImpl{t.TempDir()}

	var expectedSnaphotNames []string
	for i := 0; i < 3; i++ {
		expectedSnaphotNames = append(expectedSnaphotNames, createEmptySnapshot(t, impl.path, "test", ".snap").Name())
	}

	listedSnapshots, err := impl.ListSnapshots(context.Background(), "test", ".snap")
	listedSnapshotNames := funk.Map(listedSnapshots, func(s os.FileInfo) string { return s.Name() })

	assert.NoError(t, err)
	assert.Equal(t, len(expectedSnaphotNames), len(listedSnapshots))
	assert.ElementsMatch(t, expectedSnaphotNames, listedSnapshotNames)
}

func TestLocalGetLastModifiedTime(t *testing.T) {
	impl := localStorageImpl{t.TempDir()}

	snapshot := createEmptySnapshot(t, impl.path, "test", ".snap")
	assert.Equal(t, snapshot.ModTime(), impl.GetLastModifiedTime(snapshot))
}

func createEmptySnapshot(t *testing.T, dir string, prefix string, suffix string) os.FileInfo {
	t.Helper()

	file, err := os.Create(fmt.Sprintf("%s/%s-%d%s", dir, prefix, rand.Int(), suffix))
	assert.NoError(t, err, "could not create temp-file")

	info, err := os.Stat(file.Name())
	assert.NoError(t, err, "could not stat temp-file")

	err = file.Close()
	assert.NoError(t, err, "could not close temp-file")

	return info
}

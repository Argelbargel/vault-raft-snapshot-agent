package upload

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLocalUploaderDestination(t *testing.T) {
	config := LocalConfig{Path: "/test"}
	uploader, err := newLocalUploader(config)

	assert.NoError(t, err, "newLocalUploader failed unexpectedly")
	assert.Equal(t, "local path /test", uploader.Destination())
}

func TestLocalUploaderFailsIfFileCannotBeCreated(t *testing.T) {
	config := LocalConfig{Path: "./does/not/exist"}
	uploader, _ := newLocalUploader(config)

	err := uploader.Upload(context.Background(), &bytes.Buffer{}, time.Now().Unix(), 0)

	assert.Error(t, err, "Upload() should fail if file could not be created!")
}

func TestLocalUploaderCreatesLocalFile(t *testing.T) {
	config := LocalConfig{Path: createSnapshotPath(t)}
	uploader, _ := newLocalUploader(config)
	snapshotData := []byte("test")
	currentTs := time.Now().Unix()

	defer func() {
		_ = os.RemoveAll(filepath.Dir(config.Path))
	}()

	err := uploader.Upload(context.Background(), bytes.NewReader(snapshotData), currentTs, 0)

	assert.NoError(t, err, "Upload() failed unexpectedly!")

	backupData, err := os.ReadFile(fmt.Sprintf("%s/%s-%d%s", config.Path, snapshotFileName, currentTs, snapshotFileExt))

	assert.NoError(t, err, "Upload() failed unexpectedly!")
	assert.Equal(t, snapshotData, backupData)
}

func TestLocalUploaderDeletesOlderSnapshots(t *testing.T) {
	config := LocalConfig{Path: createSnapshotPath(t)}
	uploader, _ := newLocalUploader(config)
	snapshotData := []byte("test")

	defer func() {
		_ = os.RemoveAll(filepath.Dir(config.Path))
	}()

	ts := time.Now()
	for i := 0; i < 3; i++ {
		ts = ts.Add(time.Second)
		err := uploader.Upload(context.Background(), bytes.NewReader(snapshotData), ts.Unix(), 0)
		assert.NoError(t, err, "Upload() failed unexpectedly!")
	}

	assertNumberOfFiles(t, 3, config.Path)

	ts = ts.Add(time.Second)
	err := uploader.Upload(context.Background(), bytes.NewReader(snapshotData), ts.Unix(), 2)
	assert.NoError(t, err, "Upload() failed unexpectedly!")

	assertNumberOfFiles(t, 2, config.Path)
}

func assertNumberOfFiles(t *testing.T, expected int, path string) {
	t.Helper()

	files, err := os.ReadDir(path)
	assert.NoErrorf(t, err, "could not list files in %s: %v", path, err)

	if len(files) != expected {
		t.Errorf("%s does not contain exactly %d files, got: %d", path, expected, len(files))
	}
}

func createSnapshotPath(t *testing.T) string {
	t.Helper()

	path := fmt.Sprintf("%s/snapshots-%d-%d/", os.TempDir(), time.Now().Unix(), rand.Int())
	if err := os.MkdirAll(filepath.Dir(path), 0777); err != nil && !errors.Is(err, os.ErrExist) {
		t.Fatalf("could not create path %s: %v", path, err)
	}

	return path
}

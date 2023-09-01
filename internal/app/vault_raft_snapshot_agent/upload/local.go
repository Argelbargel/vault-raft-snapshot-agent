package upload

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

var snapshotFileName string = "raft_snapshot-"
var snapshotFileExt string = ".snap"

type LocalConfig struct {
	Path  string `validate:"required_if=Empty false,omitempty,dir"`
	Empty bool
}

type localUploader struct {
	path string
}

func newLocalUploader(config LocalConfig) (*localUploader, error) {
	return &localUploader{
		config.Path,
	}, nil
}

func (u *localUploader) Destination() string {
	return fmt.Sprintf("local path %s", u.path)
}

func (u *localUploader) Upload(ctx context.Context, reader io.Reader, currentTs int64, retain int) error {
	fileName := fmt.Sprintf("%s/%s-%d%s", u.path, snapshotFileName, currentTs, snapshotFileExt)

	file, err := os.Create(fileName)
	if err != nil {
		return fmt.Errorf("error creating local file %s: %w", fileName, err)
	}

	defer func() {
		_ = file.Close()
	}()

	if _, err = io.Copy(file, reader); err != nil {
		return fmt.Errorf("error writing snapshot to local file %s: %w", fileName, err)
	}

	if retain > 0 {
		return u.delete(retain)
	}

	return nil
}

func (u *localUploader) delete(retain int) error {
	existingSnapshots, err := u.listUploadedSnapshotsDescending(snapshotFileName)
	if err != nil {
		return fmt.Errorf("error getting existing snapshots from local storage: %w", err)
	}

	if len(existingSnapshots) > int(retain) {
		filesToDelete := existingSnapshots[retain:]

		for _, f := range filesToDelete {
			if err := os.Remove(fmt.Sprintf("%s/%s", u.path, f.Name())); err != nil {
				return fmt.Errorf("error deleting local snapshot %s: %w", f.Name(), err)
			}
		}
	}

	return nil
}

func (u *localUploader) listUploadedSnapshotsDescending(keyPrefix string) ([]os.FileInfo, error) {
	var result []os.FileInfo

	files, err := os.ReadDir(u.path)

	if err != nil {
		return result, fmt.Errorf("error reading local directory: %w", err)
	}

	for _, file := range files {
		if strings.Contains(file.Name(), keyPrefix) && strings.HasSuffix(file.Name(), ".snap") {
			info, err := file.Info()
			if err != nil {
				return result, fmt.Errorf("error getting local file info: %w", err)
			}
			result = append(result, info)
		}
	}

	timestamp := func(f1, f2 *os.FileInfo) bool {
		file1 := *f1
		file2 := *f2
		return file1.ModTime().After(file2.ModTime())
	}

	localBy(timestamp).Sort(result)

	return result, nil
}

// implementation of Sort interface for fileInfo
type localBy func(f1, f2 *os.FileInfo) bool

func (by localBy) Sort(files []os.FileInfo) {
	fs := &fileSorter{
		files: files,
		by:    by, // The Sort method's receiver is the function (closure) that defines the sort order.
	}
	sort.Sort(fs)
}

type fileSorter struct {
	files []os.FileInfo
	by    localBy
}

func (s *fileSorter) Len() int {
	return len(s.files)
}

func (s *fileSorter) Less(i, j int) bool {
	return s.by(&s.files[i], &s.files[j])
}

func (s *fileSorter) Swap(i, j int) {
	s.files[i], s.files[j] = s.files[j], s.files[i]
}

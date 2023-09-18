package upload

import (
	"context"
	"fmt"
	"go.uber.org/multierr"
	"io"
	"os"
	"strings"
)

type LocalUploaderConfig struct {
	Path  string `validate:"required_if=Empty false,omitempty,dir"`
	Empty bool
}

type localUploaderImpl struct {
	path string
}

func createLocalUploader(config LocalUploaderConfig) uploader[LocalUploaderConfig, any, os.FileInfo] {
	return uploader[LocalUploaderConfig, any, os.FileInfo]{
		config,
		localUploaderImpl{
			path: config.Path,
		},
	}
}

// nolint:unused
// implements interface uploaderImpl
func (u localUploaderImpl) destination(config LocalUploaderConfig) string {
	return fmt.Sprintf("local path %s", config.Path)
}

// nolint:unused
// implements interface uploaderImpl
func (u localUploaderImpl) connect(_ context.Context, _ LocalUploaderConfig) (any, error) {
	return nil, nil
}

func (u localUploaderImpl) uploadSnapshot(_ context.Context, _ any, name string, data io.Reader) error {
	fileName := fmt.Sprintf("%s/%s", u.path, name)

	file, err := os.Create(fileName)
	if err != nil {
		return err
	}

	_, err = io.Copy(file, data)

	return multierr.Append(err, file.Close())
}

func (u localUploaderImpl) deleteSnapshot(_ context.Context, _ any, snapshot os.FileInfo) error {
	if err := os.Remove(fmt.Sprintf("%s/%s", u.path, snapshot.Name())); err != nil {
		return err
	}

	return nil
}

func (u localUploaderImpl) listSnapshots(_ context.Context, _ any, prefix string, ext string) ([]os.FileInfo, error) {
	var snapshots []os.FileInfo

	files, err := os.ReadDir(u.path)
	if err != nil {
		return snapshots, err
	}

	for _, file := range files {
		if strings.HasPrefix(file.Name(), prefix) && strings.HasSuffix(file.Name(), ext) {
			info, err := file.Info()
			if err != nil {
				return snapshots, err
			}
			snapshots = append(snapshots, info)
		}
	}

	return snapshots, nil
}

func (u localUploaderImpl) compareSnapshots(a, b os.FileInfo) int {
	return a.ModTime().Compare(b.ModTime())
}

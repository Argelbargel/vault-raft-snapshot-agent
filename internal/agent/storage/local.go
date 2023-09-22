package storage

import (
	"context"
	"fmt"
	"go.uber.org/multierr"
	"io"
	"os"
	"strings"
	"time"
)

type LocalStorageConfig struct {
	storageConfig `mapstructure:",squash"`
	Path          string `validate:"required_if=Empty false,omitempty,dir"`
	Empty         bool
}

type localStorageImpl struct {
	path string
}

func createLocalStorageController(_ context.Context, config LocalStorageConfig) (*storageControllerImpl[os.FileInfo], error) {
	return newStorageController[os.FileInfo](
		config.storageConfig,
		fmt.Sprintf("local path %s", config.Path),
		localStorageImpl{
			path: config.Path,
		},
	), nil
}

func (u localStorageImpl) UploadSnapshot(_ context.Context, name string, data io.Reader) error {
	fileName := fmt.Sprintf("%s/%s", u.path, name)

	file, err := os.Create(fileName)
	if err != nil {
		return err
	}

	_, err = io.Copy(file, data)

	return multierr.Append(err, file.Close())
}

func (u localStorageImpl) DeleteSnapshot(_ context.Context, snapshot os.FileInfo) error {
	if err := os.Remove(fmt.Sprintf("%s/%s", u.path, snapshot.Name())); err != nil {
		return err
	}

	return nil
}

func (u localStorageImpl) ListSnapshots(_ context.Context, prefix string, ext string) ([]os.FileInfo, error) {
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

func (u localStorageImpl) GetLastModifiedTime(snapshot os.FileInfo) time.Time {
	return snapshot.ModTime()
}

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

func (conf LocalStorageConfig) Destination() string {
	return fmt.Sprintf("local path %s", conf.Path)
}

func (conf LocalStorageConfig) CreateController(context.Context) (StorageController, error) {
	return newStorageController[os.FileInfo](
		conf.storageConfig,
		localStorageImpl{
			path: conf.Path,
		},
	), nil
}

func (u localStorageImpl) uploadSnapshot(_ context.Context, name string, data io.Reader) error {
	fileName := fmt.Sprintf("%s/%s", u.path, name)

	file, err := os.Create(fileName)
	if err != nil {
		return err
	}

	_, err = io.Copy(file, data)

	return multierr.Append(err, file.Close())
}

func (u localStorageImpl) deleteSnapshot(_ context.Context, snapshot os.FileInfo) error {
	if err := os.Remove(fmt.Sprintf("%s/%s", u.path, snapshot.Name())); err != nil {
		return err
	}

	return nil
}

func (u localStorageImpl) listSnapshots(_ context.Context, prefix string, ext string) ([]os.FileInfo, error) {
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

func (u localStorageImpl) getLastModifiedTime(snapshot os.FileInfo) time.Time {
	return snapshot.ModTime()
}

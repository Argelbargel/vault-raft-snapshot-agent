package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	gcpStorage "cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

type GCPStorageConfig struct {
	storageConfig `mapstructure:",squash"`
	Bucket        string `validate:"required_if=Empty false"`
	Empty         bool
}

type gcpStorageImpl struct {
	bucket *gcpStorage.BucketHandle
}

func (conf GCPStorageConfig) Destination() string {
	return fmt.Sprintf("gcp bucket %s", conf.Bucket)
}

func (conf GCPStorageConfig) CreateController(ctx context.Context) (StorageController, error) {
	client, err := gcpStorage.NewClient(ctx)
	if err != nil {
		return nil, err
	}

	return newStorageController[gcpStorage.ObjectAttrs](
		conf.storageConfig,
		gcpStorageImpl{client.Bucket(conf.Bucket)},
	), nil
}

// nolint:unused
// implements interface storage
func (u gcpStorageImpl) uploadSnapshot(ctx context.Context, name string, data io.Reader, _ int64) error {
	obj := u.bucket.Object(name)
	w := obj.NewWriter(ctx)

	if _, err := io.Copy(w, data); err != nil {
		return err
	}

	if err := w.Close(); err != nil {
		return err
	}

	return nil
}

// nolint:unused
// implements interface storage
func (u gcpStorageImpl) deleteSnapshot(ctx context.Context, snapshot gcpStorage.ObjectAttrs) error {
	obj := u.bucket.Object(snapshot.Name)
	if err := obj.Delete(ctx); err != nil {
		return err
	}

	return nil
}

// nolint:unused
// implements interface storage
func (u gcpStorageImpl) listSnapshots(ctx context.Context, prefix string, _ string) ([]gcpStorage.ObjectAttrs, error) {
	var result []gcpStorage.ObjectAttrs

	query := &gcpStorage.Query{Prefix: prefix}
	it := u.bucket.Objects(ctx, query)

	for {
		attrs, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return result, err
		}
		result = append(result, *attrs)
	}

	return result, nil
}

// nolint:unused
// implements interface storage
func (u gcpStorageImpl) getLastModifiedTime(snapshot gcpStorage.ObjectAttrs) time.Time {
	return snapshot.Updated
}

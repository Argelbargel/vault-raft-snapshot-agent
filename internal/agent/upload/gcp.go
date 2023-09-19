package upload

import (
	"context"
	"fmt"
	"io"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

type GCPUploaderConfig struct {
	Bucket string `validate:"required_if=Empty false"`
	Empty  bool
}

type gcpUploaderImpl struct{}

func createGCPUploader(config GCPUploaderConfig) uploader[GCPUploaderConfig, *storage.BucketHandle, storage.ObjectAttrs] {
	return uploader[GCPUploaderConfig, *storage.BucketHandle, storage.ObjectAttrs]{config, gcpUploaderImpl{}}
}

// nolint:unused
// implements interface uploaderImpl
func (u gcpUploaderImpl) destination(config GCPUploaderConfig) string {
	return fmt.Sprintf("gcp bucket %s", config.Bucket)
}

// nolint:unused
// implements interface uploaderImpl
func (u gcpUploaderImpl) connect(ctx context.Context, config GCPUploaderConfig) (*storage.BucketHandle, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}

	return client.Bucket(config.Bucket), nil
}

// nolint:unused
// implements interface uploaderImpl
func (u gcpUploaderImpl) uploadSnapshot(ctx context.Context, client *storage.BucketHandle, name string, data io.Reader) error {
	obj := client.Object(name)
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
// implements interface uploaderImpl
func (u gcpUploaderImpl) deleteSnapshot(ctx context.Context, client *storage.BucketHandle, snapshot storage.ObjectAttrs) error {
	obj := client.Object(snapshot.Name)
	if err := obj.Delete(ctx); err != nil {
		return err
	}

	return nil
}

// nolint:unused
// implements interface uploaderImpl
func (u gcpUploaderImpl) listSnapshots(ctx context.Context, client *storage.BucketHandle, prefix string, _ string) ([]storage.ObjectAttrs, error) {
	var result []storage.ObjectAttrs

	query := &storage.Query{Prefix: prefix}
	it := client.Objects(ctx, query)

	for {
		attrs, err := it.Next()
		if err == iterator.Done {
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
// implements interface uploaderImpl
func (u gcpUploaderImpl) compareSnapshots(a, b storage.ObjectAttrs) int {
	return a.Updated.Compare(b.Updated)
}

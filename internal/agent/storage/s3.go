package storage

import (
	"context"
	"fmt"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/config/secret"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/logging"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"io"
	"strings"
	"time"
)

type S3StorageConfig struct {
	StorageControllerConfig `mapstructure:",squash"`
	Endpoint                string        `validate:"required_if=Empty false"`
	Bucket                  string        `validate:"required_if=Empty false"`
	AccessKeyId             secret.Secret `default:"env://S3_ACCESS_KEY_ID"`
	AccessKey               secret.Secret `default:"env://S3_SECRET_ACCESS_KEY" validate:"required_with=AccessKeyId"`
	SessionToken            secret.Secret `default:"env://S3_SESSION_TOKEN"`
	Region                  secret.Secret
	Insecure                bool
	Empty                   bool
}

type s3StorageImpl struct {
	client *minio.Client
	bucket string
}

func (conf S3StorageConfig) Destination() string {
	return fmt.Sprintf("s3 bucket %s at %s", conf.Bucket, conf.Endpoint)
}

func (conf S3StorageConfig) CreateController(ctx context.Context) (StorageController, error) {
	client, err := conf.createClient(ctx)
	if err != nil {
		return nil, err
	}

	return newStorageController[minio.ObjectInfo](
		conf.StorageControllerConfig,
		s3StorageImpl{
			client: client,
			bucket: conf.Bucket,
		},
	), nil

}

func (conf S3StorageConfig) createClient(ctx context.Context) (*minio.Client, error) {
	accessKeyId, err := conf.AccessKeyId.Resolve(false)
	if err != nil {
		return nil, err
	}

	accessKey, err := conf.AccessKey.Resolve(accessKeyId != "")
	if err != nil {
		return nil, err
	}

	sessionToken, err := conf.SessionToken.Resolve(false)
	if err != nil {
		return nil, err
	}

	region, err := conf.Region.Resolve(false)
	if err != nil {
		return nil, err
	}

	client, err := minio.New(conf.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyId, accessKey, sessionToken),
		Secure: !conf.Insecure,
		Region: region,
	})
	if err != nil {
		return nil, err
	}

	err = client.MakeBucket(ctx, conf.Bucket, minio.MakeBucketOptions{Region: region})
	if err != nil {
		exists, err := client.BucketExists(ctx, conf.Bucket)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, fmt.Errorf("bucket %s does not exist", conf.Bucket)
		}
	}

	logging.Debug("Successfully connected", "destination", conf.Destination())

	return client, nil
}

// nolint:unused
// implements interface storage
func (s s3StorageImpl) uploadSnapshot(ctx context.Context, name string, data io.Reader, size int64) error {
	_, err := s.client.PutObject(ctx, s.bucket, name, data, size, minio.PutObjectOptions{})
	if err != nil {
		return err
	}

	return nil
}

// nolint:unused
// implements interface storage
func (s s3StorageImpl) deleteSnapshot(ctx context.Context, snapshot minio.ObjectInfo) error {
	return s.client.RemoveObject(ctx, s.bucket, snapshot.Key, minio.RemoveObjectOptions{ForceDelete: true})
}

// nolint:unused
// implements interface storage
func (s s3StorageImpl) listSnapshots(ctx context.Context, prefix string, ext string) ([]minio.ObjectInfo, error) {
	var result []minio.ObjectInfo
	objectCh := s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: prefix})

	for snapshot := range objectCh {
		if snapshot.Err != nil {
			return nil, snapshot.Err
		}
		if strings.HasSuffix(snapshot.Key, ext) {
			result = append(result, snapshot)
		}
	}

	return result, nil
}

// nolint:unused
// implements interface storage
func (s s3StorageImpl) getLastModifiedTime(snapshot minio.ObjectInfo) time.Time {
	return snapshot.LastModified
}

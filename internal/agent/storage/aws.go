package storage

import (
	"context"
	"fmt"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/config/secret"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	s3Config "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type AWSStorageConfig struct {
	storageConfig           `mapstructure:",squash"`
	AccessKeyId             secret.Secret `default:"env://AWS_ACCESS_KEY_ID"`
	AccessKey               secret.Secret `default:"env://AWS_SECRET_ACCESS_KEY" validate:"required_with=AccessKeyId"`
	SessionToken            secret.Secret `default:"env://AWS_SESSION_TOKEN"`
	Region                  secret.Secret `default:"env://AWS_DEFAULT_REGION"`
	Endpoint                secret.Secret `default:"env://AWS_ENDPOINT_URL"`
	Bucket                  string        `validate:"required_if=Empty false"`
	KeyPrefix               string        `mapstructure:",omitifempty"`
	UseServerSideEncryption bool
	ForcePathStyle          bool
	Empty                   bool
}

type awsStorageImpl struct {
	client    *s3.Client
	keyPrefix string
	bucket    string
	sse       bool
}

func (conf AWSStorageConfig) Destination() string {
	return fmt.Sprintf("aws s3 bucket %s at %s", conf.Bucket, conf.Endpoint)
}

func (conf AWSStorageConfig) CreateController(ctx context.Context) (StorageController, error) {
	keyPrefix := ""
	if conf.KeyPrefix != "" {
		keyPrefix = fmt.Sprintf("%s/", conf.KeyPrefix)
	}

	client, err := conf.createClient(ctx)
	if err != nil {
		return nil, err
	}

	return newStorageController[types.Object](
		conf.storageConfig,
		awsStorageImpl{
			client:    client,
			keyPrefix: keyPrefix,
			bucket:    conf.Bucket,
			sse:       conf.UseServerSideEncryption,
		},
	), nil

}

func (conf AWSStorageConfig) createClient(ctx context.Context) (*s3.Client, error) {
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

	clientConfig, err := s3Config.LoadDefaultConfig(ctx, s3Config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load default aws config: %w", err)
	}

	if accessKeyId != "" {
		clientConfig.Credentials = credentials.NewStaticCredentialsProvider(accessKeyId, accessKey, sessionToken)
	}

	endpoint, err := conf.Endpoint.Resolve(false)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(clientConfig, func(o *s3.Options) {
		o.UsePathStyle = conf.ForcePathStyle
		if conf.Endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
	})

	return client, nil
}

// nolint:unused
// implements interface storage
func (s awsStorageImpl) uploadSnapshot(ctx context.Context, name string, data io.Reader) error {
	input := &s3.PutObjectInput{
		Bucket: &s.bucket,
		Key:    aws.String(s.keyPrefix + name),
		Body:   data,
	}

	if s.sse {
		input.ServerSideEncryption = types.ServerSideEncryptionAes256
	}

	uploader := manager.NewUploader(s.client)
	if _, err := uploader.Upload(ctx, input); err != nil {
		return err
	}

	return nil
}

// nolint:unused
// implements interface storage
func (s awsStorageImpl) deleteSnapshot(ctx context.Context, snapshot types.Object) error {
	input := &s3.DeleteObjectInput{
		Bucket: &s.bucket,
		Key:    snapshot.Key,
	}

	if _, err := s.client.DeleteObject(ctx, input); err != nil {
		return err
	}

	return nil
}

// nolint:unused
// implements interface storage
func (s awsStorageImpl) listSnapshots(ctx context.Context, prefix string, ext string) ([]types.Object, error) {
	var result []types.Object

	existingSnapshotList, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: &s.bucket,
		Prefix: aws.String(s.keyPrefix),
	})

	if err != nil {
		return result, err
	}

	for _, obj := range existingSnapshotList.Contents {
		if strings.HasSuffix(*obj.Key, ext) && strings.Contains(*obj.Key, prefix) {
			result = append(result, obj)
		}
	}

	return result, nil
}

// nolint:unused
// implements interface storage
func (s awsStorageImpl) getLastModifiedTime(snapshot types.Object) time.Time {
	return *snapshot.LastModified
}

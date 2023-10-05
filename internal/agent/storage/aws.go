package storage

import (
	"context"
	"fmt"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/config/secret"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	awsCredentials "github.com/aws/aws-sdk-go-v2/credentials"
	awsS3Manager "github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	awsS3 "github.com/aws/aws-sdk-go-v2/service/s3"
	awsS3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type AWSStorageConfig struct {
	StorageControllerConfig `mapstructure:",squash"`
	AccessKeyId             secret.Secret `default:"env://AWS_ACCESS_KEY_ID"`
	AccessKey               secret.Secret `default:"env://AWS_SECRET_ACCESS_KEY" validate:"required_with=AccessKeyId"`
	SessionToken            secret.Secret `default:"env://AWS_SESSION_TOKEN"`
	Region                  secret.Secret `default:"env://AWS_DEFAULT_REGION"`
	Endpoint                secret.Secret `default:"env://AWS_ENDPOINT_URL"`
	Bucket                  string        `validate:"required"`
	KeyPrefix               string        `mapstructure:",omitifempty"`
	UseServerSideEncryption bool
	ForcePathStyle          bool
}

type awsStorageImpl struct {
	client    *awsS3.Client
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

	return newStorageController[awsS3Types.Object](
		conf.StorageControllerConfig,
		awsStorageImpl{
			client:    client,
			keyPrefix: keyPrefix,
			bucket:    conf.Bucket,
			sse:       conf.UseServerSideEncryption,
		},
	), nil

}

func (conf AWSStorageConfig) createClient(ctx context.Context) (*awsS3.Client, error) {
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

	clientConfig, err := awsConfig.LoadDefaultConfig(ctx, awsConfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load default aws config: %w", err)
	}

	if accessKeyId != "" {
		clientConfig.Credentials = awsCredentials.NewStaticCredentialsProvider(accessKeyId, accessKey, sessionToken)
	}

	endpoint, err := conf.Endpoint.Resolve(false)
	if err != nil {
		return nil, err
	}

	client := awsS3.NewFromConfig(clientConfig, func(o *awsS3.Options) {
		o.UsePathStyle = conf.ForcePathStyle
		if conf.Endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
	})

	return client, nil
}

// nolint:unused
// implements interface storage
func (s awsStorageImpl) uploadSnapshot(ctx context.Context, name string, data io.Reader, size int64) error {
	input := &awsS3.PutObjectInput{
		Bucket:        &s.bucket,
		Key:           aws.String(s.keyPrefix + name),
		Body:          data,
		ContentLength: size,
	}

	if s.sse {
		input.ServerSideEncryption = awsS3Types.ServerSideEncryptionAes256
	}

	uploader := awsS3Manager.NewUploader(s.client)
	if _, err := uploader.Upload(ctx, input); err != nil {
		return err
	}

	return nil
}

// nolint:unused
// implements interface storage
func (s awsStorageImpl) deleteSnapshot(ctx context.Context, snapshot awsS3Types.Object) error {
	input := &awsS3.DeleteObjectInput{
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
func (s awsStorageImpl) listSnapshots(ctx context.Context, prefix string, ext string) ([]awsS3Types.Object, error) {
	var result []awsS3Types.Object

	existingSnapshotList, err := s.client.ListObjectsV2(ctx, &awsS3.ListObjectsV2Input{
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
func (s awsStorageImpl) getLastModifiedTime(snapshot awsS3Types.Object) time.Time {
	return *snapshot.LastModified
}

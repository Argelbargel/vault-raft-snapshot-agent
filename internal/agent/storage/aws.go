package storage

import (
	"context"
	"fmt"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/secret"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
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

func createAWSStorageController(ctx context.Context, config AWSStorageConfig) (*storageControllerImpl[s3Types.Object], error) {
	keyPrefix := ""
	if config.KeyPrefix != "" {
		keyPrefix = fmt.Sprintf("%s/", config.KeyPrefix)
	}

	client, err := createS3Client(ctx, config)
	if err != nil {
		return nil, nil
	}

	return newStorageController[s3Types.Object](
		config.storageConfig,
		fmt.Sprintf("aws s3 bucket %s at %s", config.Bucket, config.Endpoint),
		awsStorageImpl{
			client:    client,
			keyPrefix: keyPrefix,
			bucket:    config.Bucket,
			sse:       config.UseServerSideEncryption,
		},
	), nil
}

func createS3Client(ctx context.Context, config AWSStorageConfig) (*s3.Client, error) {
	accessKeyId, err := config.AccessKeyId.Resolve(false)
	if err != nil {
		return nil, err
	}

	accessKey, err := config.AccessKey.Resolve(accessKeyId != "")
	if err != nil {
		return nil, err
	}

	sessionToken, err := config.SessionToken.Resolve(false)
	if err != nil {
		return nil, err
	}

	region, err := config.Region.Resolve(false)
	if err != nil {
		return nil, err
	}

	clientConfig, err := awsConfig.LoadDefaultConfig(ctx, awsConfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load default aws config: %w", err)
	}

	if accessKeyId != "" {
		clientConfig.Credentials = credentials.NewStaticCredentialsProvider(accessKeyId, accessKey, sessionToken)
	}

	endpoint, err := config.Endpoint.Resolve(false)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(clientConfig, func(o *s3.Options) {
		o.UsePathStyle = config.ForcePathStyle
		if config.Endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
	})

	return client, nil
}

// nolint:unused
// implements interface storage
func (u awsStorageImpl) UploadSnapshot(ctx context.Context, name string, data io.Reader) error {
	input := &s3.PutObjectInput{
		Bucket: &u.bucket,
		Key:    aws.String(u.keyPrefix + name),
		Body:   data,
	}

	if u.sse {
		input.ServerSideEncryption = s3Types.ServerSideEncryptionAes256
	}

	uploader := manager.NewUploader(u.client)
	if _, err := uploader.Upload(ctx, input); err != nil {
		return err
	}

	return nil
}

// nolint:unused
// implements interface storage
func (u awsStorageImpl) DeleteSnapshot(ctx context.Context, snapshot s3Types.Object) error {
	input := &s3.DeleteObjectInput{
		Bucket: &u.bucket,
		Key:    snapshot.Key,
	}

	if _, err := u.client.DeleteObject(ctx, input); err != nil {
		return err
	}

	return nil
}

// nolint:unused
// implements interface storage
func (u awsStorageImpl) ListSnapshots(ctx context.Context, prefix string, ext string) ([]s3Types.Object, error) {
	var result []s3Types.Object

	existingSnapshotList, err := u.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: &u.bucket,
		Prefix: aws.String(u.keyPrefix),
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
func (u awsStorageImpl) GetLastModifiedTime(snapshot s3Types.Object) time.Time {
	return *snapshot.LastModified
}

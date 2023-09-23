package storage

import (
	"context"
	"fmt"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/config/secret"
	"io"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
)

type AzureStorageConfig struct {
	storageConfig `mapstructure:",squash"`
	AccountName   secret.Secret `default:"env://AZURE_STORAGE_ACCOUNT" validate:"required_if=Empty false"`
	AccountKey    secret.Secret `default:"env://AZURE_STORAGE_KEY" validate:"required_if=Empty false"`
	Container     string        `validate:"required_if=Empty false"`
	CloudDomain   string        `default:"blob.core.windows.net" validate:"required_if=Empty false"`
	Empty         bool
}

type azureStorageImpl struct {
	client    *azblob.Client
	container string
}

func (conf AzureStorageConfig) Destination() string {
	return fmt.Sprintf("azure container %s at %s", conf.Container, conf.CloudDomain)
}

func (conf AzureStorageConfig) CreateController(context.Context) (StorageController, error) {
	client, err := createAzBlobClient(conf)
	if err != nil {
		return nil, err
	}

	return newStorageController[*container.BlobItem](
		conf.storageConfig,
		azureStorageImpl{client, conf.Container},
	), nil

}

func createAzBlobClient(config AzureStorageConfig) (*azblob.Client, error) {
	accountName, err := config.AccountName.Resolve(true)
	if err != nil {
		return nil, err
	}

	accountKey, err := config.AccountName.Resolve(true)
	if err != nil {
		return nil, err
	}

	credential, err := azblob.NewSharedKeyCredential(accountName, accountKey)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials for azure: %w", err)
	}

	serviceURL := fmt.Sprintf("https://%s.%s/", config.AccountName, config.CloudDomain)
	return azblob.NewClientWithSharedKeyCredential(serviceURL, credential, nil)
}

// nolint:unused
// implements interface storage
func (s azureStorageImpl) uploadSnapshot(ctx context.Context, name string, data io.Reader) error {
	uploadOptions := &azblob.UploadStreamOptions{
		BlockSize:   4 * 1024 * 1024,
		Concurrency: 16,
	}

	if _, err := s.client.UploadStream(ctx, s.container, name, data, uploadOptions); err != nil {
		return err
	}

	return nil
}

// nolint:unused
// implements interface storage
func (s azureStorageImpl) deleteSnapshot(ctx context.Context, snapshot *container.BlobItem) error {
	if _, err := s.client.DeleteBlob(ctx, s.container, *snapshot.Name, nil); err != nil {
		return err
	}

	return nil
}

// nolint:unused
// implements interface storage
func (s azureStorageImpl) listSnapshots(ctx context.Context, prefix string, _ string) ([]*container.BlobItem, error) {
	var results []*container.BlobItem

	var maxResults int32 = 500

	pager := s.client.NewListBlobsFlatPager(s.container, &azblob.ListBlobsFlatOptions{
		Prefix:     &prefix,
		MaxResults: &maxResults,
	})

	for pager.More() {
		resp, err := pager.NextPage(ctx)

		if err != nil {
			return results, err
		}

		results = append(results, resp.Segment.BlobItems...)
	}

	return results, nil
}

// nolint:unused
// implements interface storage
func (s azureStorageImpl) getLastModifiedTime(snapshot *container.BlobItem) time.Time {
	return *snapshot.Properties.LastModified
}

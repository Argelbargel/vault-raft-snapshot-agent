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

func createAzureStorageController(_ context.Context, config AzureStorageConfig) (*storageControllerImpl[*container.BlobItem], error) {
	client, err := createAzBlobClient(config)
	if err != nil {
		return nil, err
	}

	return newStorageController[*container.BlobItem](
		config.storageConfig,
		fmt.Sprintf("azure container %s at %s", config.Container, config.CloudDomain),
		azureStorageImpl{client, config.Container},
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
func (u azureStorageImpl) UploadSnapshot(ctx context.Context, name string, data io.Reader) error {
	uploadOptions := &azblob.UploadStreamOptions{
		BlockSize:   4 * 1024 * 1024,
		Concurrency: 16,
	}

	if _, err := u.client.UploadStream(ctx, u.container, name, data, uploadOptions); err != nil {
		return err
	}

	return nil
}

// nolint:unused
// implements interface storage
func (u azureStorageImpl) DeleteSnapshot(ctx context.Context, snapshot *container.BlobItem) error {
	if _, err := u.client.DeleteBlob(ctx, u.container, *snapshot.Name, nil); err != nil {
		return err
	}

	return nil
}

// nolint:unused
// implements interface storage
func (u azureStorageImpl) ListSnapshots(ctx context.Context, prefix string, _ string) ([]*container.BlobItem, error) {
	var results []*container.BlobItem

	var maxResults int32 = 500

	pager := u.client.NewListBlobsFlatPager(u.container, &azblob.ListBlobsFlatOptions{
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
func (u azureStorageImpl) GetLastModifiedTime(snapshot *container.BlobItem) time.Time {
	return *snapshot.Properties.LastModified
}

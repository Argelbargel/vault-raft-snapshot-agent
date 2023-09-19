package upload

import (
	"context"
	"fmt"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/secret"
	"io"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
)

type AzureUploaderConfig struct {
	AccountName secret.Secret `default:"env://AZURE_STORAGE_ACCOUNT" validate:"required_if=Empty false"`
	AccountKey  secret.Secret `default:"env://AZURE_STORAGE_KEY" validate:"required_if=Empty false"`
	Container   string        `validate:"required_if=Empty false"`
	CloudDomain string        `default:"blob.core.windows.net" validate:"required_if=Empty false"`
	Empty       bool
}

type azureUploaderImpl struct {
	container string
}

func createAzureUploader(config AzureUploaderConfig) uploader[AzureUploaderConfig, *azblob.Client, *container.BlobItem] {
	return uploader[AzureUploaderConfig, *azblob.Client, *container.BlobItem]{
		config, azureUploaderImpl{config.Container},
	}
}

// nolint:unused
// implements interface uploaderImpl
func (u azureUploaderImpl) destination(config AzureUploaderConfig) string {
	return fmt.Sprintf("azure container %s at %s", config.Container, config.CloudDomain)
}

// nolint:unused
// implements interface uploaderImpl
func (u azureUploaderImpl) connect(_ context.Context, config AzureUploaderConfig) (*azblob.Client, error) {
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
// implements interface uploaderImpl
func (u azureUploaderImpl) uploadSnapshot(ctx context.Context, client *azblob.Client, name string, data io.Reader) error {
	uploadOptions := &azblob.UploadStreamOptions{
		BlockSize:   4 * 1024 * 1024,
		Concurrency: 16,
	}

	if _, err := client.UploadStream(ctx, u.container, name, data, uploadOptions); err != nil {
		return err
	}

	return nil
}

// nolint:unused
// implements interface uploaderImpl
func (u azureUploaderImpl) deleteSnapshot(ctx context.Context, client *azblob.Client, snapshot *container.BlobItem) error {
	if _, err := client.DeleteBlob(ctx, u.container, *snapshot.Name, nil); err != nil {
		return err
	}

	return nil
}

// nolint:unused
// implements interface uploaderImpl
func (u azureUploaderImpl) listSnapshots(ctx context.Context, client *azblob.Client, prefix string, _ string) ([]*container.BlobItem, error) {
	var results []*container.BlobItem

	var maxResults int32 = 500

	pager := client.NewListBlobsFlatPager(u.container, &azblob.ListBlobsFlatOptions{
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
// implements interface uploaderImpl
func (u azureUploaderImpl) compareSnapshots(a, b *container.BlobItem) int {
	return a.Properties.LastModified.Compare(*b.Properties.LastModified)
}

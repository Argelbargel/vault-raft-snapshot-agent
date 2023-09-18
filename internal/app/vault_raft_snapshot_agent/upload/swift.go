package upload

import (
	"context"
	"fmt"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/app/vault_raft_snapshot_agent/secret"
	"io"
	"time"

	"github.com/ncw/swift/v2"
)

type SwiftUploaderConfig struct {
	Container string        `validate:"required_if=Empty false"`
	UserName  secret.Secret `default:"env://SWIFT_USERNAME" validate:"required_if=Empty false"`
	ApiKey    secret.Secret `default:"env://SWIFT_API_KEY" valide:"required_if=Empty false"`
	Region    secret.Secret `default:"env://SWIFT_REGION"`
	AuthUrl   string        `validate:"required_if=Empty false,omitempty,http_url"`
	Domain    string        `validate:"omitempty,http_url"`
	TenantId  string
	Timeout   time.Duration `default:"60s"`
	Empty     bool
}

type swiftUploaderImpl struct {
	container string
}

func createSwiftUploader(config SwiftUploaderConfig) uploader[SwiftUploaderConfig, *swift.Connection, swift.Object] {
	return uploader[SwiftUploaderConfig, *swift.Connection, swift.Object]{
		config,
		swiftUploaderImpl{config.Container},
	}
}

// nolint:unused
// implements interface uploaderImpl
func (u swiftUploaderImpl) destination(config SwiftUploaderConfig) string {
	return fmt.Sprintf("swift container %s", config.Container)
}

// nolint:unused
// implements interface uploaderImpl
func (u swiftUploaderImpl) connect(ctx context.Context, config SwiftUploaderConfig) (*swift.Connection, error) {
	username, err := config.UserName.Resolve(true)
	if err != nil {
		return nil, err
	}

	apiKey, err := config.ApiKey.Resolve(true)
	if err != nil {
		return nil, err
	}

	region, err := config.Region.Resolve(false)
	if err != nil {
		return nil, err
	}

	conn := swift.Connection{
		UserName: username,
		ApiKey:   apiKey,
		AuthUrl:  config.AuthUrl,
		Region:   region,
		TenantId: config.TenantId,
		Domain:   config.Domain,
		Timeout:  config.Timeout,
	}

	if err := conn.Authenticate(ctx); err != nil {
		return nil, fmt.Errorf("invalid credentials: %s", err)
	}

	if _, _, err := conn.Container(ctx, config.Container); err != nil {
		return nil, fmt.Errorf("invalid container %s: %s", config.Container, err)
	}

	return &conn, nil
}

// nolint:unused
// implements interface uploaderImpl
func (u swiftUploaderImpl) uploadSnapshot(ctx context.Context, client *swift.Connection, name string, data io.Reader) error {
	_, header, err := client.Container(ctx, u.container)
	if err != nil {
		return err
	}

	object, err := client.ObjectCreate(ctx, u.container, name, false, "", "", header)
	if err != nil {
		return err
	}

	if _, err := io.Copy(object, data); err != nil {
		return err
	}

	if err := object.Close(); err != nil {
		return err
	}

	return nil
}

// nolint:unused
// implements interface uploaderImpl
func (u swiftUploaderImpl) deleteSnapshot(ctx context.Context, client *swift.Connection, snapshot swift.Object) error {
	if err := client.ObjectDelete(ctx, u.container, snapshot.Name); err != nil {
		return err
	}

	return nil
}

// nolint:unused
// implements interface uploaderImpl
func (u swiftUploaderImpl) listSnapshots(ctx context.Context, client *swift.Connection, prefix string, _ string) ([]swift.Object, error) {
	return client.ObjectsAll(ctx, u.container, &swift.ObjectsOpts{Prefix: prefix})
}

// nolint:unused
// implements interface uploaderImpl
func (u swiftUploaderImpl) compareSnapshots(a, b swift.Object) int {
	return a.LastModified.Compare(b.LastModified)
}

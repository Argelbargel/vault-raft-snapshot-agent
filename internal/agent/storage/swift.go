package storage

import (
	"context"
	"fmt"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/config/secret"
	"io"
	"time"

	"github.com/ncw/swift/v2"
)

type SwiftStorageConfig struct {
	StorageControllerConfig `mapstructure:",squash"`
	Container               string        `validate:"required_if=Empty false"`
	UserName                secret.Secret `default:"env://SWIFT_USERNAME" validate:"required_if=Empty false"`
	ApiKey                  secret.Secret `default:"env://SWIFT_API_KEY" valide:"required_if=Empty false"`
	Region                  secret.Secret `default:"env://SWIFT_REGION"`
	AuthUrl                 string        `validate:"required_if=Empty false,omitempty,http_url"`
	Domain                  string        `validate:"omitempty,http_url"`
	TenantId                string
	Empty                   bool
}

type swiftStorageImpl struct {
	conn      *swift.Connection
	container string
}

func (conf SwiftStorageConfig) Destination() string {
	return fmt.Sprintf("swift container %s", conf.Container)
}

func (conf SwiftStorageConfig) CreateController(ctx context.Context) (StorageController, error) {
	conn, err := createSwiftConnection(ctx, conf)
	if err != nil {
		return nil, err
	}

	return newStorageController[swift.Object](
		conf.StorageControllerConfig,
		swiftStorageImpl{conn, conf.Container},
	), nil
}

func createSwiftConnection(ctx context.Context, config SwiftStorageConfig) (*swift.Connection, error) {
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
// implements interface storage
func (u swiftStorageImpl) uploadSnapshot(ctx context.Context, name string, data io.Reader, _ int64) error {
	_, header, err := u.conn.Container(ctx, u.container)
	if err != nil {
		return err
	}

	object, err := u.conn.ObjectCreate(ctx, u.container, name, false, "", "", header)
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
// implements interface storage
func (u swiftStorageImpl) deleteSnapshot(ctx context.Context, snapshot swift.Object) error {
	if err := u.conn.ObjectDelete(ctx, u.container, snapshot.Name); err != nil {
		return err
	}

	return nil
}

// nolint:unused
// implements interface storage
func (u swiftStorageImpl) listSnapshots(ctx context.Context, prefix string, _ string) ([]swift.Object, error) {
	return u.conn.ObjectsAll(ctx, u.container, &swift.ObjectsOpts{Prefix: prefix})
}

// nolint:unused
// implements interface storage
func (u swiftStorageImpl) getLastModifiedTime(snapshot swift.Object) time.Time {
	return snapshot.LastModified
}

package vault

import (
	"context"
	"fmt"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/vault/auth"
	"io"
	"time"

	"github.com/hashicorp/vault/api"
)

type ClientConfig struct {
	Url      string        `default:"http://127.0.0.1:8200" validate:"required,http_url"`
	Timeout  time.Duration `default:"60s"`
	Insecure bool
	Auth     auth.VaultAuthConfig
}

// Client is the public implementation of the client communicating with vault to authenticate and take snapshots
type Client[CLI any, AUTH clientVaultAPIAuth[CLI]] struct {
	api            clientVaultAPI[CLI, AUTH]
	auth           AUTH
	authExpiration time.Time
}

// internal definition of vault-api used by Client
type clientVaultAPI[CLI any, AUTH clientVaultAPIAuth[CLI]] interface {
	Address() string
	IsLeader() (bool, error)
	TakeSnapshot(context.Context, io.Writer) error
	RefreshAuth(context.Context, AUTH) (time.Duration, error)
}

// internal definition of vault-api used for authentication
type clientVaultAPIAuth[CLI any] interface {
	Login(context.Context, CLI) (time.Duration, error)
}

// internal implementation of the vault-api using a real vault-api-client
type clientVaultAPIImpl struct {
	client *api.Client
}

// CreateClient creates a Client using an api-implementation delegation to a real vault-api-client
func CreateClient(config ClientConfig) (*Client[*api.Client, clientVaultAPIAuth[*api.Client]], error) {
	impl, err := newClientVaultAPIImpl(config.Url, config.Insecure, config.Timeout)
	if err != nil {
		return nil, err
	}

	vaultAuth, err := auth.CreateVaultAuth(config.Auth)
	if err != nil {
		return nil, err
	}

	return NewClient[*api.Client, clientVaultAPIAuth[*api.Client]](impl, vaultAuth, time.Time{}), nil
}

// NewClient creates a Client using the given api-implementation and auth
// this function should only be used in tests!
func NewClient[CLI any, AUTH clientVaultAPIAuth[CLI]](api clientVaultAPI[CLI, AUTH], auth AUTH, tokenExpiration time.Time) *Client[CLI, AUTH] {
	return &Client[CLI, AUTH]{api, auth, tokenExpiration}
}

func (c *Client[CLI, AUTH]) TakeSnapshot(ctx context.Context, writer io.Writer) error {
	if err := c.refreshAuth(ctx); err != nil {
		return err
	}

	leader, err := c.api.IsLeader()
	if err != nil {
		return fmt.Errorf("unable to determine leader status for %s: %v", c.api.Address(), err)
	}

	if !leader {
		return fmt.Errorf("%s is not vault-leader-node", c.api.Address())
	}

	return c.api.TakeSnapshot(ctx, writer)
}

func (c *Client[CLI, AUTH]) refreshAuth(ctx context.Context) error {
	if c.authExpiration.Before(time.Now()) {
		leaseDuration, err := c.api.RefreshAuth(ctx, c.auth)
		if err != nil {
			return fmt.Errorf("could not refresh auth: %s", err)
		}
		c.authExpiration = time.Now().Add(leaseDuration / 2)
	}
	return nil
}

// creates a api-implementation using a real vault-api-client
func newClientVaultAPIImpl(address string, insecure bool, timeout time.Duration) (clientVaultAPIImpl, error) {
	apiConfig := api.DefaultConfig()
	apiConfig.Address = address
	apiConfig.HttpClient.Timeout = timeout

	tlsConfig := &api.TLSConfig{
		Insecure: insecure,
	}

	if err := apiConfig.ConfigureTLS(tlsConfig); err != nil {
		return clientVaultAPIImpl{}, err
	}

	client, err := api.NewClient(apiConfig)
	if err != nil {
		return clientVaultAPIImpl{}, err
	}

	return clientVaultAPIImpl{
		client,
	}, nil
}

func (impl clientVaultAPIImpl) Address() string {
	return impl.client.Address()
}

func (impl clientVaultAPIImpl) TakeSnapshot(ctx context.Context, writer io.Writer) error {
	return impl.client.Sys().RaftSnapshotWithContext(ctx, writer)
}

func (impl clientVaultAPIImpl) IsLeader() (bool, error) {
	leader, err := impl.client.Sys().Leader()
	if err != nil {
		return false, err
	}

	return leader.IsSelf, nil
}

func (impl clientVaultAPIImpl) RefreshAuth(ctx context.Context, auth clientVaultAPIAuth[*api.Client]) (time.Duration, error) {
	return auth.Login(ctx, impl.client)
}

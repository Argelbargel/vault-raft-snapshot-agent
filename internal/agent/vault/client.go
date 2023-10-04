package vault

import (
	"context"
	"fmt"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/logging"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/vault/auth"
	"io"
	"time"

	"github.com/hashicorp/vault/api"
)

type VaultClientConfig struct {
	Url      string        `default:"http://127.0.0.1:8200" validate:"required,http_url"`
	Timeout  time.Duration `default:"60s"`
	Insecure bool
	Auth     auth.VaultAuthConfig
}

// VaultClient is the public implementation of the client communicating with vault to authenticate and take snapshots
type VaultClient struct {
	api            vaultAPI
	auth           api.AuthMethod
	authExpiration time.Time
}

// internal definition of vault-api used by VaultClient
type vaultAPI interface {
	Address() string
	IsLeader() (bool, error)
	TakeSnapshot(context.Context, io.Writer) error
	RefreshAuth(context.Context, api.AuthMethod) (time.Duration, error)
}

// internal implementation of the vault-api using a real vault-api-client
type vaultAPIImpl struct {
	client *api.Client
}

// CreateClient creates a VaultClient using an api-implementation delegation to a real vault-api-client
func CreateClient(config VaultClientConfig) (*VaultClient, error) {
	impl, err := newClientVaultAPIImpl(config.Url, config.Insecure, config.Timeout)
	if err != nil {
		return nil, err
	}

	vaultAuth, err := auth.CreateVaultAuth(config.Auth)
	if err != nil {
		return nil, err
	}

	return NewClient(impl, vaultAuth, time.Time{}), nil
}

// NewClient creates a VaultClient using the given api-implementation and auth
// this function should only be used in tests!
func NewClient(api vaultAPI, auth api.AuthMethod, tokenExpiration time.Time) *VaultClient {
	return &VaultClient{api, auth, tokenExpiration}
}

func (c *VaultClient) TakeSnapshot(ctx context.Context, writer io.Writer) error {
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

func (c *VaultClient) refreshAuth(ctx context.Context) error {
	if c.authExpiration.Before(time.Now()) {
		leaseDuration, err := c.api.RefreshAuth(ctx, c.auth)
		if err != nil {
			return fmt.Errorf("could not refresh auth: %s", err)
		}
		c.authExpiration = time.Now().Add(leaseDuration / 2)
		logging.Debug("Successfully refreshed authentication", "authExpiration", c.authExpiration, "leaseDuration", leaseDuration)
	}
	return nil
}

// creates a api-implementation using a real vault-api-client
func newClientVaultAPIImpl(address string, insecure bool, timeout time.Duration) (vaultAPIImpl, error) {
	apiConfig := api.DefaultConfig()
	apiConfig.Address = address
	apiConfig.HttpClient.Timeout = timeout

	tlsConfig := &api.TLSConfig{
		Insecure: insecure,
	}

	if err := apiConfig.ConfigureTLS(tlsConfig); err != nil {
		return vaultAPIImpl{}, err
	}

	client, err := api.NewClient(apiConfig)
	if err != nil {
		return vaultAPIImpl{}, err
	}

	return vaultAPIImpl{
		client,
	}, nil
}

func (impl vaultAPIImpl) Address() string {
	return impl.client.Address()
}

func (impl vaultAPIImpl) TakeSnapshot(ctx context.Context, writer io.Writer) error {
	return impl.client.Sys().RaftSnapshotWithContext(ctx, writer)
}

func (impl vaultAPIImpl) IsLeader() (bool, error) {
	leader, err := impl.client.Sys().Leader()
	if err != nil {
		return false, err
	}

	return leader.IsSelf, nil
}

func (impl vaultAPIImpl) RefreshAuth(ctx context.Context, auth api.AuthMethod) (time.Duration, error) {
	authSecret, err := auth.Login(ctx, impl.client)
	if err != nil {
		return 0, err
	}

	tokenTTL, err := authSecret.TokenTTL()
	if err != nil {
		return 0, err
	}

	tokenPolicies, err := authSecret.TokenPolicies()
	if err != nil {
		return 0, err
	}

	logging.Debug("Successfully logged into vault", "ttl", tokenTTL, "policies", tokenPolicies)
	return tokenTTL, nil
}

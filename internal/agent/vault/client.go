package vault

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/logging"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/vault/auth"

	"github.com/hashicorp/vault/api"
)

// VaultClient is the public implementation of the client communicating with vault to authenticate and take snapshots
type VaultClient struct {
	api              vaultAPI
	nodes            []string
	autoDetectLeader bool
	auth             auth.VaultAuth
	connection       *api.Client
}

// internal definition of vault-api used by VaultClient
type vaultAPI interface {
	Connect(string) (*api.Client, error)
	GetLeader(context.Context, *api.Client) (bool, string)
	TakeSnapshot(context.Context, *api.Client, io.Writer) error
}

// internal implementation of the vault-api
type vaultAPIImpl struct {
	config *api.Config
}

// CreateClient creates a VaultClient using an api-implementation delegating to a real vault-api-client
func CreateClient(config VaultClientConfig) (*VaultClient, error) {
	nodes := []string{}

	for _, node := range config.Nodes.Urls {
		nodes = append(nodes, node)
	}

	auth, err := auth.CreateVaultAuth(config.Auth)
	if err != nil {
		return nil, err
	}

	api, err := newVaultAPIImpl(config.Insecure, config.Timeout)
	if err != nil {
		return nil, err
	}

	return NewClient(api, nodes, config.Nodes.AutoDetectLeader, auth), nil
}

// NewClient creates a VaultClient using the given api-implementation and auth
// this function should only be used in tests!
func NewClient(api vaultAPI, nodes []string, autoDetectLeader bool, auth auth.VaultAuth) *VaultClient {
	return &VaultClient{
		api:              api,
		nodes:            nodes,
		autoDetectLeader: autoDetectLeader,
		auth:             auth,
	}
}

func (c *VaultClient) TakeSnapshot(ctx context.Context, writer io.Writer) error {
	if err := c.ensureLeader(ctx); err != nil {
		return fmt.Errorf("could not (re-)connect to leader: %v", err)
	}

	return c.api.TakeSnapshot(ctx, c.connection, writer)
}

func (c *VaultClient) ensureLeader(ctx context.Context) error {
	leader, detectedLeader := c.isConnectedToLeader(ctx, c.connection)
	if leader {
		return nil
	}

	client, err := c.connectToLeader(ctx, c.selectNodesToConnect(slices.Clone(c.nodes), detectedLeader))
	if err != nil {
		return err
	}

	logging.Info("(re-)connected to leader", "node", client.Address())
	c.connection = client
	return nil
}

func (c *VaultClient) isConnectedToLeader(ctx context.Context, conn *api.Client) (bool, string) {
	if conn == nil {
		logging.Debug("currently not connected")
		return false, ""
	}

	if err := c.auth.Refresh(ctx, conn, false); err != nil {
		logging.Warn("unable to refresh auth", "node", conn.Address(), "err", err)
		return false, ""
	}

	leader, detectedLeader := c.api.GetLeader(ctx, conn)
	if !c.autoDetectLeader {
		logging.Debug("ignoring auto-detected-leader due to configuration-setting", "node", conn.Address(), "detectedLeader", detectedLeader)
		detectedLeader = ""
	}

	return leader, detectedLeader
}

func (c *VaultClient) connectToLeader(ctx context.Context, nodes []string) (*api.Client, error) {
	c.connection = nil

	for i, node := range nodes {
		logging.Debug("connecting...", "node", node)
		conn, err := c.api.Connect(node)
		if err != nil {
			logging.Warn("could not connect to node", "node", node, "err", err)
			continue
		}

		leader, detectedLeader := c.isConnectedToLeader(ctx, conn)
		logging.Debug("connection established", "node", node, "leader", leader, "detectedLeader", detectedLeader)
		if leader {
			return conn, nil
		}

		if detectedLeader != "" {
			return c.connectToLeader(ctx, c.selectNodesToConnect(nodes[i:], detectedLeader))
		}
	}

	return nil, errors.New("could not connect to leader")
}

func (c *VaultClient) selectNodesToConnect(nodes []string, detectedLeader string) []string {
	if detectedLeader != "" {
		logging.Info("auto-detected leader-node", "node", detectedLeader)
		nodes = slices.DeleteFunc(nodes, func(node string) bool {
			return detectedLeader == node
		})
		nodes = slices.Insert(nodes, 0, detectedLeader)
	}

	// when reconnecting, ignore current connected node
	if c.connection != nil {
		logging.Debug("ignoring currently connected node", "node", c.connection.Address())
		nodes = slices.DeleteFunc(nodes, func(node string) bool {
			return c.connection.Address() == node
		})
	}

	return nodes
}

// creates a api-implementation using a real vault-api-client
func newVaultAPIImpl(insecure bool, timeout time.Duration) (vaultAPIImpl, error) {
	tlsConfig := &api.TLSConfig{
		Insecure: insecure,
	}

	apiConfig := api.DefaultConfig()
	apiConfig.HttpClient.Timeout = timeout

	if err := apiConfig.ConfigureTLS(tlsConfig); err != nil {
		return vaultAPIImpl{}, err
	}

	return vaultAPIImpl{apiConfig}, nil
}

func (impl vaultAPIImpl) Connect(url string) (*api.Client, error) {
	impl.config.Address = url
	return api.NewClient(impl.config)
}

func (impl vaultAPIImpl) TakeSnapshot(ctx context.Context, client *api.Client, writer io.Writer) error {
	return client.Sys().RaftSnapshotWithContext(ctx, writer)
}

func (impl vaultAPIImpl) GetLeader(ctx context.Context, client *api.Client) (bool, string) {
	leader, err := client.Sys().LeaderWithContext(ctx)
	if err != nil {
		logging.Warn("could not determine leader-state of node", "node", client.Address())
		return false, ""
	}

	return leader.IsSelf, leader.LeaderAddress
}

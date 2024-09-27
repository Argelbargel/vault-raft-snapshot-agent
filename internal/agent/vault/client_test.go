package vault

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"testing"
	"time"

	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/config/secret"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/vault/auth"

	"github.com/hashicorp/vault/api"

	"github.com/stretchr/testify/assert"
)

func TestClientConnectsToLeaderIfNotConnected(t *testing.T) {
	node1 := "http://node1"
	node2 := "http://node2"
	node3 := "http://node3"
	node4 := "http://node4"

	auth := &authMethodStub{}
	apiStub := &vaultAPIStub{
		Nodes: map[string]bool{
			node1: false,
			node2: false,
			node3: true,
			node4: false,
		},
	}

	client := NewClient(apiStub, []string{node1, node2, node3, node4}, false, auth)

	err := client.TakeSnapshot(context.Background(), bufio.NewWriter(&bytes.Buffer{}))

	assert.NoError(t, err, "TakeSnapshot() failed unexpectedly")
	assert.Equal(t, node3, client.connection.Address())
	assert.Equal(t, []string{node1, node2, node3}, apiStub.Connections)
	assert.Equal(t, []string{node1, node2, node3}, auth.Connections)
}

func TestClientConnectsToAutoDetectedLeaderIfNotConnected(t *testing.T) {
	node1 := "http://node1"
	node2 := "http://node2"
	node3 := "http://node3"

	auth := &authMethodStub{}
	apiStub := &vaultAPIStub{
		Nodes: map[string]bool{
			node1: false,
			node2: false,
			node3: true,
		},
	}

	client := NewClient(apiStub, []string{node1, node2, node3}, true, auth)

	err := client.TakeSnapshot(context.Background(), bufio.NewWriter(&bytes.Buffer{}))

	assert.NoError(t, err, "TakeSnapshot() failed unexpectedly")
	assert.Equal(t, node3, client.connection.Address())
	assert.Equal(t, []string{node1, node3}, apiStub.Connections)
	assert.Equal(t, []string{node1, node3}, auth.Connections)
}

func TestClientSwitchesToLeader(t *testing.T) {
	node1 := "http://node1"
	node2 := "http://node2"
	node3 := "http://node3"

	auth := &authMethodStub{}
	apiStub := &vaultAPIStub{
		Nodes: map[string]bool{
			node1: false,
			node2: false,
			node3: true,
		},
	}

	client := NewClient(apiStub, []string{node1, node2, node3}, false, auth)
	connectionConfig := api.DefaultConfig()
	connectionConfig.Address = node1
	client.connection, _ = api.NewClient(connectionConfig)

	err := client.TakeSnapshot(context.Background(), bufio.NewWriter(&bytes.Buffer{}))

	assert.NoError(t, err, "TakeSnapshot() failed unexpectedly")
	assert.Equal(t, node3, client.connection.Address())
	assert.Equal(t, []string{node2, node3}, apiStub.Connections)
	assert.Equal(t, []string{node1, node2, node3}, auth.Connections)
}

func TestClientSwitchesToAutoDetectedLeaderOnLeaderChange(t *testing.T) {
	node1 := "http://node1"
	node2 := "http://node2"
	node3 := "http://node3"

	auth := &authMethodStub{}
	apiStub := &vaultAPIStub{
		Nodes: map[string]bool{
			node1: false,
			node2: false,
			node3: true,
		},
	}

	client := NewClient(apiStub, []string{node1, node2, node3}, true, auth)
	connectionConfig := api.DefaultConfig()
	connectionConfig.Address = node1
	client.connection, _ = api.NewClient(connectionConfig)

	err := client.TakeSnapshot(context.Background(), bufio.NewWriter(&bytes.Buffer{}))

	assert.NoError(t, err, "TakeSnapshot() failed unexpectedly")
	assert.Equal(t, node3, client.connection.Address())
	assert.Equal(t, []string{node3}, apiStub.Connections)
	assert.Equal(t, []string{node1, node3}, auth.Connections)
}

func TestClientSwitchesToLeaderOnAuthFailure(t *testing.T) {
	node1 := "http://node1"
	node2 := "http://node2"
	node3 := "http://node3"

	auth := &authMethodStub{
		FailingNodes: []string{node1},
	}
	apiStub := &vaultAPIStub{
		Nodes: map[string]bool{
			node1: true,
			node2: false,
			node3: true,
		},
	}

	client := NewClient(apiStub, []string{node1, node2, node3}, false, auth)
	connectionConfig := api.DefaultConfig()
	connectionConfig.Address = node1
	client.connection, _ = api.NewClient(connectionConfig)

	err := client.TakeSnapshot(context.Background(), bufio.NewWriter(&bytes.Buffer{}))

	assert.NoError(t, err, "TakeSnapshot() failed unexpectedly")
	assert.Equal(t, node3, client.connection.Address())
	assert.Equal(t, []string{node2, node3}, apiStub.Connections)
	assert.Equal(t, []string{node1, node2, node3}, auth.Connections)
}

func TestClientIgnoresConnectionFailuresToNonLeader(t *testing.T) {
	node1 := "http://node1"
	node2 := "http://node2"
	node3 := "http://node3"

	auth := &authMethodStub{}
	apiStub := &vaultAPIStub{
		Nodes: map[string]bool{
			node1: false,
			node2: false,
			node3: true,
		},
		FailingNodes: []string{node1, node2},
	}

	client := NewClient(apiStub, []string{node1, node2, node3}, false, auth)

	err := client.TakeSnapshot(context.Background(), bufio.NewWriter(&bytes.Buffer{}))

	assert.NoError(t, err, "TakeSnapshot() failed unexpectedly")
	assert.Equal(t, node3, client.connection.Address())
	assert.Equal(t, []string{node1, node2, node3}, apiStub.Connections)
	assert.Equal(t, []string{node3}, auth.Connections)
}

func TestClientIgnoresAuthenticationFailuresToNonLeader(t *testing.T) {
	node1 := "http://node1"
	node2 := "http://node2"
	node3 := "http://node3"

	auth := &authMethodStub{
		FailingNodes: []string{node1, node2},
	}
	apiStub := &vaultAPIStub{
		Nodes: map[string]bool{
			node1: false,
			node2: false,
			node3: true,
		},
	}

	client := NewClient(apiStub, []string{node1, node2, node3}, false, auth)

	err := client.TakeSnapshot(context.Background(), bufio.NewWriter(&bytes.Buffer{}))

	assert.NoError(t, err, "TakeSnapshot() failed unexpectedly")
	assert.Equal(t, node3, client.connection.Address())
	assert.Equal(t, []string{node1, node2, node3}, apiStub.Connections)
	assert.Equal(t, []string{node1, node2, node3}, auth.Connections)
}

func TestClientFailsWhenNoLeaderCanBeFound(t *testing.T) {
	node1 := "http://node1"
	node2 := "http://node2"
	node3 := "http://node3"

	auth := &authMethodStub{}
	apiStub := &vaultAPIStub{
		Nodes: map[string]bool{
			node1: false,
			node2: false,
			node3: false,
		},
	}

	client := NewClient(apiStub, []string{node1, node2, node3}, false, auth)

	err := client.TakeSnapshot(context.Background(), bufio.NewWriter(&bytes.Buffer{}))

	assert.Error(t, err, "TakeSnapshot() should fail if no connection to leader is possible")
	assert.Nil(t, client.connection)
	assert.Equal(t, []string{node1, node2, node3}, apiStub.Connections)
	assert.Equal(t, []string{node1, node2, node3}, auth.Connections)
}

func TestClientFailsWhenNoLeaderCanBeConnected(t *testing.T) {
	node1 := "http://node1"
	node2 := "http://node2"
	node3 := "http://node3"

	auth := &authMethodStub{}
	apiStub := &vaultAPIStub{
		Nodes: map[string]bool{
			node1: false,
			node2: false,
			node3: true,
		},
		FailingNodes: []string{node3},
	}

	client := NewClient(apiStub, []string{node1, node2, node3}, false, auth)

	err := client.TakeSnapshot(context.Background(), bufio.NewWriter(&bytes.Buffer{}))

	assert.Error(t, err, "TakeSnapshot() should fail if no connection to leader is possible")
	assert.Nil(t, client.connection)
	assert.Equal(t, []string{node1, node2, node3}, apiStub.Connections)
	assert.Equal(t, []string{node1, node2}, auth.Connections)
}

func TestClientFailsWhenNoLeaderCanBeAuthenticated(t *testing.T) {
	node1 := "http://node1"
	node2 := "http://node2"
	node3 := "http://node3"

	auth := &authMethodStub{
		FailingNodes: []string{node3},
	}
	apiStub := &vaultAPIStub{
		Nodes: map[string]bool{
			node1: false,
			node2: false,
			node3: true,
		},
	}

	client := NewClient(apiStub, []string{node1, node2, node3}, false, auth)

	err := client.TakeSnapshot(context.Background(), bufio.NewWriter(&bytes.Buffer{}))

	assert.Error(t, err, "TakeSnapshot() should fail if no connection to leader is possible")
	assert.Nil(t, client.connection)
	assert.Equal(t, []string{node1, node2, node3}, apiStub.Connections)
	assert.Equal(t, []string{node1, node2, node3}, auth.Connections)
}

func TestClientKeepsConnectionToLeader(t *testing.T) {
	node1 := "http://node1"
	node2 := "http://node2"

	auth := &authMethodStub{}
	apiStub := &vaultAPIStub{
		Nodes: map[string]bool{
			node1: true,
			node2: true,
		},
	}

	client := NewClient(apiStub, []string{node1, node2}, false, auth)
	connectionConfig := api.DefaultConfig()
	connectionConfig.Address = node1
	client.connection, _ = api.NewClient(connectionConfig)

	err := client.TakeSnapshot(context.Background(), bufio.NewWriter(&bytes.Buffer{}))

	assert.NoError(t, err, "TakeSnapshot() failed unexpectedly")
	assert.Equal(t, node1, client.connection.Address())
	assert.Nil(t, apiStub.Connections)
	assert.Equal(t, []string{node1}, auth.Connections)
}

func TestClientTakesSnapshot(t *testing.T) {
	node1 := "http://node1"

	auth := &authMethodStub{}
	apiStub := &vaultAPIStub{
		Nodes: map[string]bool{
			node1: true,
		},
	}

	client := NewClient(apiStub, []string{node1}, false, auth)
	connectionConfig := api.DefaultConfig()
	connectionConfig.Address = node1
	client.connection, _ = api.NewClient(connectionConfig)

	context := context.Background()
	writer := bufio.NewWriter(&bytes.Buffer{})
	err := client.TakeSnapshot(context, writer)

	assert.NoError(t, err, "TakeSnapshot() failed unexpectedly")
	assert.True(t, apiStub.snapshotTaken)
	assert.Equal(t, context, apiStub.snapshotContext)
	assert.Same(t, client.connection, apiStub.snapshotConnection)
	assert.Same(t, writer, apiStub.snapshotWriter)
}

func TestCreateClient(t *testing.T) {
	node1 := "http://node1"
	node2 := "http://node2"
	node3 := "http://node3"

	config := VaultClientConfig{
		Url: node1,
		Nodes: VaultNodesConfig{
			Urls:             []string{node2, node3},
			AutoDetectLeader: true,
		},
		Auth: auth.VaultAuthConfig{
			UserPass: &auth.UserPassAuthConfig{
				Username: secret.FromString("test"),
				Password: secret.FromString("test"),
			},
		},
		Insecure: true,
		Timeout:  time.Duration(60) * time.Second,
	}

	client, _ := CreateClient(config)
	assert.Equal(t, []string{node1, node2, node3}, client.nodes)
	assert.True(t, client.autoDetectLeader)
	assert.NotEmpty(t, client.auth)
	assert.True(t, client.api.(vaultAPIImpl).config.TLSConfig().InsecureSkipVerify)
	assert.Equal(t, config.Timeout, client.api.(vaultAPIImpl).config.Timeout)
}

type vaultAPIStub struct {
	Nodes              map[string]bool
	FailingNodes       []string
	Connections        []string
	snapshotTaken      bool
	snapshotContext    context.Context
	snapshotConnection *api.Client
	snapshotWriter     io.Writer
}

func (stub *vaultAPIStub) Connect(node string) (*api.Client, error) {
	stub.Connections = append(stub.Connections, node)

	if slices.Contains(stub.FailingNodes, node) {
		return nil, fmt.Errorf("could not connect to %s", node)
	}

	config := api.DefaultConfig()
	config.Address = node

	return api.NewClient(config)
}

func (stub *vaultAPIStub) GetLeader(_ context.Context, client *api.Client) (bool, string) {
	if stub.Nodes[client.Address()] {
		return true, client.Address()
	}

	for node, leader := range stub.Nodes {
		if leader {
			return false, node
		}
	}

	return false, ""
}

func (stub *vaultAPIStub) Address() string {
	return "test"
}

func (stub *vaultAPIStub) TakeSnapshot(ctx context.Context, conn *api.Client, writer io.Writer) error {
	stub.snapshotTaken = true
	stub.snapshotContext = ctx
	stub.snapshotConnection = conn
	stub.snapshotWriter = writer
	return nil
}

type authMethodStub struct {
	Connections  []string
	FailingNodes []string
}

func (a *authMethodStub) Refresh(_ context.Context, client *api.Client, _ bool) error {
	a.Connections = append(a.Connections, client.Address())
	if slices.Contains(a.FailingNodes, client.Address()) {
		return errors.New("refresh of auth failed")
	}
	return nil
}

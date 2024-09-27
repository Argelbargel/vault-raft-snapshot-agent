package vault

import (
	"time"

	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/vault/auth"
)

type VaultClientConfig struct {
	Url      string           `default:"http://127.0.0.1:8200" validate:"required_without=Nodes,http_url"`
	Nodes    VaultNodesConfig `validate:"required_without=Url"`
	Timeout  time.Duration    `default:"60s"`
	Insecure bool
	Auth     auth.VaultAuthConfig
}

type VaultNodesConfig struct {
	Urls             []string `validate:"dive,http_url"`
	AutoDetectLeader bool
}

package vault

import (
	"time"

	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/vault/auth"
)

type VaultClientConfig struct {
	Nodes    VaultNodesConfig `validate:"required"`
	Timeout  time.Duration    `default:"60s"`
	Insecure bool
	Auth     auth.VaultAuthConfig
}

type VaultNodesConfig struct {
	Urls             []string `validate:"dive,required,http_url"`
	AutoDetectLeader bool
}

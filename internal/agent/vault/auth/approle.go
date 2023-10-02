package auth

import (
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/config/secret"
	"github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/api/auth/approle"
)

type AppRoleAuthConfig struct {
	Path     string        `default:"approle"`
	RoleId   secret.Secret `mapstructure:"role" validate:"required_if=Empty false"`
	SecretId secret.Secret `mapstructure:"secret" validate:"required_if=Empty false"`
	Empty    bool
}

func (c AppRoleAuthConfig) createAuthMethod() (api.AuthMethod, error) {
	roleId, err := c.RoleId.Resolve(true)
	if err != nil {
		return nil, err
	}

	secretId, err := c.SecretId.Resolve(true)
	if err != nil {
		return nil, err
	}

	return approle.NewAppRoleAuth(
		roleId,
		&approle.SecretID{FromString: secretId},
		approle.WithMountPath(c.Path),
	)
}

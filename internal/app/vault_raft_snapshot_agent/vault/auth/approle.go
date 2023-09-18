package auth

import (
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/app/vault_raft_snapshot_agent/secret"
	"github.com/hashicorp/vault/api/auth/approle"
)

type AppRoleAuthConfig struct {
	Path     string        `default:"approle"`
	RoleId   secret.Secret `mapstructure:"role" validate:"required_if=Empty false"`
	SecretId secret.Secret `mapstructure:"secret" validate:"required_if=Empty false"`
	Empty    bool
}

func createAppRoleAuth(config AppRoleAuthConfig) vaultAuthMethod[AppRoleAuthConfig, *approle.AppRoleAuth] {
	return vaultAuthMethod[AppRoleAuthConfig, *approle.AppRoleAuth]{
		config,
		func(config AppRoleAuthConfig) (*approle.AppRoleAuth, error) {
			roleId, err := config.RoleId.Resolve(true)
			if err != nil {
				return nil, err
			}

			secretId, err := config.SecretId.Resolve(true)
			if err != nil {
				return nil, err
			}

			return approle.NewAppRoleAuth(
				roleId,
				&approle.SecretID{FromString: secretId},
				approle.WithMountPath(config.Path),
			)
		},
	}
}

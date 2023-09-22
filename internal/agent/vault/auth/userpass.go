package auth

import (
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/config/secret"
	"github.com/hashicorp/vault/api/auth/userpass"
)

type UserPassAuthConfig struct {
	Path     string        `default:"userpass"`
	Username secret.Secret `validate:"required_if=Empty false"`
	Password secret.Secret `validate:"required_if=Empty false"`
	Empty    bool
}

func createUserPassAuth(config UserPassAuthConfig) vaultAuthMethod[UserPassAuthConfig, *userpass.UserpassAuth] {
	return vaultAuthMethod[UserPassAuthConfig, *userpass.UserpassAuth]{
		config,
		func(config UserPassAuthConfig) (*userpass.UserpassAuth, error) {
			username, err := config.Username.Resolve(true)
			if err != nil {
				return nil, err
			}
			password, err := config.Password.Resolve(true)
			if err != nil {
				return nil, err
			}

			return userpass.NewUserpassAuth(
				username,
				&userpass.Password{FromString: password},
				userpass.WithMountPath(config.Path),
			)
		},
	}
}

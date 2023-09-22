package auth

import (
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/config/secret"
	"github.com/hashicorp/vault/api/auth/ldap"
)

type LDAPAuthConfig struct {
	Path     string        `default:"ldap"`
	Username secret.Secret `validate:"required_if=Empty false"`
	Password secret.Secret `validate:"required_if=Empty false"`
	Empty    bool
}

func createLDAPAuth(config LDAPAuthConfig) vaultAuthMethod[LDAPAuthConfig, *ldap.LDAPAuth] {
	return vaultAuthMethod[LDAPAuthConfig, *ldap.LDAPAuth]{
		config,
		func(config LDAPAuthConfig) (*ldap.LDAPAuth, error) {
			username, err := config.Username.Resolve(true)
			if err != nil {
				return nil, err
			}
			password, err := config.Password.Resolve(true)
			if err != nil {
				return nil, err
			}

			return ldap.NewLDAPAuth(
				username,
				&ldap.Password{FromString: password},
				ldap.WithMountPath(config.Path),
			)
		},
	}
}

package auth

import (
	"errors"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/config/secret"
	"github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCreateAuthMethod(t *testing.T) {
	expectedToken := "test"

	auth, _ := Token(secret.FromString(expectedToken)).createAuthMethod()
	assert.Equal(t, tokenAuth{expectedToken}, auth)
}

func TestTokenAuthFailsIfLoginFails(t *testing.T) {
	authApiStub := tokenVaultAuthApiStub{loginFails: true}
	auth := tokenAuth{"test"}

	_, err := auth.login(&authApiStub)

	assert.Error(t, err, "token-VaultAuth did not report error although login failed!")
}

func TestTokenAuthReturnsExpirationBasedOnLoginLeaseDuration(t *testing.T) {
	authApiStub := tokenVaultAuthApiStub{leaseDuration: 60}

	auth := tokenAuth{"test"}

	authSecret, err := auth.login(&authApiStub)

	assert.NoErrorf(t, err, "token-VaultAuth failed unexpectedly")

	expectedSecret := &api.Secret{
		Auth: &api.SecretAuth{
			LeaseDuration: authApiStub.leaseDuration,
		},
	}
	assert.Equal(t, expectedSecret, authSecret)
}

type tokenVaultAuthApiStub struct {
	token         string
	loginFails    bool
	leaseDuration int
}

func (stub *tokenVaultAuthApiStub) SetToken(token string) {
	stub.token = token
}

func (stub *tokenVaultAuthApiStub) ClearToken() {
	stub.token = ""
}

func (stub *tokenVaultAuthApiStub) LookupToken() (*api.Secret, error) {
	if stub.loginFails {
		return &api.Secret{}, errors.New("lookup failed")
	}

	return &api.Secret{
		Auth: &api.SecretAuth{
			LeaseDuration: stub.leaseDuration,
		},
	}, nil
}

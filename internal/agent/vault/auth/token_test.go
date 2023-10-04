package auth

import (
	"context"
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
	client := &api.Client{}
	lookup := tokenLookupStub{lookupFails: true}
	auth := tokenAuth{"test"}

	_, err := auth.login(context.Background(), client, &lookup)

	assert.Error(t, err, "token-VaultAuth did not report error although login failed!")
	assert.Equal(t, auth.token, lookup.token)
	assert.Zero(t, client.Token())
}

func TestTokenAuthReturnsExpirationBasedOnLoginLeaseDuration(t *testing.T) {
	client := &api.Client{}
	lookup := tokenLookupStub{leaseDuration: 60}

	auth := tokenAuth{"test"}

	authSecret, err := auth.login(context.Background(), client, &lookup)

	assert.NoErrorf(t, err, "token-VaultAuth failed unexpectedly")

	expectedSecret := &api.Secret{
		Auth: &api.SecretAuth{
			LeaseDuration: lookup.leaseDuration,
		},
	}

	assert.Equal(t, expectedSecret, authSecret)
	assert.Equal(t, auth.token, client.Token())
}

type tokenLookupStub struct {
	lookupFails   bool
	leaseDuration int
	token         string
}

func (stub *tokenLookupStub) Lookup(_ context.Context, client *api.Client) (*api.Secret, error) {
	stub.token = client.Token()

	if stub.lookupFails {
		return nil, errors.New("lookup failed")
	}

	return &api.Secret{
		Auth: &api.SecretAuth{
			LeaseDuration: stub.leaseDuration,
		},
	}, nil
}

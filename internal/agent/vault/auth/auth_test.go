package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/assert"
)

func TestVaultAuth_Refresh_FailsIfMethodFactoryFails(t *testing.T) {
	expectedErr := errors.New("create failed")
	auth := vaultAuthImpl{
		factory: authMethodFactoryStub{createErr: expectedErr},
	}

	err := auth.Refresh(context.Background(), nil, true)
	assert.ErrorIs(t, err, expectedErr)
	assert.Equal(t, time.Time{}, auth.expires)
}

func TestVaultAuth_Refresh_FailsIfAuthMethodRefreshFails(t *testing.T) {
	expectedErr := errors.New("login failed")
	auth := vaultAuthImpl{
		factory: authMethodFactoryStub{
			method: authMethodStub{loginError: expectedErr},
		},
	}

	err := auth.Refresh(context.Background(), nil, true)
	assert.ErrorIs(t, err, expectedErr)
	assert.Equal(t, time.Time{}, auth.expires)
}

func TestVaultAuth_Refresh_SkipsLoginUntilExpired(t *testing.T) {
	expires := time.Now().Add(time.Duration(1) * time.Second)
	expectedErr := errors.New("login failed")
	auth := vaultAuthImpl{
		factory: authMethodFactoryStub{
			method: authMethodStub{loginError: expectedErr},
		},
		expires: expires,
	}

	err := auth.Refresh(context.Background(), nil, false)
	assert.NoError(t, err, "Refresh failed unexpectedly")
	assert.Equal(t, expires, auth.expires)

	time.Sleep(time.Second * 2)

	err = auth.Refresh(context.Background(), nil, false)
	assert.ErrorIs(t, err, expectedErr)
}

func TestVaultAuth_Expires_AfterTokenTTL(t *testing.T) {
	expectedTTL := 60
	expectedExpires := time.Now().Add((time.Duration(expectedTTL) * time.Second) / 2)
	expectedSecret := &api.Secret{
		Auth: &api.SecretAuth{
			ClientToken:   "test",
			LeaseDuration: expectedTTL,
		},
	}

	auth := vaultAuthImpl{
		factory: authMethodFactoryStub{
			method: authMethodStub{secret: expectedSecret},
		},
	}

	err := auth.Refresh(context.Background(), &api.Client{}, false)

	assert.NoError(t, err, "Refresh failed unexpectedly")
	assert.Equal(t, expectedExpires, auth.expires)
}

func TestVaultAuth_Refresh_IgnoresExpiresIfForced(t *testing.T) {
	expires := time.Now().Add(time.Duration(60) * time.Second)
	expectedTTL := 30
	expectedExpires := time.Now().Add((time.Duration(expectedTTL) * time.Second) / 2)
	expectedSecret := &api.Secret{
		Auth: &api.SecretAuth{
			ClientToken:   "test",
			LeaseDuration: expectedTTL,
		},
	}

	auth := vaultAuthImpl{
		factory: authMethodFactoryStub{
			method: authMethodStub{secret: expectedSecret},
		},
		expires: expires,
	}

	err := auth.Refresh(context.Background(), &api.Client{}, true)
	assert.NoError(t, err, "Refresh failed unexpectedly")
	assert.Equal(t, expectedExpires, auth.expires)
}

type authMethodFactoryStub struct {
	method    api.AuthMethod
	createErr error
}

func (stub authMethodFactoryStub) createAuthMethod() (api.AuthMethod, error) {
	if stub.createErr != nil {
		return nil, stub.createErr
	}
	return stub.method, nil
}

type authMethodStub struct {
	loginError error
	secret     *api.Secret
}

func (stub authMethodStub) Login(_ context.Context, _ *api.Client) (*api.Secret, error) {
	if stub.loginError != nil {
		return nil, stub.loginError
	}

	return stub.secret, nil
}

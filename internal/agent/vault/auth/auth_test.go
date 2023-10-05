package auth

import (
	"context"
	"errors"
	"github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestVaultAuthMethod_Login_FailsIfMethodFactoryFails(t *testing.T) {
	expectedErr := errors.New("create failed")
	auth := vaultAuthMethodImpl{
		authMethodFactoryStub{createErr: expectedErr},
	}

	_, err := auth.Login(context.Background(), nil)
	assert.ErrorIs(t, err, expectedErr)
}

func TestVaultAuthMethod_Login_FailsIfAuthMethodLoginFails(t *testing.T) {
	expectedErr := errors.New("login failed")
	auth := vaultAuthMethodImpl{
		authMethodFactoryStub{
			method: authMethodStub{loginError: expectedErr},
		},
	}

	_, err := auth.Login(context.Background(), nil)
	assert.ErrorIs(t, err, expectedErr)
}

func TestVaultAuthMethod_Login_ReturnsLeaseDuration(t *testing.T) {
	expectedSecret := &api.Secret{
		Auth: &api.SecretAuth{
			ClientToken: "test",
		},
	}

	auth := vaultAuthMethodImpl{
		authMethodFactoryStub{
			method: authMethodStub{secret: expectedSecret},
		},
	}

	authSecret, err := auth.Login(context.Background(), &api.Client{})

	assert.NoError(t, err, "Login failed unexpectedly")
	assert.Equal(t, expectedSecret, authSecret)
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

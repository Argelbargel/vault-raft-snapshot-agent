package auth

import (
	"context"
	"errors"
	"github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestVaultAuthMethod_Login_FailsIfMethodFactoryFails(t *testing.T) {
	expectedErr := errors.New("create failed")
	auth := vaultAuthMethod{
		authMethodFactoryStub{createErr: expectedErr},
	}

	_, err := auth.Login(context.Background(), nil)
	assert.ErrorIs(t, err, expectedErr)
}

func TestVaultAuthMethod_Login_FailsIfAuthMethodLoginFails(t *testing.T) {
	expectedErr := errors.New("login failed")
	auth := vaultAuthMethod{
		authMethodFactoryStub{
			method: authMethodStub{loginError: expectedErr},
		},
	}

	_, err := auth.Login(context.Background(), nil)
	assert.ErrorIs(t, err, expectedErr)
}

func TestVaultAuthMethod_Login_ReturnsLeaseDuration(t *testing.T) {
	expectedLeaseDuration := 60
	auth := vaultAuthMethod{
		authMethodFactoryStub{
			method: authMethodStub{leaseDuration: expectedLeaseDuration},
		},
	}

	leaseDuration, err := auth.Login(context.Background(), &api.Client{})

	assert.NoError(t, err, "Login failed unexpectedly")
	assert.Equal(t, time.Duration(expectedLeaseDuration)*time.Second, leaseDuration)
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
	loginError    error
	leaseDuration int
}

func (stub authMethodStub) Login(_ context.Context, _ *api.Client) (*api.Secret, error) {
	if stub.loginError != nil {
		return nil, stub.loginError
	}

	return &api.Secret{
		Auth: &api.SecretAuth{
			ClientToken:   "Test",
			LeaseDuration: stub.leaseDuration,
		},
	}, nil
}

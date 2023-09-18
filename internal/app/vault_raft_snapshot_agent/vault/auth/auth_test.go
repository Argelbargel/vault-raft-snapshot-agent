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
	expectedErr := errors.New("methodFactory failed")
	auth := vaultAuthMethod[any, api.AuthMethod]{
		methodFactory: func(_ any) (api.AuthMethod, error) {
			return nil, expectedErr
		},
	}

	_, err := auth.Login(context.Background(), nil)
	assert.ErrorIs(t, err, expectedErr)
}

func TestVaultAuthMethod_Login_FailsIfAuthMethodLoginFails(t *testing.T) {
	expectedErr := errors.New("login failed")
	auth := vaultAuthMethod[any, api.AuthMethod]{
		methodFactory: func(_ any) (api.AuthMethod, error) {
			return authMethodStub{loginError: expectedErr}, nil
		},
	}

	_, err := auth.Login(context.Background(), nil)
	assert.ErrorIs(t, err, expectedErr)
}

func TestVaultAuthMethod_Login_ReturnsLeaseDuration(t *testing.T) {
	expectedLeaseDuration := 60
	auth := vaultAuthMethod[any, api.AuthMethod]{
		methodFactory: func(_ any) (api.AuthMethod, error) {
			return authMethodStub{leaseDuration: expectedLeaseDuration}, nil
		},
	}

	leaseDuration, err := auth.Login(context.Background(), nil)

	assert.NoError(t, err, "Login failed unexpectedly")
	assert.Equal(t, time.Duration(expectedLeaseDuration), leaseDuration)
}

type authMethodStub struct {
	loginError    error
	leaseDuration int
}

func (stub authMethodStub) Login(_ context.Context, _ *api.Client) (*api.Secret, error) {
	if stub.loginError != nil {
		return nil, stub.loginError
	}

	return &api.Secret{LeaseDuration: stub.leaseDuration}, nil
}

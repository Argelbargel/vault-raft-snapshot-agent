package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAuthBackendFailsIfAuthCredentialsFactoryFails(t *testing.T) {
	authApiStub := backendVaultAuthApiStub{}

	auth := authBackend{
		credentialsFactory: func() (map[string]interface{}, error) {
			return map[string]interface{}{}, errors.New("could not create credentials")
		},
	}

	_, err := auth.Refresh(&authApiStub)
	
	assert.Error(t, err, "auth backend did not fail when credentials-factory failed")
	assert.False(t, authApiStub.triedToLogin, "auth backend did try to login although credentials-factory failed")
}

func TestAuthBackendFailsIfLoginFails(t *testing.T) {
	authApiStub := backendVaultAuthApiStub{loginFails: true}
	auth := authBackend{
		credentialsFactory: func() (map[string]interface{}, error) {
			return map[string]interface{}{}, nil
		},
	}

	_, err := auth.Refresh(&authApiStub)

	assert.Error(t, err, "auth backend did not fail when login failed")
	assert.True(t, authApiStub.triedToLogin, "auth backend did not try to login")
}

func TestAuthBackendPassesPathAndLoginCredentials(t *testing.T) {
	authApiStub := backendVaultAuthApiStub{}
	authPath := "test"
	expectedAuthPath := "auth/" + authPath + "/login"
	expectedCredentials := map[string]interface{}{
		"key": "value",
	}

	auth := authBackend{
		path: authPath,
		credentialsFactory: func() (map[string]interface{}, error) {
			return expectedCredentials, nil
		},
	}

	_, err := auth.Refresh(&authApiStub)

	assert.NoError(t, err, "auth backend failed unexpectedly")
	assert.Equal(t, expectedAuthPath, authApiStub.loginPath)
	assert.Equalf(t, expectedCredentials, authApiStub.loginCredentials, "auth backend did not pass expected credentials")
}

func TestBackendAuthReturnsExpirationBasedOnLoginLeaseDuration(t *testing.T) {
	authApiStub := backendVaultAuthApiStub{leaseDuration: time.Minute}

	auth := authBackend{
		credentialsFactory: func() (map[string]interface{}, error) {
			return map[string]interface{}{}, nil
		},
	}

	expiration, err := auth.Refresh(&authApiStub)
	assert.NoError(t, err, "auth backend failed unexpectedly")

	expectedExpiration := time.Now().Add((time.Second * authApiStub.leaseDuration) / 2)
	assert.Equal(t, expectedExpiration, expiration)
}

type backendVaultAuthApiStub struct {
	loginFails       bool
	triedToLogin     bool
	loginPath        string
	loginCredentials map[string]interface{}
	leaseDuration    time.Duration
}

func (stub *backendVaultAuthApiStub) LoginToBackend(path string, credentials map[string]interface{}) (leaseDuration time.Duration, err error) {
	stub.triedToLogin = true
	stub.loginPath = path
	stub.loginCredentials = credentials
	if stub.loginFails {
		return 0, errors.New("login failed")
	} else {
		return stub.leaseDuration, nil
	}
}

func (stub *backendVaultAuthApiStub) LoginWithToken(token string) (leaseDuration time.Duration, err error) {
	return 0, errors.New("not implemented")
}

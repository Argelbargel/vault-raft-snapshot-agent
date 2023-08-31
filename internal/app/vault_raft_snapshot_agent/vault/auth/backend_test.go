package auth

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestAuthBackendFailsIfAuthCredentialsFactoryFails(t *testing.T) {
	authApiStub := backendVaultAuthApiStub{}

	auth := authBackend{
		credentialsFactory: func() (map[string]interface{}, error) {
			return map[string]interface{}{}, errors.New("could not create credentials")
		},
	}
	_, err := auth.Refresh(&authApiStub)
	if err == nil {
		t.Fatalf("auth backend did not fail when credentials-factory failed")
	}

	if authApiStub.triedToLogin {
		t.Fatalf("auth backend did try to login although credentials-factory failed")
	}
}

func TestAuthBackendFailsIfLoginFails(t *testing.T) {
	authApiStub := backendVaultAuthApiStub{loginFails: true}
	auth := authBackend{
		credentialsFactory: func() (map[string]interface{}, error) {
			return map[string]interface{}{}, nil
		},
	}

	_, err := auth.Refresh(&authApiStub)
	if err == nil {
		t.Fatalf("auth backend did not fail when login failed")
	}

	if !authApiStub.triedToLogin {
		t.Fatalf("auth backend did not try to login")
	}
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
	if err != nil {
		t.Fatalf("auth backend failed unexpectedly: %v", err)
	}

	if authApiStub.loginPath != expectedAuthPath {
		t.Fatalf("auth backend did not pass expected auth-path - expected %s, got %s", expectedAuthPath, authApiStub.loginPath)
	}

	if !reflect.DeepEqual(authApiStub.loginCredentials, expectedCredentials) {
		t.Fatalf("auth backend did not pass expected credentials - expected %v, got %v", expectedCredentials, authApiStub.loginCredentials)
	}
}

func TestBackendAuthReturnsExpirationBasedOnLoginLeaseDuration(t *testing.T) {
	authApiStub := backendVaultAuthApiStub{leaseDuration: time.Minute}

	auth := authBackend{
		credentialsFactory: func() (map[string]interface{}, error) {
			return map[string]interface{}{}, nil
		},
	}

	expiration, err := auth.Refresh(&authApiStub)
	if err != nil {
		t.Fatalf("auth backend failed unexpectedly: %v", err)
	}

	expectedExpiration := time.Now().Add((time.Second*authApiStub.leaseDuration)/2)
	if expiration != expectedExpiration {
		t.Fatalf("auth backend returned unexpected expiration - expected %v got %v", expectedExpiration, expiration)
	}
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

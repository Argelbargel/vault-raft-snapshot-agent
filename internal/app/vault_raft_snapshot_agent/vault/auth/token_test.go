package auth

import (
	"errors"
	"testing"
	"time"
)

func TestCreateTokenAuth(t *testing.T) {
	expectedToken := "test"

	authApiStub := tokenVaultAuthApiStub{}

	auth := createTokenAuth(expectedToken)
	_, err := auth.Refresh(&authApiStub)
	if err != nil {
		t.Fatalf("token-auth failed unexpectedly: %v", err)
	}

	if authApiStub.token != expectedToken {
		t.Fatalf("token-auth did not pass expected token - expected %s, got %s", expectedToken, authApiStub.token)
	}
}

func TestTokenAuthFailsIfLoginFails(t *testing.T) {
	authApiStub := tokenVaultAuthApiStub{loginFails: true}

	auth := createTokenAuth("test")
	_, err := auth.Refresh(&authApiStub)
	if err == nil {
		t.Fatalf("token-auth did not report error although login failed!")
	}
}

func TestTokenAuthReturnsExpirationBasedOnLoginLeaseDuration(t *testing.T) {
	authApiStub := tokenVaultAuthApiStub{leaseDuration: time.Minute}

	auth := createTokenAuth("test")

	expiration, err := auth.Refresh(&authApiStub)
	if err != nil {
		t.Fatalf("token-auth failed unexpectedly: %v", err)
	}

	expectedExpiration := time.Now().Add((time.Second * authApiStub.leaseDuration) / 2)
	if expiration != expectedExpiration {
		t.Fatalf("token-auth returned unexpected expiration - expected %v got %v", expectedExpiration, expiration)
	}
}

type tokenVaultAuthApiStub struct {
	token         string
	loginFails    bool
	leaseDuration time.Duration
}

func (stub *tokenVaultAuthApiStub) LoginToBackend(path string, credentials map[string]interface{}) (leaseDuration time.Duration, err error) {
	return 0, errors.New("not implemented")
}

func (stub *tokenVaultAuthApiStub) LoginWithToken(token string) (leaseDuration time.Duration, err error) {
	stub.token = token
	if stub.loginFails {
		return 0, errors.New("login failed")
	}
	return stub.leaseDuration, nil
}

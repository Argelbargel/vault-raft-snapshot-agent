package auth

import (
	"errors"
	"testing"
	"time"
)

func TestCreateDefaultAppRoleAuth(t *testing.T) {
	authPath := "test"
	expectedLoginPath := "auth/" + authPath + "/login"
	expectedRoleId := "testRoleId"
	expectedSecretId := "testSecretId"

	config := AppRoleAuthConfig{
		Path:     authPath,
		RoleId:   expectedRoleId,
		SecretId: expectedSecretId,
	}

	authApiStub := appRoleVaultAuthApiStub{}

	auth := createAppRoleAuth(config)
	auth.Refresh(&authApiStub)


	assertAppRoleAuthValues(t, expectedLoginPath, expectedRoleId, expectedSecretId, auth, authApiStub)
}

func assertAppRoleAuthValues(t *testing.T, expectedLoginPath string, expectedRoleId string, expectedSecretId string, auth authBackend, api appRoleVaultAuthApiStub) {
	if auth.name != "AppRole" {
		t.Fatalf("AppRoleAuth has wrong name - expected AppRole, got %v", auth.name)
	}

	if api.path != expectedLoginPath {
		t.Fatalf("default path of AppRoleAuth is not approle - got: %v", api.path)
	}

	if api.roleId != expectedRoleId {
		t.Fatalf("auth did not pass correct role-id - expected %v, got %v", expectedRoleId, api.roleId)
	}

	if api.secretId != expectedSecretId {
		t.Fatalf("auth did not pass correct role-id - expected %v, got %v", expectedSecretId, api.secretId)
	}
}

type appRoleVaultAuthApiStub struct {
	path     string
	roleId   string
	secretId string
}

func (stub *appRoleVaultAuthApiStub) LoginToBackend(path string, credentials map[string]interface{}) (leaseDuration time.Duration, err error) {
	stub.path = path
	stub.roleId = credentials["role_id"].(string)
	stub.secretId = credentials["secret_id"].(string)
	return 0, nil
}

func (stub *appRoleVaultAuthApiStub) LoginWithToken(token string) (leaseDuration time.Duration, err error) {
	return 0, errors.New("not implemented")
}

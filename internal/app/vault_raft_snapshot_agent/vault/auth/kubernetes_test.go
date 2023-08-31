package auth

import (
	"bufio"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCreateDefaultKubernetesAuth(t *testing.T) {
	authPath := "test"
	jwtPath := "/tmp/kubernetes"
	expectedLoginPath := "auth/" + authPath + "/login"
	expectedRole := "testRole"
	expectedJwt, createdFile := createJwtFile(t, jwtPath, "testSecret")
	if createdFile {
		defer os.Remove(jwtPath)
	}

	config := KubernetesAuthConfig{
		Path:    authPath,
		Role:    expectedRole,
		JWTPath: jwtPath,
	}

	authApiStub := kubernetesVaultAuthApiStub{}

	auth := createKubernetesAuth(config)
	auth.Refresh(&authApiStub)

	assertKubernetesAuthValues(t, expectedLoginPath, expectedRole, expectedJwt, auth, authApiStub)
}

func TestCreateKubernetesAuthWithMissingJwtPath(t *testing.T) {
	authPath := "test"
	customJwtPath := "./does/not/exist"
	expectedRole := "testRole"

	config := KubernetesAuthConfig{
		Path:    authPath,
		Role:    expectedRole,
		JWTPath: customJwtPath,
	}

	authApiStub := kubernetesVaultAuthApiStub{}

	auth := createKubernetesAuth(config)
	_, err := auth.Refresh(&authApiStub)
	if err == nil {
		t.Fatalf("kubernetes auth refresh does not fail when jwt-file is missing")
	}
}

func assertKubernetesAuthValues(t *testing.T, expectedLoginPath string, expectedRole string, expectedJwt string, auth authBackend, api kubernetesVaultAuthApiStub) {
	if auth.name != "Kubernetes" {
		t.Fatalf("AppRoleAuth has wrong name - expected AppRole, got %v", auth.name)
	}

	if api.path != expectedLoginPath {
		t.Fatalf("default path of kubernetes-auth is not approle - got: %v", api.path)
	}

	if api.role != expectedRole {
		t.Fatalf("auth did not pass correct role-id - expected %v, got %v", expectedRole, api.role)
	}

	if api.jwt != expectedJwt {
		t.Fatalf("auth did not pass correct role-id - expected %v, got %v", expectedJwt, api.jwt)
	}
}

type kubernetesVaultAuthApiStub struct {
	path string
	role string
	jwt  string
}

func (stub *kubernetesVaultAuthApiStub) LoginToBackend(path string, credentials map[string]interface{}) (leaseDuration time.Duration, err error) {
	stub.path = path
	stub.role = credentials["role"].(string)
	stub.jwt = credentials["jwt"].(string)
	return 0, nil
}

func (stub *kubernetesVaultAuthApiStub) LoginWithToken(token string) (leaseDuration time.Duration, err error) {
	return 0, errors.New("not implemented")
}

func createJwtFile(t *testing.T, path string, defaultJwt string) (jwt string, created bool) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0777); err != nil && !errors.Is(err, os.ErrExist) {
		t.Fatalf("could not create directorys for jwt-file %s: %v", path, err)
	}

	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		t.Fatalf("could not create jwt-file %s: %v", path, err)
	}

	defer file.Close()

	content, err := io.ReadAll(bufio.NewReader(file))
	if err != nil {
		t.Fatalf("could not read jwt-file %s: %v", path, err)
	}

	if len(content) > 0 {
		return string(content), false
	} else {
		_, err := file.Write([]byte(defaultJwt))
		if err != nil {
			t.Fatalf("could not write expected secret to %s: %v", path, err)
		}
		return defaultJwt, true
	}
}
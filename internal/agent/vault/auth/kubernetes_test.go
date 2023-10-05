package auth

import (
	"fmt"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/config/secret"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/test"
	"github.com/hashicorp/vault/api/auth/kubernetes"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCreateKubernetesAuth(t *testing.T) {
	jwtPath := fmt.Sprintf("%s/jwt", t.TempDir())
	config := KubernetesAuthConfig{
		Role:     "test-role",
		JWTToken: secret.Secret(fmt.Sprintf("file://%s", jwtPath)),
		Path:     "test-path",
	}

	err := test.WriteFile(t, jwtPath, "test")
	assert.NoError(t, err, "could not write jwt-file")

	expectedAuthMethod, err := kubernetes.NewKubernetesAuth(
		config.Role,
		kubernetes.WithMountPath(config.Path),
		kubernetes.WithServiceAccountToken("test"),
	)
	assert.NoError(t, err, "NewKubernetesAuth failed unexpectedly")

	authMethod, err := config.createAuthMethod()
	assert.NoError(t, err, "createKubernetesAuth failed unexpectedly")

	assert.Equal(t, expectedAuthMethod, authMethod)
}

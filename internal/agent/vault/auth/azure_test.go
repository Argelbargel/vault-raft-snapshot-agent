package auth

import (
	"testing"

	"github.com/hashicorp/vault/api/auth/azure"
	"github.com/stretchr/testify/assert"
)

func TestCreateAzureAuth(t *testing.T) {
	config := AzureAuthConfig{
		Role:     "test-role",
		Resource: "test-resource",
		Path:     "test-path",
	}

	expectedAuthMethod, err := azure.NewAzureAuth(
		config.Role,
		azure.WithResource(config.Resource),
		azure.WithMountPath(config.Path),
	)
	assert.NoError(t, err, "NewAzureAuth failed unexpectedly")

	authMethod, err := createAzureAuth(config).createAuthMethod()
	assert.NoError(t, err, "createAuthMethod failed unexpectedly")

	assert.Equal(t, expectedAuthMethod, authMethod)
}

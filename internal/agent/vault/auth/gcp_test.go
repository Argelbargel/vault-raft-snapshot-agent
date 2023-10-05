package auth

import (
	"testing"

	"github.com/hashicorp/vault/api/auth/gcp"
	"github.com/stretchr/testify/assert"
)

func TestCreateGCPGCEAuth(t *testing.T) {
	config := GCPAuthConfig{
		Role: "test-role",
		Path: "test-path",
	}

	expectedAuthMethod, err := gcp.NewGCPAuth(
		config.Role,
		gcp.WithGCEAuth(),
		gcp.WithMountPath("test-path"),
	)
	assert.NoError(t, err, "NewGCPAuth failed unexpectedly")

	authMethod, err := config.createAuthMethod()
	assert.NoError(t, err, "createAuthMethod failed unexpectedly")

	assert.Equal(t, expectedAuthMethod, authMethod)
}

func TestCreateGCPIAMAuth(t *testing.T) {
	config := GCPAuthConfig{
		Role:                "test-role",
		ServiceAccountEmail: "test@email.com",
		Path:                "test-path",
	}

	expectedAuthMethod, err := gcp.NewGCPAuth(
		config.Role,
		gcp.WithIAMAuth(config.ServiceAccountEmail),
		gcp.WithMountPath("test-path"),
	)
	assert.NoError(t, err, "NewGCPAuth failed unexpectedly")

	authMethod, err := config.createAuthMethod()
	assert.NoError(t, err, "createAuthMethod failed unexpectedly")

	assert.Equal(t, expectedAuthMethod, authMethod)
}

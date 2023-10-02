package auth

import (
	"testing"

	"github.com/hashicorp/vault/api/auth/approle"
	"github.com/stretchr/testify/assert"
)

func TestCreateAppRoleAuth(t *testing.T) {
	config := AppRoleAuthConfig{
		RoleId:   "test-role",
		SecretId: "test-secret",
		Path:     "test-path",
	}

	expectedAuthMethod, err := approle.NewAppRoleAuth(
		config.RoleId.String(),
		&approle.SecretID{FromString: config.SecretId.String()},
		approle.WithMountPath(config.Path),
	)
	assert.NoError(t, err, "NewAppRoleAuth failed unexpectedly")

	method, err := config.createAuthMethod()
	assert.NoError(t, err, "createAuthMethod failed unexpectedly")

	assert.Equal(t, expectedAuthMethod, method)
}

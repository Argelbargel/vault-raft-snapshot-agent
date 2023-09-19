package auth

import (
	"github.com/hashicorp/vault/api/auth/userpass"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCreateUserpassAuth(t *testing.T) {
	config := UserPassAuthConfig{
		Username: "test-user",
		Password: "test-password",
		Path:     "test-path",
	}

	expectedAuthMethod, err := userpass.NewUserpassAuth(
		config.Username.String(),
		&userpass.Password{FromString: config.Password.String()},
		userpass.WithMountPath(config.Path),
	)
	assert.NoError(t, err, "NewUserPassAuth failed unexpectedly")

	authMethod, err := createUserPassAuth(config).createAuthMethod()
	assert.NoError(t, err, "createAuthMethod failed unexpectedly")

	assert.Equal(t, expectedAuthMethod, authMethod)
}

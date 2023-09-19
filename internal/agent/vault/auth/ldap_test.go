package auth

import (
	"github.com/hashicorp/vault/api/auth/ldap"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCreateLDAPAuth(t *testing.T) {
	config := LDAPAuthConfig{
		Username: "test-user",
		Password: "test-password",
		Path:     "test-path",
	}

	expectedAuthMethod, err := ldap.NewLDAPAuth(
		config.Username.String(),
		&ldap.Password{FromString: config.Password.String()},
		ldap.WithMountPath(config.Path),
	)
	assert.NoError(t, err, "NewLDAPAuth failed unexpectedly")

	authMethod, err := createLDAPAuth(config).createAuthMethod()
	assert.NoError(t, err, "createAuthMethod failed unexpectedly")

	assert.Equal(t, expectedAuthMethod, authMethod)
}

package auth

import (
	"github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/api/auth/azure"
)

type AzureAuthConfig struct {
	Path     string `default:"azure"`
	Role     string `validate:"required"`
	Resource string
}

func (config AzureAuthConfig) createAuthMethod() (api.AuthMethod, error) {
	var loginOpts = []azure.LoginOption{azure.WithMountPath(config.Path)}

	if config.Resource != "" {
		loginOpts = append(loginOpts, azure.WithResource(config.Resource))
	}

	return azure.NewAzureAuth(config.Role, loginOpts...)
}

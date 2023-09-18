package auth

import "github.com/hashicorp/vault/api/auth/azure"

type AzureAuthConfig struct {
	Path     string `default:"azure"`
	Role     string `validate:"required_if=Empty false"`
	Resource string
	Empty    bool
}

func createAzureAuth(config AzureAuthConfig) vaultAuthMethod[AzureAuthConfig, *azure.AzureAuth] {
	return vaultAuthMethod[AzureAuthConfig, *azure.AzureAuth]{
		config,
		func(config AzureAuthConfig) (*azure.AzureAuth, error) {
			var loginOpts = []azure.LoginOption{azure.WithMountPath(config.Path)}

			if config.Resource != "" {
				loginOpts = append(loginOpts, azure.WithResource(config.Resource))
			}

			return azure.NewAzureAuth(config.Role, loginOpts...)
		},
	}
}

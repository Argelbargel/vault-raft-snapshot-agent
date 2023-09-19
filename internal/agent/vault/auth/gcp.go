package auth

import "github.com/hashicorp/vault/api/auth/gcp"

type GCPAuthConfig struct {
	Path                string `default:"gcp"`
	Role                string `validate:"required_if=Empty false"`
	ServiceAccountEmail string
	Empty               bool
}

func createGCPAuth(config GCPAuthConfig) vaultAuthMethod[GCPAuthConfig, *gcp.GCPAuth] {
	return vaultAuthMethod[GCPAuthConfig, *gcp.GCPAuth]{
		config,
		func(config GCPAuthConfig) (*gcp.GCPAuth, error) {
			var loginOpts = []gcp.LoginOption{gcp.WithMountPath(config.Path)}

			if config.ServiceAccountEmail != "" {
				loginOpts = append(loginOpts, gcp.WithIAMAuth(config.ServiceAccountEmail))
			} else {
				loginOpts = append(loginOpts, gcp.WithGCEAuth())
			}

			return gcp.NewGCPAuth(config.Role, loginOpts...)
		},
	}
}

package auth

import (
	"github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/api/auth/gcp"
)

type GCPAuthConfig struct {
	Path                string `default:"gcp"`
	Role                string `validate:"required"`
	ServiceAccountEmail string
}

func (config GCPAuthConfig) createAuthMethod() (api.AuthMethod, error) {
	var loginOpts = []gcp.LoginOption{gcp.WithMountPath(config.Path)}

	if config.ServiceAccountEmail != "" {
		loginOpts = append(loginOpts, gcp.WithIAMAuth(config.ServiceAccountEmail))
	} else {
		loginOpts = append(loginOpts, gcp.WithGCEAuth())
	}

	return gcp.NewGCPAuth(config.Role, loginOpts...)
}

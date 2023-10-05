package auth

import (
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/config/secret"
	"github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/api/auth/kubernetes"
)

type KubernetesAuthConfig struct {
	Path     string        `default:"kubernetes"`
	Role     string        `validate:"required_if=Empty false"`
	JWTToken secret.Secret `default:"file:///var/run/secrets/kubernetes.io/serviceaccount/token" validate:"required_if=Empty false"`
	Empty    bool
}

func (config KubernetesAuthConfig) createAuthMethod() (api.AuthMethod, error) {
	token, err := config.JWTToken.Resolve(true)
	if err != nil {
		return nil, err
	}

	var loginOpts = []kubernetes.LoginOption{
		kubernetes.WithMountPath(config.Path),
		kubernetes.WithServiceAccountToken(token),
	}

	return kubernetes.NewKubernetesAuth(config.Role, loginOpts...)
}

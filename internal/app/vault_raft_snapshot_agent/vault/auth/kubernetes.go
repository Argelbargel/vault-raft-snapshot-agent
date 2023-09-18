package auth

import (
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/app/vault_raft_snapshot_agent/secret"
	"github.com/hashicorp/vault/api/auth/kubernetes"
)

type KubernetesAuthConfig struct {
	Path     string        `default:"kubernetes"`
	Role     string        `validate:"required_if=Empty false"`
	JWTToken secret.Secret `default:"file:///var/run/secrets/kubernetes.io/serviceaccount/token" validate:"required_if=Empty false"`
	Empty    bool
}

func createKubernetesAuth(config KubernetesAuthConfig) vaultAuthMethod[KubernetesAuthConfig, *kubernetes.KubernetesAuth] {
	return vaultAuthMethod[KubernetesAuthConfig, *kubernetes.KubernetesAuth]{
		config,
		func(config KubernetesAuthConfig) (*kubernetes.KubernetesAuth, error) {
			token, err := config.JWTToken.Resolve(true)
			if err != nil {
				return nil, err
			}

			var loginOpts = []kubernetes.LoginOption{
				kubernetes.WithMountPath(config.Path),
				kubernetes.WithServiceAccountToken(token),
			}

			return kubernetes.NewKubernetesAuth(config.Role, loginOpts...)
		},
	}
}

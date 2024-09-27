package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/logging"
	"github.com/hashicorp/vault/api"
)

type VaultAuth interface {
	Refresh(context.Context, *api.Client, bool) error
}

type vaultAuthMethodFactory interface {
	createAuthMethod() (api.AuthMethod, error)
}

type vaultAuthImpl struct {
	factory vaultAuthMethodFactory
	expires time.Time
}

func CreateVaultAuth(config VaultAuthConfig) (VaultAuth, error) {
	if config.AppRole != nil {
		return &vaultAuthImpl{factory: config.AppRole}, nil
	} else if config.AWS != nil {
		return &vaultAuthImpl{factory: config.AWS}, nil
	} else if config.Azure != nil {
		return &vaultAuthImpl{factory: config.Azure}, nil
	} else if config.GCP != nil {
		return &vaultAuthImpl{factory: config.GCP}, nil
	} else if config.Kubernetes != nil {
		return &vaultAuthImpl{factory: config.Kubernetes}, nil
	} else if config.LDAP != nil {
		return &vaultAuthImpl{factory: config.LDAP}, nil
	} else if config.UserPass != nil {
		return &vaultAuthImpl{factory: config.UserPass}, nil
	} else if config.Token != nil {
		return &vaultAuthImpl{factory: config.Token}, nil
	} else {
		return nil, fmt.Errorf("unknown authenticatin method")
	}
}

func (auth *vaultAuthImpl) Refresh(ctx context.Context, client *api.Client, force bool) error {
	if !force && auth.expires.After(time.Now()) {
		return nil
	}

	method, err := auth.factory.createAuthMethod()
	if err != nil {
		return err
	}

	logging.Debug("Logging into vault", "method", fmt.Sprintf("%T", method))
	authSecret, err := client.Auth().Login(ctx, method)
	if err != nil {
		return err
	}

	tokenTTL, err := authSecret.TokenTTL()
	if err != nil {
		return err
	}

	tokenPolicies, err := authSecret.TokenPolicies()
	if err != nil {
		return err
	}

	auth.expires = time.Now().Add(tokenTTL / 2)
	logging.Debug("Successfully logged in ", "policies", tokenPolicies, "ttl", tokenTTL, "expires", auth.expires)
	return nil
}

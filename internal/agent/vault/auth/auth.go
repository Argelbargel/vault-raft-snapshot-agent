package auth

import (
	"context"
	"fmt"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/logging"
	"github.com/hashicorp/vault/api"
)

type VaultAuthConfig struct {
	AppRole    *AppRoleAuthConfig
	AWS        *AWSAuthConfig
	Azure      *AzureAuthConfig
	GCP        *GCPAuthConfig
	Kubernetes *KubernetesAuthConfig
	LDAP       *LDAPAuthConfig
	UserPass   *UserPassAuthConfig
	Token      *Token
}

type vaultAuthMethodFactory interface {
	createAuthMethod() (api.AuthMethod, error)
}

type vaultAuthMethodImpl struct {
	methodFactory vaultAuthMethodFactory
}

func CreateVaultAuth(config VaultAuthConfig) (api.AuthMethod, error) {
	if config.AppRole != nil {
		return vaultAuthMethodImpl{config.AppRole}, nil
	} else if config.AWS != nil {
		return vaultAuthMethodImpl{config.AWS}, nil
	} else if config.Azure != nil {
		return vaultAuthMethodImpl{config.Azure}, nil
	} else if config.GCP != nil {
		return vaultAuthMethodImpl{config.GCP}, nil
	} else if config.Kubernetes != nil {
		return vaultAuthMethodImpl{config.Kubernetes}, nil
	} else if config.LDAP != nil {
		return vaultAuthMethodImpl{config.LDAP}, nil
	} else if config.UserPass != nil {
		return vaultAuthMethodImpl{config.UserPass}, nil
	} else if config.Token != nil {
		return vaultAuthMethodImpl{config.Token}, nil
	} else {
		return nil, fmt.Errorf("unknown authenticatin method")
	}
}

func (am vaultAuthMethodImpl) Login(ctx context.Context, client *api.Client) (*api.Secret, error) {
	method, err := am.methodFactory.createAuthMethod()
	if err != nil {
		return nil, err
	}

	logging.Debug("Logging into vault", "method", fmt.Sprintf("%T", method))
	return client.Auth().Login(ctx, method)
}

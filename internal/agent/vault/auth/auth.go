package auth

import (
	"context"
	"fmt"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/logging"
	"github.com/hashicorp/vault/api"
)

type VaultAuthConfig struct {
	AppRole    AppRoleAuthConfig    `default:"{\"Empty\": true}"`
	AWS        AWSAuthConfig        `default:"{\"Empty\": true}"`
	Azure      AzureAuthConfig      `default:"{\"Empty\": true}"`
	GCP        GCPAuthConfig        `default:"{\"Empty\": true}"`
	Kubernetes KubernetesAuthConfig `default:"{\"Empty\": true}"`
	LDAP       LDAPAuthConfig       `default:"{\"Empty\": true}"`
	UserPass   UserPassAuthConfig   `default:"{\"Empty\": true}"`
	Token      Token
}

type vaultAuthMethodFactory interface {
	createAuthMethod() (api.AuthMethod, error)
}

type vaultAuthMethodImpl struct {
	methodFactory vaultAuthMethodFactory
}

func CreateVaultAuth(config VaultAuthConfig) (api.AuthMethod, error) {
	if !config.AppRole.Empty {
		return vaultAuthMethodImpl{config.AppRole}, nil
	} else if !config.AWS.Empty {
		return vaultAuthMethodImpl{config.AWS}, nil
	} else if !config.Azure.Empty {
		return vaultAuthMethodImpl{config.Azure}, nil
	} else if !config.GCP.Empty {
		return vaultAuthMethodImpl{config.GCP}, nil
	} else if !config.Kubernetes.Empty {
		return vaultAuthMethodImpl{config.Kubernetes}, nil
	} else if !config.LDAP.Empty {
		return vaultAuthMethodImpl{config.LDAP}, nil
	} else if !config.UserPass.Empty {
		return vaultAuthMethodImpl{config.UserPass}, nil
	} else if config.Token != "" {
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

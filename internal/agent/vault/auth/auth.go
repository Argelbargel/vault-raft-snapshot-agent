package auth

import (
	"context"
	"fmt"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/config/secret"
	"time"

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
	Token      secret.Secret
}

type VaultAuth[C any] interface {
	Login(ctx context.Context, client C) (time.Duration, error)
}

type vaultAuthMethod[C any, M api.AuthMethod] struct {
	config        C
	methodFactory func(config C) (M, error)
}

func CreateVaultAuth(config VaultAuthConfig) (VaultAuth[*api.Client], error) {
	if !config.AppRole.Empty {
		return createAppRoleAuth(config.AppRole), nil
	} else if !config.AWS.Empty {
		return createAWSAuth(config.AWS), nil
	} else if !config.Azure.Empty {
		return createAzureAuth(config.Azure), nil
	} else if !config.GCP.Empty {
		return createGCPAuth(config.GCP), nil
	} else if !config.Kubernetes.Empty {
		return createKubernetesAuth(config.Kubernetes), nil
	} else if !config.LDAP.Empty {
		return createLDAPAuth(config.LDAP), nil
	} else if !config.UserPass.Empty {
		return createUserPassAuth(config.UserPass), nil
	} else if config.Token != "" {
		return createTokenAuth(config.Token), nil
	} else {
		return nil, fmt.Errorf("unknown authenticatin method")
	}
}

func (am vaultAuthMethod[C, M]) Login(ctx context.Context, client *api.Client) (time.Duration, error) {
	method, err := am.methodFactory(am.config)
	if err != nil {
		return 0, err
	}

	authSecret, err := method.Login(ctx, client)
	if err != nil {
		return 0, err
	}

	return time.Duration(authSecret.LeaseDuration), nil
}

func (am vaultAuthMethod[C, M]) createAuthMethod() (M, error) {
	return am.methodFactory(am.config)
}

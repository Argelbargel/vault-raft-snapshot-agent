package auth

import (
	"context"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/config/secret"
	"github.com/hashicorp/vault/api"
	"time"
)

type Token secret.Secret

type tokenAuth struct {
	token string
}

type tokenLookup interface {
	Lookup(context.Context, *api.Client) (*api.Secret, error)
}

func (t Token) createAuthMethod() (api.AuthMethod, error) {

	token, err := secret.Secret(t).Resolve(true)
	if err != nil {
		return nil, err
	}

	return tokenAuth{token}, nil
}

func (auth tokenAuth) Login(ctx context.Context, client *api.Client) (*api.Secret, error) {
	return auth.login(ctx, client, vaultTokenLookup{})
}

func (auth tokenAuth) login(ctx context.Context, client *api.Client, lookup tokenLookup) (*api.Secret, error) {
	client.SetToken(auth.token)

	authSecret, err := lookup.Lookup(ctx, client)
	if err != nil {
		client.ClearToken()
		return nil, err
	}

	if authSecret.Auth == nil {
		authSecret.Auth = &api.SecretAuth{
			LeaseDuration: int(24 * time.Hour.Seconds()),
			ClientToken:   auth.token,
		}
	}

	return authSecret, nil
}

type vaultTokenLookup struct{}

func (impl vaultTokenLookup) Lookup(ctx context.Context, client *api.Client) (*api.Secret, error) {
	return client.Auth().Token().LookupSelfWithContext(ctx)
}

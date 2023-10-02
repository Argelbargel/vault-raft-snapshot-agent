package auth

import (
	"context"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/config/secret"
	"github.com/hashicorp/vault/api"
)

type Token secret.Secret

type tokenAuth struct {
	token string
}

type tokenAuthAPI interface {
	SetToken(token string)
	LookupToken() (*api.Secret, error)
	ClearToken()
}

func (t Token) createAuthMethod() (api.AuthMethod, error) {
	token, err := secret.Secret(t).Resolve(true)
	if err != nil {
		return nil, err
	}

	return tokenAuth{token}, nil
}

func (auth tokenAuth) Login(_ context.Context, client *api.Client) (*api.Secret, error) {
	return auth.login(tokenAuthImpl{client})
}

func (auth tokenAuth) login(authAPI tokenAuthAPI) (*api.Secret, error) {
	authAPI.SetToken(auth.token)
	authSecret, err := authAPI.LookupToken()
	if err != nil {
		authAPI.ClearToken()
		return nil, err
	}

	return authSecret, nil

}

type tokenAuthImpl struct {
	client *api.Client
}

func (impl tokenAuthImpl) SetToken(token string) {
	impl.client.SetToken(token)
}

func (impl tokenAuthImpl) LookupToken() (*api.Secret, error) {
	return impl.client.Auth().Token().LookupSelf()
}

func (impl tokenAuthImpl) ClearToken() {
	impl.client.ClearToken()
}

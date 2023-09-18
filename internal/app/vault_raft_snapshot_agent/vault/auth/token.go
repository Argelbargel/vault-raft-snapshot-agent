package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/app/vault_raft_snapshot_agent/secret"
	"time"

	"github.com/hashicorp/vault/api"
)

type tokenAuth struct {
	token secret.Secret
}

type tokenAuthAPI interface {
	SetToken(token string)
	LookupToken() (*api.Secret, error)
	ClearToken()
}

func createTokenAuth(token secret.Secret) tokenAuth {
	return tokenAuth{token}
}

func (auth tokenAuth) Login(_ context.Context, client *api.Client) (time.Duration, error) {
	return auth.login(tokenAuthImpl{client})
}

func (auth tokenAuth) login(authAPI tokenAuthAPI) (time.Duration, error) {
	token, err := auth.token.Resolve(true)
	if err != nil {
		return 0, err
	}

	authAPI.SetToken(token)
	info, err := authAPI.LookupToken()
	if err != nil {
		authAPI.ClearToken()
		return 0, err
	}

	ttl, err := info.Data["ttl"].(json.Number).Int64()
	if err != nil {
		authAPI.ClearToken()
		return 0, fmt.Errorf("error converting ttl to int: %s", err)
	}

	return time.Duration(ttl), nil

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

package vault

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"github.com/hashicorp/vault/api"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestClientRefreshesAuthAfterTokenExpires(t *testing.T) {
	auth := &authMethodStub{
		leaseDuration: time.Minute,
	}

	client := NewClient(
		&vaultAPIStub{
			leader: true,
		},
		auth,
		time.Now().Add(time.Second*1),
	)

	_ = client.TakeSnapshot(context.Background(), bufio.NewWriter(&bytes.Buffer{}))

	assertAuthRefresh(t, false, client, auth)

	time.Sleep(time.Second)

	_ = client.TakeSnapshot(context.Background(), bufio.NewWriter(&bytes.Buffer{}))

	assertAuthRefresh(t, true, client, auth)
}

func TestClientDoesNotTakeSnapshotIfAuthRefreshFails(t *testing.T) {
	auth := &authMethodStub{}
	vaultApi := &vaultAPIStub{
		leader: true,
	}

	initalAuthExpiration := time.Now().Add(time.Second * -1)
	client := NewClient(
		vaultApi,
		auth,
		initalAuthExpiration,
	)

	err := client.TakeSnapshot(context.Background(), bufio.NewWriter(&bytes.Buffer{}))

	assert.Error(t, err, "TakeSnapshot() returned no error although auth-refresh failed")
	assert.Equal(t, initalAuthExpiration, client.authExpiration, "TakeSnapshot() refreshed auth-expiration although auth-refresh failed")
	assert.False(t, vaultApi.snapshotTaken, "TakeSnapshot() took snapshot although aut-refresh failed")
}

func TestClientOnlyTakesSnapshotWhenLeader(t *testing.T) {
	vaultApi := &vaultAPIStub{
		leader: false,
	}
	client := NewClient(
		vaultApi,
		&authMethodStub{},
		time.Now().Add(time.Minute),
	)

	ctx := context.Background()
	writer := bufio.NewWriter(&bytes.Buffer{})

	err := client.TakeSnapshot(ctx, writer)

	assert.Error(t, err, "TakeSnapshot() reported no error although not leader!")
	assert.False(t, vaultApi.snapshotTaken, "TakeSnapshot() took snapshot when not leader!")

	vaultApi.leader = true
	err = client.TakeSnapshot(ctx, writer)

	assert.NoError(t, err, "TakeSnapshot() failed unexpectedly")
	assert.True(t, vaultApi.snapshotTaken, "TakeSnapshot() took no snapshot when leader")
	assert.Equal(t, ctx, vaultApi.snapshotContext)
	assert.Equal(t, writer, vaultApi.snapshotWriter)
}

func TestClientDoesNotTakeSnapshotIfLeaderCheckFails(t *testing.T) {
	auth := &authMethodStub{}
	vaultApi := &vaultAPIStub{
		sysLeaderFails: true,
		leader:         true,
	}

	client := NewClient(
		vaultApi,
		auth,
		time.Now(),
	)

	err := client.TakeSnapshot(context.Background(), bufio.NewWriter(&bytes.Buffer{}))

	assert.Error(t, err, "TakeSnapshot() reported success or returned no error when leader-check failed")
	assert.False(t, vaultApi.snapshotTaken, "TakeSnapshot() took snapshot when leader-check failed")
	assert.NotEqual(t, auth.leaseDuration, client.authExpiration)
}

func assertAuthRefresh(t *testing.T, refreshed bool, client *VaultClient, auth *authMethodStub) {
	t.Helper()

	if auth.refreshed != refreshed {
		if !auth.refreshed {
			t.Fatalf("TakeSnapshot did not call Auth#Refresh() when expected")
		}
		if auth.refreshed {
			t.Fatalf("TakeSnapshot did call Auth#Refresh() unexpectedly")
		}
	}

	if refreshed {
		assert.WithinDuration(t, time.Now().Add(auth.leaseDuration/2), client.authExpiration, time.Second, "client did not refresh auth-expiration!")
	}
}

type vaultAPIStub struct {
	leader          bool
	sysLeaderFails  bool
	snapshotTaken   bool
	snapshotContext context.Context
	snapshotWriter  io.Writer
}

func (stub *vaultAPIStub) Address() string {
	return "test"
}

func (stub *vaultAPIStub) TakeSnapshot(ctx context.Context, writer io.Writer) error {
	stub.snapshotTaken = true
	stub.snapshotContext = ctx
	stub.snapshotWriter = writer
	return nil
}

func (stub *vaultAPIStub) IsLeader() (bool, error) {
	if stub.sysLeaderFails {
		return false, errors.New("leader-Check failed")
	}

	return stub.leader, nil
}

func (stub *vaultAPIStub) RefreshAuth(ctx context.Context, auth api.AuthMethod) (time.Duration, error) {
	authSecret, err := auth.Login(ctx, nil)
	if err != nil {
		return 0, err
	}

	ttl, err := authSecret.TokenTTL()
	if err != nil {
		return 0, err
	}

	return ttl, nil
}

type authMethodStub struct {
	leaseDuration time.Duration
	refreshed     bool
}

func (a *authMethodStub) Login(context.Context, *api.Client) (*api.Secret, error) {
	a.refreshed = true
	if a.leaseDuration <= 0 {
		return nil, errors.New("refresh of auth failed")
	}

	return &api.Secret{
		Auth: &api.SecretAuth{
			LeaseDuration: int(a.leaseDuration.Seconds()),
		},
	}, nil
}

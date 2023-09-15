package vault

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestClientRefreshesAuthAfterTokenExpires(t *testing.T) {
	auth := &clientVaultAPIAuthStub{
		leaseDuration: time.Minute,
	}

	client := NewVaultClient[any, *clientVaultAPIAuthStub](
		&clientVaultAPIStub{
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
	authStub := &clientVaultAPIAuthStub{}
	clientApi := &clientVaultAPIStub{
		leader: true,
	}

	client := NewVaultClient[any, *clientVaultAPIAuthStub](
		clientApi,
		authStub,
		time.Now().Add(time.Second*-1),
	)

	err := client.TakeSnapshot(context.Background(), bufio.NewWriter(&bytes.Buffer{}))

	assert.Error(t, err, "TakeSnapshot() returned no error although auth-refresh failed")
	assert.NotEqual(t, authStub.leaseDuration, client.tokenExpiration, "TakeSnapshot() refreshed token-expiration although auth-refresh failed")
	assert.False(t, clientApi.snapshotTaken, "TakeSnapshot() took snapshot although aut-refresh failed")
}

func TestClientOnlyTakesSnaphotWhenLeader(t *testing.T) {
	clientApi := &clientVaultAPIStub{
		leader: false,
	}
	client := NewVaultClient[any, *clientVaultAPIAuthStub](
		clientApi,
		&clientVaultAPIAuthStub{},
		time.Now(),
	)

	ctx := context.Background()
	writer := bufio.NewWriter(&bytes.Buffer{})

	err := client.TakeSnapshot(ctx, writer)

	assert.Error(t, err, "TakeSnapshot() reported no error although not leader!")
	assert.False(t, clientApi.snapshotTaken, "TakeSnapshot() took snapshot when not leader!")

	clientApi.leader = true
	err = client.TakeSnapshot(ctx, writer)

	assert.NoError(t, err, "TakeSnapshot() failed unexpectedly")
	assert.True(t, clientApi.snapshotTaken, "TakeSnapshot() took no snapshot when leader")
	assert.Equal(t, ctx, clientApi.snapshotContext)
	assert.Equal(t, writer, clientApi.snapshotWriter)
}

func TestClientDoesNotTakeSnapshotIfLeaderCheckFails(t *testing.T) {
	authStub := &clientVaultAPIAuthStub{}
	api := &clientVaultAPIStub{
		sysLeaderFails: true,
		leader:         true,
	}

	client := NewVaultClient[any, *clientVaultAPIAuthStub](
		api,
		authStub,
		time.Now(),
	)

	err := client.TakeSnapshot(context.Background(), bufio.NewWriter(&bytes.Buffer{}))

	assert.Error(t, err, "TakeSnapshot() reported success or returned no error when leader-check failed")
	assert.False(t, api.snapshotTaken, "TakeSnapshot() took snapshot when leader-check failed")
	assert.NotEqual(t, authStub.leaseDuration, client.tokenExpiration)
}

func assertAuthRefresh(t *testing.T, refreshed bool, client *VaultClient[any, *clientVaultAPIAuthStub], auth *clientVaultAPIAuthStub) {
	t.Helper()

	if auth.refreshed != refreshed {
		if !auth.refreshed {
			t.Fatalf("TakeSnapshot did not call Auth#Refresh() when expected")
		}
		if auth.refreshed {
			t.Fatalf("TakeSnapshot did call Auth#Refresh() unexpectedly")
		}
	}

	expectedTokenExpiration := time.Now().Add((time.Second * auth.leaseDuration) / 2)
	if refreshed && client.tokenExpiration.Before(expectedTokenExpiration) {
		t.Fatalf("client did not accept tokenExpiration from auth! expected: %v, client: %v, lease-duration: %v", expectedTokenExpiration, client.tokenExpiration, auth.leaseDuration)
	}
}

type clientVaultAPIStub struct {
	leader          bool
	sysLeaderFails  bool
	snapshotTaken   bool
	snapshotContext context.Context
	snapshotWriter  io.Writer
}

func (stub *clientVaultAPIStub) Address() string {
	return "test"
}

func (stub *clientVaultAPIStub) TakeSnapshot(ctx context.Context, writer io.Writer) error {
	stub.snapshotTaken = true
	stub.snapshotContext = ctx
	stub.snapshotWriter = writer
	return nil
}

func (stub *clientVaultAPIStub) IsLeader() (bool, error) {
	if stub.sysLeaderFails {
		return false, errors.New("leader-Check failed")
	}

	return stub.leader, nil
}

func (stub *clientVaultAPIStub) RefreshAuth(ctx context.Context, auth *clientVaultAPIAuthStub) (time.Duration, error) {
	return auth.Login(ctx, nil)
}

type clientVaultAPIAuthStub struct {
	leaseDuration time.Duration
	refreshed     bool
}

func (a *clientVaultAPIAuthStub) Login(ctx context.Context, api any) (time.Duration, error) {
	a.refreshed = true
	var err error
	if a.leaseDuration <= 0 {
		err = errors.New("refresh of auth failed")
	}
	return a.leaseDuration, err
}

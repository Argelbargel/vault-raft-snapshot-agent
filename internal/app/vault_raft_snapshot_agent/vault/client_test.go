package vault

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/app/vault_raft_snapshot_agent/vault/auth"
)

func TestClientRefreshesAuthAfterTokenExpires(t *testing.T) {
	auth := &authStub{
		tokenExpiration: time.Now(),
	}

	client := VaultClient{
		api: &clientAPIStub{
			leader: false,
		},
		auth:            auth,
		tokenExpiration: time.Now().Add(time.Second * 1),
	}

	client.TakeSnapshot(context.Background(), bufio.NewWriter(&bytes.Buffer{}))

	assertAuthRefresh(t, false, client, auth)

	time.Sleep(time.Second)

	client.TakeSnapshot(context.Background(), bufio.NewWriter(&bytes.Buffer{}))

	assertAuthRefresh(t, true, client, auth)
}

func TestClientDoesNotTakeSnapshotIfAuthRefreshFails(t *testing.T) {
	authStub := &authStub{}
	clientApi := &clientAPIStub{
		leader: true,
	}

	client := VaultClient{
		api:             clientApi,
		auth:            authStub,
		tokenExpiration: time.Now().Add(time.Second * -1),
	}

	err := client.TakeSnapshot(context.Background(), bufio.NewWriter(&bytes.Buffer{}))
	if err == nil {
		t.Fatalf("TakeSnapshot() returned no error although auth-refresh failed")
	}

	if client.tokenExpiration == authStub.tokenExpiration {
		t.Fatalf("TakeSnapshot() refreshed token-expiration although auth-refresh failed: client: %v, auth: %v", client.tokenExpiration, authStub.tokenExpiration)
	}

	if clientApi.snapshotTaken {
		t.Fatalf("TakeSnapshot took snapshot although aut-refresh failed")
	}
}

func TestClientOnlyTakesSnaphotWhenLeader(t *testing.T) {
	clientApi := &clientAPIStub{
		leader: false,
	}
	client := VaultClient{
		api:             clientApi,
		auth:            nil,
		tokenExpiration: time.Now(),
	}

	ctx := context.Background()
	writer := bufio.NewWriter(&bytes.Buffer{})

	err := client.TakeSnapshot(ctx, writer)
	if err == nil {
		t.Fatalf("TakeSnapshot() reported no error although not leader!")
	}

	if clientApi.snapshotTaken {
		t.Fatalf("TakeSnapshot() took snapshot when not leader!")
	}

	clientApi.leader = true
	err = client.TakeSnapshot(ctx, writer)
	if err != nil {
		t.Fatalf("TakeSnapshot() failed unexpectedly: %v", err)
	}

	if !clientApi.snapshotTaken {
		t.Fatalf("TakeSnapshot() took no snapshot when leader")
	}

	if clientApi.snapshotContext != ctx {
		t.Fatalf("TakeSnapshot() did not pass context to api (expected: %v, api: %v)", ctx, clientApi.snapshotContext)
	}

	if clientApi.snapshotWriter != writer {
		t.Fatalf("TakeSnapshot() did not pass writer to api (expected: %v, api: %v)", writer, clientApi.snapshotWriter)
	}
}

func TestClientDoesNotTakeSnapshotIfLeaderCheckFails(t *testing.T) {
	authStub := &authStub{}
	api := &clientAPIStub{
		sysLeaderFails: true,
		leader:         true,
	}

	client := VaultClient{
		api:             api,
		auth:            nil,
		tokenExpiration: time.Now(),
	}

	err := client.TakeSnapshot(context.Background(), bufio.NewWriter(&bytes.Buffer{}))
	if err == nil {
		t.Fatalf("TakeSnapshot() reported success or returned no error when leader-check failed")
	}

	if api.snapshotTaken {
		t.Fatalf("TakeSnapshot() took snapshot when leader-check failed")
	}

	if client.tokenExpiration == authStub.tokenExpiration {
		t.Fatalf("TakeSnapshot() refresh token-expiration when leader-check failed: client: %v, auth: %v", client.tokenExpiration, authStub.tokenExpiration)
	}
}

func assertAuthRefresh(t *testing.T, refreshed bool, client VaultClient, auth *authStub) {
	t.Helper()

	if auth.refreshed != refreshed {
		if !auth.refreshed {
			t.Fatalf("TakeSnapshot did not call Auth#Refresh() when expected")
		}
		if auth.refreshed {
			t.Fatalf("TakeSnapshot did call Auth#Refresh() unexpectedly")
		}
	}

	if refreshed && client.tokenExpiration != auth.tokenExpiration {
		t.Fatalf("client did not accept tokenExpiration from auth! client: %v, auth: %v", client.tokenExpiration, auth.tokenExpiration)
	}
}

type authStub struct {
	tokenExpiration time.Time
	refreshed       bool
}

func (a *authStub) Refresh(api auth.VaultAuthAPI) (time.Time, error) {
	a.refreshed = true
	var err error
	if a.tokenExpiration.IsZero() {
		err = errors.New("refresh of auth failed")
	}
	return a.tokenExpiration, err
}

type clientAPIStub struct {
	leader          bool
	sysLeaderFails  bool
	snapshotTaken   bool
	snapshotContext context.Context
	snapshotWriter  io.Writer
}

func (stub *clientAPIStub) TakeSnapshot(ctx context.Context, writer io.Writer) error {
	stub.snapshotTaken = true
	stub.snapshotContext = ctx
	stub.snapshotWriter = writer
	return nil
}

func (stub *clientAPIStub) IsLeader() (bool, error) {
	if stub.sysLeaderFails {
		return false, errors.New("leader-Check failed")
	}

	return stub.leader, nil
}

func (stub *clientAPIStub) AuthAPI() auth.VaultAuthAPI {
	return nil
}

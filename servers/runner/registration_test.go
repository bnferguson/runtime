package runner

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"net"
	"testing"

	"miren.dev/runtime/api/runner/runner_v1alpha"
	"miren.dev/runtime/pkg/caauth"
	"miren.dev/runtime/pkg/enrolltoken"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/entity/testutils"
	"miren.dev/runtime/pkg/rpc"
)

type testEnv struct {
	client *runner_v1alpha.RunnerRegistrationClient
	store  *entity.MockStore
}

func newTestServer(t *testing.T) (*testEnv, func()) {
	t.Helper()

	es, cleanup := testutils.NewInMemEntityServer(t)

	ca, err := caauth.New(caauth.Options{
		CommonName:   "test-ca",
		Organization: "test",
	})
	if err != nil {
		cleanup()
		t.Fatalf("failed to create CA: %v", err)
	}

	regServer := NewRegistrationServer(RegistrationServerConfig{
		Log:             testutils.TestLogger(t),
		Authority:       ca,
		EAC:             es.EAC,
		CoordinatorAddr: "127.0.0.1:8443",
	})

	localClient := rpc.LocalClient(runner_v1alpha.AdaptRunnerRegistration(regServer))
	client := runner_v1alpha.NewRunnerRegistrationClient(localClient)

	return &testEnv{client: client, store: es.Store}, cleanup
}

// createInviteAndDecode creates a one-time invite and returns the secret.
func (e *testEnv) createInviteAndDecode(t *testing.T, ctx context.Context) string {
	t.Helper()

	res, err := e.client.CreateInvite(ctx, nil, 1, "", false, 0, "")
	if err != nil {
		t.Fatalf("CreateInvite failed: %v", err)
	}

	token := res.Code()
	if !enrolltoken.IsToken(token) {
		t.Fatalf("CreateInvite returned non-token code: %q", token)
	}

	_, secret, err := enrolltoken.Decode(token)
	if err != nil {
		t.Fatalf("failed to decode token: %v", err)
	}

	return secret
}

// findInviteEntityID finds a runner_invite entity in the mock store by its
// code hash. The entity ID doesn't survive the CBOR round-trip in the local
// RPC client's ListInvites response, so we look it up directly.
func (e *testEnv) findInviteEntityID(t *testing.T, secret string) string {
	t.Helper()
	hash := enrolltoken.Hash(secret)
	for id, ent := range e.store.Entities {
		// Check if this entity is a runner_invite by looking for the code_hash attr
		if attr, ok := ent.Get(runner_v1alpha.RunnerInviteCodeHashId); ok {
			if attr.Value.String() == hash {
				return string(id)
			}
		}
	}
	t.Fatal("invite entity not found in store")
	return ""
}

func TestCreateInviteReturnsToken(t *testing.T) {
	ctx := context.Background()
	env, cleanup := newTestServer(t)
	defer cleanup()

	res, err := env.client.CreateInvite(ctx, nil, 1, "", false, 0, "")
	if err != nil {
		t.Fatalf("CreateInvite failed: %v", err)
	}

	token := res.Code()
	if !enrolltoken.IsToken(token) {
		t.Fatalf("expected mren_ token, got %q", token)
	}

	addr, secret, err := enrolltoken.Decode(token)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if addr != "127.0.0.1:8443" {
		t.Errorf("token addr = %q, want %q", addr, "127.0.0.1:8443")
	}

	if !enrolltoken.IsHexSecret(secret) {
		t.Errorf("token secret is not valid hex: %q", secret)
	}
}

func TestCreateInviteCoordinatorAddrOverride(t *testing.T) {
	ctx := context.Background()
	env, cleanup := newTestServer(t)
	defer cleanup()

	res, err := env.client.CreateInvite(ctx, nil, 1, "", false, 0, "10.0.0.5:8443")
	if err != nil {
		t.Fatalf("CreateInvite failed: %v", err)
	}

	addr, _, err := enrolltoken.Decode(res.Code())
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if addr != "10.0.0.5:8443" {
		t.Errorf("token addr = %q, want overridden %q", addr, "10.0.0.5:8443")
	}
}

func TestJoinCreatesNodeEntity(t *testing.T) {
	ctx := context.Background()
	env, cleanup := newTestServer(t)
	defer cleanup()

	secret := env.createInviteAndDecode(t, ctx)

	joinResult, err := env.client.Join(ctx, secret, "", "10.0.0.1:8443", "test-version", nil, "test-runner")
	if err != nil {
		t.Fatalf("Join RPC failed: %v", err)
	}

	if joinResult.HasError() {
		t.Fatalf("Join returned error: %s", joinResult.Error())
	}

	if joinResult.RunnerId() == "" {
		t.Error("Join did not return a runner ID")
	}

	if len(joinResult.CertPem()) == 0 {
		t.Error("Join did not return a certificate")
	}

	if joinResult.CoordinatorAddr() != "127.0.0.1:8443" {
		t.Errorf("Join returned coordinator addr %q, want %q", joinResult.CoordinatorAddr(), "127.0.0.1:8443")
	}

	// Verify the issued certificate includes proper IP SANs
	block, _ := pem.Decode(joinResult.CertPem())
	if block == nil {
		t.Fatal("failed to decode cert PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse certificate: %v", err)
	}

	wantIPs := []net.IP{
		net.ParseIP("127.0.0.1"),
		net.ParseIP("::1"),
		net.ParseIP("10.0.0.1"),
	}
	for _, wantIP := range wantIPs {
		found := false
		for _, gotIP := range cert.IPAddresses {
			if gotIP.Equal(wantIP) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("certificate missing IP SAN %s, got %v", wantIP, cert.IPAddresses)
		}
	}

	foundLocalhost := false
	for _, name := range cert.DNSNames {
		if name == "localhost" {
			foundLocalhost = true
			break
		}
	}
	if !foundLocalhost {
		t.Errorf("certificate missing DNS SAN 'localhost', got %v", cert.DNSNames)
	}
}

func TestOneTimeInviteConsumedOnUse(t *testing.T) {
	ctx := context.Background()
	env, cleanup := newTestServer(t)
	defer cleanup()

	secret := env.createInviteAndDecode(t, ctx)

	// First join should succeed
	res, err := env.client.Join(ctx, secret, "", "10.0.0.1:8443", "v1", nil, "runner-1")
	if err != nil {
		t.Fatalf("Join failed: %v", err)
	}
	if res.HasError() {
		t.Fatalf("first join failed: %s", res.Error())
	}

	// Second join with same secret should fail (invite consumed)
	res2, err := env.client.Join(ctx, secret, "", "10.0.0.2:8443", "v1", nil, "runner-2")
	if err != nil {
		t.Fatalf("Join RPC failed: %v", err)
	}
	if res2.Error() == "" {
		t.Error("expected error on second join with consumed invite")
	}
}

func TestReusableInviteMultipleJoins(t *testing.T) {
	ctx := context.Background()
	env, cleanup := newTestServer(t)
	defer cleanup()

	res, err := env.client.CreateInvite(ctx, nil, 1, "test-token", true, 0, "")
	if err != nil {
		t.Fatalf("CreateInvite failed: %v", err)
	}

	_, secret, err := enrolltoken.Decode(res.Code())
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Join 3 times, all should succeed
	for i := range 3 {
		joinRes, err := env.client.Join(ctx, secret, "", "10.0.0.1:8443", "v1", nil, "")
		if err != nil {
			t.Fatalf("Join %d failed: %v", i, err)
		}
		if joinRes.HasError() {
			t.Fatalf("Join %d returned error: %s", i, joinRes.Error())
		}
		if joinRes.RunnerId() == "" {
			t.Fatalf("Join %d did not return a runner ID", i)
		}
	}

	// Verify enrollment count via list
	listRes, err := env.client.ListInvites(ctx)
	if err != nil {
		t.Fatalf("ListInvites failed: %v", err)
	}

	invites := listRes.Invites()
	if len(invites) != 1 {
		t.Fatalf("expected 1 invite, got %d", len(invites))
	}

	inv := invites[0]
	if inv.EnrollmentCount() != 3 {
		t.Errorf("enrollment_count = %d, want 3", inv.EnrollmentCount())
	}
	if inv.Name() != "test-token" {
		t.Errorf("name = %q, want %q", inv.Name(), "test-token")
	}
	if !inv.Reusable() {
		t.Error("invite should be marked reusable")
	}
	if inv.Status() != "status.pending" {
		t.Errorf("reusable invite should stay pending, got %q", inv.Status())
	}
}

func TestReusableInviteRevoke(t *testing.T) {
	ctx := context.Background()
	env, cleanup := newTestServer(t)
	defer cleanup()

	res, err := env.client.CreateInvite(ctx, nil, 1, "revoke-me", true, 0, "")
	if err != nil {
		t.Fatalf("CreateInvite failed: %v", err)
	}

	_, secret, err := enrolltoken.Decode(res.Code())
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Join once to confirm it works
	joinRes, err := env.client.Join(ctx, secret, "", "10.0.0.1:8443", "v1", nil, "runner-1")
	if err != nil {
		t.Fatalf("Join failed: %v", err)
	}
	if joinRes.HasError() {
		t.Fatalf("Join returned error: %s", joinRes.Error())
	}

	// Look up the invite entity ID directly from the mock store. The mock
	// store doesn't generate entity IDs like the real etcd store does, so
	// we revoke by directly calling the server's RevokeInvite with the
	// entity key from the store. When the mock store assigns an empty key,
	// we fall back to finding it via ListInvites on the EAC.
	inviteID := env.findInviteEntityID(t, secret)

	// Revoke it
	revokeRes, err := env.client.RevokeInvite(ctx, inviteID)
	if err != nil {
		t.Fatalf("RevokeInvite failed: %v", err)
	}
	if !revokeRes.Success() {
		t.Fatalf("RevokeInvite failed: %s", revokeRes.Error())
	}

	// Subsequent join should fail
	joinRes2, err := env.client.Join(ctx, secret, "", "10.0.0.2:8443", "v1", nil, "runner-2")
	if err != nil {
		t.Fatalf("Join RPC failed: %v", err)
	}
	if joinRes2.Error() == "" {
		t.Error("expected error joining with revoked token")
	}
}

func TestRemoveRunner(t *testing.T) {
	ctx := context.Background()
	env, cleanup := newTestServer(t)
	defer cleanup()

	secret := env.createInviteAndDecode(t, ctx)

	joinResult, err := env.client.Join(ctx, secret, "", "10.0.0.1:8443", "test-version", nil, "test-runner")
	if err != nil {
		t.Fatalf("Join failed: %v", err)
	}
	if joinResult.HasError() {
		t.Fatalf("Join returned error: %s", joinResult.Error())
	}

	listResult, err := env.client.ListRunners(ctx)
	if err != nil {
		t.Fatalf("ListRunners failed: %v", err)
	}
	if len(listResult.Runners()) != 1 {
		t.Fatalf("expected 1 runner, got %d", len(listResult.Runners()))
	}

	removeResult, err := env.client.RemoveRunner(ctx, "test-runner", false)
	if err != nil {
		t.Fatalf("RemoveRunner failed: %v", err)
	}
	if removeResult.Error() != "" {
		t.Fatalf("RemoveRunner returned error: %s", removeResult.Error())
	}
	if removeResult.Name() != "test-runner" {
		t.Errorf("RemoveRunner returned name %q, want %q", removeResult.Name(), "test-runner")
	}
	if removeResult.RunnerId() != joinResult.RunnerId() {
		t.Errorf("RemoveRunner returned runner_id %q, want %q", removeResult.RunnerId(), joinResult.RunnerId())
	}

	listResult, err = env.client.ListRunners(ctx)
	if err != nil {
		t.Fatalf("ListRunners after remove failed: %v", err)
	}
	if len(listResult.Runners()) != 0 {
		t.Errorf("expected 0 runners after remove, got %d", len(listResult.Runners()))
	}
}

func TestRemoveRunnerNotFound(t *testing.T) {
	ctx := context.Background()
	env, cleanup := newTestServer(t)
	defer cleanup()

	removeResult, err := env.client.RemoveRunner(ctx, "nonexistent", false)
	if err != nil {
		t.Fatalf("RemoveRunner failed: %v", err)
	}
	if removeResult.Error() == "" {
		t.Error("expected error for nonexistent runner, got none")
	}
}

func TestRemoveRunnerByShortId(t *testing.T) {
	ctx := context.Background()
	env, cleanup := newTestServer(t)
	defer cleanup()

	secret := env.createInviteAndDecode(t, ctx)

	joinResult, err := env.client.Join(ctx, secret, "", "10.0.0.3:8443", "v1", nil, "runner-short")
	if err != nil {
		t.Fatalf("Join failed: %v", err)
	}
	if joinResult.HasError() {
		t.Fatalf("Join returned error: %s", joinResult.Error())
	}

	listResult, err := env.client.ListRunners(ctx)
	if err != nil {
		t.Fatalf("ListRunners failed: %v", err)
	}
	if len(listResult.Runners()) != 1 {
		t.Fatalf("expected 1 runner, got %d", len(listResult.Runners()))
	}
	shortId := listResult.Runners()[0].ShortId()
	if shortId == "" {
		t.Fatalf("expected runner to have a short id assigned")
	}

	removeResult, err := env.client.RemoveRunner(ctx, shortId, false)
	if err != nil {
		t.Fatalf("RemoveRunner failed: %v", err)
	}
	if removeResult.Error() != "" {
		t.Fatalf("RemoveRunner returned error: %s", removeResult.Error())
	}
	if removeResult.Name() != "runner-short" {
		t.Errorf("RemoveRunner returned name %q, want %q", removeResult.Name(), "runner-short")
	}
}

func TestRemoveRunnerByRunnerId(t *testing.T) {
	ctx := context.Background()
	env, cleanup := newTestServer(t)
	defer cleanup()

	secret := env.createInviteAndDecode(t, ctx)

	joinResult, err := env.client.Join(ctx, secret, "", "10.0.0.2:8443", "v1", nil, "runner-two")
	if err != nil {
		t.Fatalf("Join failed: %v", err)
	}
	if joinResult.HasError() {
		t.Fatalf("Join returned error: %s", joinResult.Error())
	}

	removeResult, err := env.client.RemoveRunner(ctx, joinResult.RunnerId(), false)
	if err != nil {
		t.Fatalf("RemoveRunner failed: %v", err)
	}
	if removeResult.Error() != "" {
		t.Fatalf("RemoveRunner returned error: %s", removeResult.Error())
	}
	if removeResult.Name() != "runner-two" {
		t.Errorf("RemoveRunner returned name %q, want %q", removeResult.Name(), "runner-two")
	}

	listResult, err := env.client.ListRunners(ctx)
	if err != nil {
		t.Fatalf("ListRunners failed: %v", err)
	}
	if len(listResult.Runners()) != 0 {
		t.Errorf("expected 0 runners, got %d", len(listResult.Runners()))
	}
}

package runner

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"net"
	"testing"
	"time"

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
	server *RegistrationServer
	ca     *caauth.Authority
}

func newTestServer(t *testing.T) (*testEnv, func()) {
	t.Helper()

	es, cleanup := testutils.NewInMemEntityServer(t)

	ca, err := caauth.New(caauth.Options{
		CommonName:   "test-ca",
		Organization: "test",
		ValidFor:     24 * time.Hour,
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

	return &testEnv{client: client, store: es.Store, server: regServer, ca: ca}, cleanup
}

// issueLeafCert issues a certificate from the given authority and returns the
// parsed leaf certificate (the first PEM block; IssueCertificate appends the CA
// cert after it).
func issueLeafCert(t *testing.T, ca *caauth.Authority, commonName, org string, ip string) *x509.Certificate {
	t.Helper()

	opts := caauth.Options{
		CommonName:   commonName,
		Organization: org,
		ValidFor:     time.Hour,
	}
	if ip != "" {
		opts.IPs = []net.IP{net.ParseIP(ip)}
	}

	cc, err := ca.IssueCertificate(opts)
	if err != nil {
		t.Fatalf("failed to issue cert: %v", err)
	}

	block, _ := pem.Decode(cc.CertPEM)
	if block == nil {
		t.Fatal("failed to decode issued cert PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse issued cert: %v", err)
	}
	return cert
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

func TestBuildRunnerSANs(t *testing.T) {
	t.Run("IP listen address adds IP SAN", func(t *testing.T) {
		ips, dnsNames := buildRunnerSANs("10.0.0.7:8444")

		wantIPs := []string{"127.0.0.1", "::1", "10.0.0.7"}
		for _, want := range wantIPs {
			if !containsIP(ips, want) {
				t.Errorf("missing IP SAN %s, got %v", want, ips)
			}
		}
		if !containsStr(dnsNames, "localhost") {
			t.Errorf("missing DNS SAN localhost, got %v", dnsNames)
		}
		if containsStr(dnsNames, "10.0.0.7") {
			t.Errorf("IP should not appear as a DNS SAN, got %v", dnsNames)
		}
	})

	t.Run("hostname listen address adds DNS SAN", func(t *testing.T) {
		ips, dnsNames := buildRunnerSANs("runner.example.com:8444")

		if !containsStr(dnsNames, "runner.example.com") {
			t.Errorf("missing DNS SAN runner.example.com, got %v", dnsNames)
		}
		if !containsIP(ips, "127.0.0.1") || !containsIP(ips, "::1") {
			t.Errorf("missing loopback IP SANs, got %v", ips)
		}
	})

	t.Run("empty listen address yields only loopback", func(t *testing.T) {
		ips, dnsNames := buildRunnerSANs("")
		if len(ips) != 2 {
			t.Errorf("expected only loopback IPs, got %v", ips)
		}
		if len(dnsNames) != 1 || dnsNames[0] != "localhost" {
			t.Errorf("expected only localhost DNS, got %v", dnsNames)
		}
	})
}

// joinRunner performs a Join and returns the issued leaf certificate and the
// assigned runner ID, so tests can exercise refresh as a real, registered runner.
func (e *testEnv) joinRunner(t *testing.T, ctx context.Context, listenAddr, name string) (*x509.Certificate, string) {
	t.Helper()

	secret := e.createInviteAndDecode(t, ctx)
	res, err := e.client.Join(ctx, secret, "", listenAddr, "v1", nil, name)
	if err != nil {
		t.Fatalf("Join failed: %v", err)
	}
	if res.HasError() {
		t.Fatalf("Join returned error: %s", res.Error())
	}

	block, _ := pem.Decode(res.CertPem())
	if block == nil {
		t.Fatal("failed to decode join cert PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse join cert: %v", err)
	}
	return cert, res.RunnerId()
}

func TestReissueRunnerCertificate(t *testing.T) {
	ctx := context.Background()
	env, cleanup := newTestServer(t)
	defer cleanup()

	t.Run("happy path re-issues with new IP and preserves CN", func(t *testing.T) {
		peer, _ := env.joinRunner(t, ctx, "10.0.0.1:8444", "happy-runner")

		cc, err := env.server.reissueRunnerCertificate(ctx, peer, "10.0.0.2:8444")
		if err != nil {
			t.Fatalf("reissueRunnerCertificate failed: %v", err)
		}

		// The re-issued cert must be signed by the cluster CA.
		if err := env.ca.VerifyCertificate(cc.CertPEM); err != nil {
			t.Fatalf("re-issued cert not signed by CA: %v", err)
		}

		block, _ := pem.Decode(cc.CertPEM)
		if block == nil {
			t.Fatal("failed to decode re-issued cert PEM")
		}
		newCert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			t.Fatalf("failed to parse re-issued cert: %v", err)
		}

		if newCert.Subject.CommonName != peer.Subject.CommonName {
			t.Errorf("CommonName = %q, want preserved %q", newCert.Subject.CommonName, peer.Subject.CommonName)
		}

		var foundIPs []string
		for _, ip := range newCert.IPAddresses {
			foundIPs = append(foundIPs, ip.String())
		}
		if !containsIP(newCert.IPAddresses, "10.0.0.2") {
			t.Errorf("re-issued cert missing new IP SAN 10.0.0.2, got %v", foundIPs)
		}
	})

	t.Run("nil peer is rejected", func(t *testing.T) {
		_, err := env.server.reissueRunnerCertificate(ctx, nil, "10.0.0.2:8444")
		if err == nil {
			t.Fatal("expected error for nil peer certificate")
		}
	})

	t.Run("cert from another CA is rejected", func(t *testing.T) {
		otherCA, err := caauth.New(caauth.Options{CommonName: "other-ca", Organization: "other", ValidFor: 24 * time.Hour})
		if err != nil {
			t.Fatalf("failed to create other CA: %v", err)
		}
		peer := issueLeafCert(t, otherCA, "runner-abc12345", "miren", "10.0.0.1")

		_, err = env.server.reissueRunnerCertificate(ctx, peer, "10.0.0.2:8444")
		if err == nil {
			t.Fatal("expected error for cert signed by a different CA")
		}
	})

	t.Run("non-runner CN is rejected", func(t *testing.T) {
		peer := issueLeafCert(t, env.ca, "operator-abc", "miren", "10.0.0.1")

		_, err := env.server.reissueRunnerCertificate(ctx, peer, "10.0.0.2:8444")
		if err == nil {
			t.Fatal("expected error for non-runner CommonName")
		}
	})

	t.Run("wrong organization is rejected", func(t *testing.T) {
		peer := issueLeafCert(t, env.ca, "runner-abc12345", "intruder", "10.0.0.1")

		_, err := env.server.reissueRunnerCertificate(ctx, peer, "10.0.0.2:8444")
		if err == nil {
			t.Fatal("expected error for cert with wrong organization")
		}
	})

	t.Run("unregistered runner cert is rejected", func(t *testing.T) {
		// A genuine CA-signed runner cert, but no matching Node exists (e.g. the
		// runner was never registered or has been removed). The identity comes
		// from the cert, so a caller cannot substitute another runner's ID.
		peer := issueLeafCert(t, env.ca, "runner-deadbeef", "miren", "10.0.0.1")

		_, err := env.server.reissueRunnerCertificate(ctx, peer, "10.0.0.2:8444")
		if err == nil {
			t.Fatal("expected error for a runner cert with no registered node")
		}
	})

	t.Run("removed runner cannot refresh", func(t *testing.T) {
		peer, runnerID := env.joinRunner(t, ctx, "10.0.0.1:8444", "removed-runner")

		// Remove the runner, deleting its Node entity.
		removeRes, err := env.client.RemoveRunner(ctx, runnerID, false)
		if err != nil {
			t.Fatalf("RemoveRunner failed: %v", err)
		}
		if removeRes.Error() != "" {
			t.Fatalf("RemoveRunner returned error: %s", removeRes.Error())
		}

		// Its certificate is still cryptographically valid, but it is no longer
		// registered, so refresh must be rejected.
		_, err = env.server.reissueRunnerCertificate(ctx, peer, "10.0.0.2:8444")
		if err == nil {
			t.Fatal("expected error refreshing a removed runner's certificate")
		}
	})

}

func TestRefreshCertificateRequiresClientCert(t *testing.T) {
	ctx := context.Background()
	env, cleanup := newTestServer(t)
	defer cleanup()

	// The local RPC client does not carry a TLS peer certificate, so the
	// handler must reject the request rather than minting a cert.
	res, err := env.client.RefreshCertificate(ctx, "10.0.0.2:8444")
	if err != nil {
		t.Fatalf("RefreshCertificate RPC failed: %v", err)
	}
	if res.Error() == "" {
		t.Fatal("expected RefreshCertificate to reject a call without a client certificate")
	}
	if len(res.CertPem()) != 0 {
		t.Error("expected no certificate when the call is rejected")
	}
}

func containsIP(ips []net.IP, want string) bool {
	w := net.ParseIP(want)
	for _, ip := range ips {
		if ip.Equal(w) {
			return true
		}
	}
	return false
}

func containsStr(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

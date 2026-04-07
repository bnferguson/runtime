package runner

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"net"
	"testing"

	"miren.dev/runtime/api/runner/runner_v1alpha"
	"miren.dev/runtime/pkg/caauth"
	"miren.dev/runtime/pkg/entity/testutils"
	"miren.dev/runtime/pkg/joincode"
	"miren.dev/runtime/pkg/rpc"
)

func TestJoinCodeIntegration(t *testing.T) {
	code, err := joincode.Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if !joincode.Validate(code) {
		t.Errorf("Generated code %q did not validate", code)
	}

	hash := joincode.Hash(code)
	if hash == "" {
		t.Error("Hash() returned empty string")
	}

	if len(hash) != 64 {
		t.Errorf("Hash() returned string of length %d, expected 64", len(hash))
	}
}

func TestJoinCreatesNodeEntity(t *testing.T) {
	ctx := context.Background()

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	ca, err := caauth.New(caauth.Options{
		CommonName:   "test-ca",
		Organization: "test",
	})
	if err != nil {
		t.Fatalf("failed to create CA: %v", err)
	}

	regServer := NewRegistrationServer(
		testutils.TestLogger(t),
		ca,
		es.EAC,
		"127.0.0.1:8443",
		nil, "", "",
	)

	localClient := rpc.LocalClient(runner_v1alpha.AdaptRunnerRegistration(regServer))
	client := runner_v1alpha.NewRunnerRegistrationClient(localClient)

	// Create an invite
	inviteResult, err := client.CreateInvite(ctx, nil, 1)
	if err != nil {
		t.Fatalf("CreateInvite failed: %v", err)
	}

	code := inviteResult.Code()
	if code == "" {
		t.Fatal("CreateInvite returned empty code")
	}

	// Join using the invite code — this was failing before the fix because
	// the node entity included a session attribute (status) without a session ID.
	joinResult, err := client.Join(ctx, code, "", "10.0.0.1:8443", "test-version", nil, "test-runner")
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

	// Verify the issued certificate includes proper IP SANs so the
	// coordinator can connect to the runner by IP without TLS errors.
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

func TestRemoveRunner(t *testing.T) {
	ctx := context.Background()

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	ca, err := caauth.New(caauth.Options{
		CommonName:   "test-ca",
		Organization: "test",
	})
	if err != nil {
		t.Fatalf("failed to create CA: %v", err)
	}

	regServer := NewRegistrationServer(
		testutils.TestLogger(t),
		ca,
		es.EAC,
		"127.0.0.1:8443",
		nil, "", "",
	)

	localClient := rpc.LocalClient(runner_v1alpha.AdaptRunnerRegistration(regServer))
	client := runner_v1alpha.NewRunnerRegistrationClient(localClient)

	// Create and join a runner
	inviteResult, err := client.CreateInvite(ctx, nil, 1)
	if err != nil {
		t.Fatalf("CreateInvite failed: %v", err)
	}

	joinResult, err := client.Join(ctx, inviteResult.Code(), "", "10.0.0.1:8443", "test-version", nil, "test-runner")
	if err != nil {
		t.Fatalf("Join failed: %v", err)
	}
	if joinResult.HasError() {
		t.Fatalf("Join returned error: %s", joinResult.Error())
	}

	// Verify the runner shows up in the list
	listResult, err := client.ListRunners(ctx)
	if err != nil {
		t.Fatalf("ListRunners failed: %v", err)
	}
	if len(listResult.Runners()) != 1 {
		t.Fatalf("expected 1 runner, got %d", len(listResult.Runners()))
	}

	// Remove the runner by name
	removeResult, err := client.RemoveRunner(ctx, "test-runner", false)
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

	// Verify the runner is gone
	listResult, err = client.ListRunners(ctx)
	if err != nil {
		t.Fatalf("ListRunners after remove failed: %v", err)
	}
	if len(listResult.Runners()) != 0 {
		t.Errorf("expected 0 runners after remove, got %d", len(listResult.Runners()))
	}
}

func TestRemoveRunnerNotFound(t *testing.T) {
	ctx := context.Background()

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	ca, err := caauth.New(caauth.Options{
		CommonName:   "test-ca",
		Organization: "test",
	})
	if err != nil {
		t.Fatalf("failed to create CA: %v", err)
	}

	regServer := NewRegistrationServer(
		testutils.TestLogger(t),
		ca,
		es.EAC,
		"127.0.0.1:8443",
		nil, "", "",
	)

	localClient := rpc.LocalClient(runner_v1alpha.AdaptRunnerRegistration(regServer))
	client := runner_v1alpha.NewRunnerRegistrationClient(localClient)

	removeResult, err := client.RemoveRunner(ctx, "nonexistent", false)
	if err != nil {
		t.Fatalf("RemoveRunner failed: %v", err)
	}
	if removeResult.Error() == "" {
		t.Error("expected error for nonexistent runner, got none")
	}
}

func TestRemoveRunnerByRunnerId(t *testing.T) {
	ctx := context.Background()

	es, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	ca, err := caauth.New(caauth.Options{
		CommonName:   "test-ca",
		Organization: "test",
	})
	if err != nil {
		t.Fatalf("failed to create CA: %v", err)
	}

	regServer := NewRegistrationServer(
		testutils.TestLogger(t),
		ca,
		es.EAC,
		"127.0.0.1:8443",
		nil, "", "",
	)

	localClient := rpc.LocalClient(runner_v1alpha.AdaptRunnerRegistration(regServer))
	client := runner_v1alpha.NewRunnerRegistrationClient(localClient)

	// Create and join a runner
	inviteResult, err := client.CreateInvite(ctx, nil, 1)
	if err != nil {
		t.Fatalf("CreateInvite failed: %v", err)
	}

	joinResult, err := client.Join(ctx, inviteResult.Code(), "", "10.0.0.2:8443", "v1", nil, "runner-two")
	if err != nil {
		t.Fatalf("Join failed: %v", err)
	}
	if joinResult.HasError() {
		t.Fatalf("Join returned error: %s", joinResult.Error())
	}

	// Remove by runner ID
	removeResult, err := client.RemoveRunner(ctx, joinResult.RunnerId(), false)
	if err != nil {
		t.Fatalf("RemoveRunner failed: %v", err)
	}
	if removeResult.Error() != "" {
		t.Fatalf("RemoveRunner returned error: %s", removeResult.Error())
	}
	if removeResult.Name() != "runner-two" {
		t.Errorf("RemoveRunner returned name %q, want %q", removeResult.Name(), "runner-two")
	}

	// Verify gone
	listResult, err := client.ListRunners(ctx)
	if err != nil {
		t.Fatalf("ListRunners failed: %v", err)
	}
	if len(listResult.Runners()) != 0 {
		t.Errorf("expected 0 runners, got %d", len(listResult.Runners()))
	}
}

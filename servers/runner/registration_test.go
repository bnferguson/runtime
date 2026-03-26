package runner

import (
	"context"
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
	joinResult, err := client.Join(ctx, code, "", "10.0.0.1:8443", "test-version", nil)
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
}

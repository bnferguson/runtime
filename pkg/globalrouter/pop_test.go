package globalrouter

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"
)

func TestPOPManagerHandleConnectionRequest(t *testing.T) {
	pm := NewPOPManager("test-cluster", nil, slog.Default())
	defer pm.Close()

	req := ConnectionRequest{
		POPXID:     "pop-1",
		POPAddress: "https://pop1.example.com:443",
		Hostname:   "myapp.example.com",
		RequestID:  "req-1",
	}

	if err := pm.HandleConnectionRequest(t.Context(), req); err != nil {
		t.Fatalf("HandleConnectionRequest: %v", err)
	}

	pops := pm.ConnectedPOPs()
	if len(pops) != 1 || pops[0] != "pop-1" {
		t.Errorf("expected [pop-1], got %v", pops)
	}

	// Duplicate request replaces the existing connection
	if err := pm.HandleConnectionRequest(t.Context(), req); err != nil {
		t.Fatalf("duplicate HandleConnectionRequest: %v", err)
	}

	pops = pm.ConnectedPOPs()
	if len(pops) != 1 {
		t.Errorf("expected 1 POP after duplicate, got %d", len(pops))
	}
}

func TestPOPManagerRemovePOP(t *testing.T) {
	pm := NewPOPManager("test-cluster", nil, slog.Default())
	defer pm.Close()

	req := ConnectionRequest{
		POPXID:     "pop-1",
		POPAddress: "https://pop1.example.com:443",
		Hostname:   "myapp.example.com",
		RequestID:  "req-1",
	}

	pm.HandleConnectionRequest(t.Context(), req)
	pm.RemovePOP("pop-1")

	// Give the goroutine time to clean up
	pops := pm.ConnectedPOPs()

	// After RemovePOP the entry is removed immediately from the map
	if len(pops) != 0 {
		t.Errorf("expected 0 POPs after remove, got %d", len(pops))
	}
}

func TestPOPManagerClose(t *testing.T) {
	pm := NewPOPManager("test-cluster", nil, slog.Default())

	for i := range 3 {
		req := ConnectionRequest{
			POPXID:     "pop-" + string(rune('a'+i)),
			POPAddress: "https://pop.example.com:443",
			Hostname:   "app.example.com",
			RequestID:  "req",
		}
		pm.HandleConnectionRequest(t.Context(), req)
	}

	pm.Close()

	pops := pm.ConnectedPOPs()
	if len(pops) != 0 {
		t.Errorf("expected 0 POPs after Close, got %d", len(pops))
	}
}

func TestPOPManagerReplaceStaleConnection(t *testing.T) {
	pm := NewPOPManager("test-cluster", nil, slog.Default())
	defer pm.Close()

	req1 := ConnectionRequest{
		POPXID:     "pop-1",
		POPAddress: "https://pop1.example.com:443",
		Hostname:   "myapp.example.com",
		RequestID:  "req-1",
	}

	if err := pm.HandleConnectionRequest(t.Context(), req1); err != nil {
		t.Fatalf("first HandleConnectionRequest: %v", err)
	}

	pops := pm.ConnectedPOPs()
	if len(pops) != 1 || pops[0] != "pop-1" {
		t.Fatalf("expected [pop-1], got %v", pops)
	}

	// Send a second connection request for the same POPXID (simulates
	// a POP VM recreate). The old entry should be replaced, not ignored.
	req2 := ConnectionRequest{
		POPXID:       "pop-1",
		POPAddress:   "https://pop1.example.com:443",
		Hostname:     "myapp.example.com",
		RequestID:    "req-2",
		ConnectToken: "new-token",
	}

	if err := pm.HandleConnectionRequest(t.Context(), req2); err != nil {
		t.Fatalf("replacement HandleConnectionRequest: %v", err)
	}

	// Immediately after replacement, should still have exactly one entry.
	pops = pm.ConnectedPOPs()
	if len(pops) != 1 || pops[0] != "pop-1" {
		t.Errorf("expected [pop-1] after replacement, got %v", pops)
	}

	// Wait for both servePOP goroutines to finish (they fail on DNS
	// resolution of the fake address). The guarded defer in the old
	// goroutine must not delete the replacement entry — only the new
	// goroutine's defer should clean up its own entry.
	time.Sleep(200 * time.Millisecond)

	// Both goroutines exited: old one was cancelled, new one failed DNS.
	// Map should be empty since the new goroutine legitimately cleaned
	// up after itself. The key invariant is that we never see 2 entries
	// and the replacement was served (not short-circuited).
	pops = pm.ConnectedPOPs()
	if len(pops) > 1 {
		t.Errorf("expected at most 1 POP after goroutine cleanup, got %v", pops)
	}
}

func TestEnvelopeRoundTrip(t *testing.T) {
	router := NewMessageRouter()

	var dispatched bool
	router.Handle("test.msg", func(_ context.Context, data json.RawMessage) error {
		dispatched = true
		return nil
	})

	env := Envelope{
		Type: "test.msg",
		Data: []byte(`{}`),
	}

	if err := router.Dispatch(t.Context(), env); err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	if !dispatched {
		t.Error("handler was not called")
	}
}

func TestMessageRouterUnknownType(t *testing.T) {
	router := NewMessageRouter()

	env := Envelope{Type: "unknown.type", Data: []byte(`{}`)}
	err := router.Dispatch(t.Context(), env)
	if err == nil {
		t.Error("expected error for unknown type")
	}
}

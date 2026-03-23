package globalrouter

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
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

	// Duplicate request should be a no-op
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

package cloudauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNetcheck(t *testing.T) {
	t.Run("success with reachable ports", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if r.URL.Path != "/api/v1/netcheck" {
				t.Errorf("expected /api/v1/netcheck, got %s", r.URL.Path)
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("expected application/json content type, got %s", r.Header.Get("Content-Type"))
			}

			var req NetcheckRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("failed to decode request: %v", err)
			}
			if len(req.Ports) != 2 {
				t.Fatalf("expected 2 ports, got %d", len(req.Ports))
			}

			resp := NetcheckResponse{
				SourceAddress: "203.0.113.10",
				DurationMs:    42,
				Results: []NetcheckResult{
					{Port: 8443, Protocol: "https", Reachable: true, LatencyMs: 15},
					{Port: 8443, Protocol: "http3", Reachable: false, Error: "timeout"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		result, err := Netcheck(context.Background(), srv.URL, []NetcheckPort{
			{Port: 8443, Protocol: "https"},
			{Port: 8443, Protocol: "http3"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.SourceAddress != "203.0.113.10" {
			t.Errorf("expected source address 203.0.113.10, got %s", result.SourceAddress)
		}
		if result.DurationMs != 42 {
			t.Errorf("expected duration 42ms, got %d", result.DurationMs)
		}
		if len(result.Results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(result.Results))
		}
		if !result.Results[0].Reachable {
			t.Error("expected first port to be reachable")
		}
		if result.Results[1].Reachable {
			t.Error("expected second port to be unreachable")
		}
	})

	t.Run("private address returns sentinel error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "client IP is not a public address"})
		}))
		defer srv.Close()

		_, err := Netcheck(context.Background(), srv.URL, []NetcheckPort{
			{Port: 8443, Protocol: "https"},
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err != ErrPrivateAddress {
			t.Errorf("expected ErrPrivateAddress, got %v", err)
		}
	})

	t.Run("server error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal error"))
		}))
		defer srv.Close()

		_, err := Netcheck(context.Background(), srv.URL, []NetcheckPort{
			{Port: 8443, Protocol: "https"},
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

package ipdiscovery

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"miren.dev/runtime/pkg/cloudauth"
	"miren.dev/runtime/pkg/slogfmt"
)

// testLogger returns a logger suitable for tests
func testLogger(t *testing.T) *slog.Logger {
	return slog.New(slogfmt.NewTestHandler(t, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestDiscover(t *testing.T) {
	ctx := context.Background()
	log := testLogger(t)
	discovery, err := Discover(ctx, log, Options{})
	require.NoError(t, err)
	require.NotNil(t, discovery)

	// Should have at least one address (at minimum loopback)
	assert.NotEmpty(t, discovery.Addresses)

	// Check that addresses have required fields
	for _, addr := range discovery.Addresses {
		assert.NotEmpty(t, addr.Interface)
		assert.NotEmpty(t, addr.IP)
		assert.NotEmpty(t, addr.Network)
	}
}

func TestDiscoverWithTimeout(t *testing.T) {
	log := testLogger(t)
	discovery, err := DiscoverWithTimeout(5*time.Second, log, Options{})
	require.NoError(t, err)
	require.NotNil(t, discovery)

	// Should have addresses
	assert.NotEmpty(t, discovery.Addresses)
}

func TestAddressTypes(t *testing.T) {
	ctx := context.Background()
	log := testLogger(t)
	discovery, err := Discover(ctx, log, Options{})
	require.NoError(t, err)

	var hasIPv4 bool
	for _, addr := range discovery.Addresses {
		ip := net.ParseIP(addr.IP)
		require.NotNil(t, ip)

		if ip.To4() != nil {
			hasIPv4 = true
			assert.False(t, addr.IsIPv6)
		} else {
			assert.True(t, addr.IsIPv6)
		}
	}

	// Most systems should have at least IPv4
	assert.True(t, hasIPv4)
}

func fakeNetcheckServer(t *testing.T, resp cloudauth.NetcheckResponse) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/v1/netcheck" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestDiscoverWithNetcheck(t *testing.T) {
	srv := fakeNetcheckServer(t, cloudauth.NetcheckResponse{
		SourceAddress: "203.0.113.42",
	})

	log := testLogger(t)
	discovery, err := Discover(context.Background(), log, Options{
		NetcheckURL: srv.URL,
	})
	require.NoError(t, err)

	var found bool
	for _, addr := range discovery.Addresses {
		if addr.IP == "203.0.113.42" {
			found = true
			assert.Equal(t, "netcheck", addr.Interface)
			assert.False(t, addr.IsIPv6)
			break
		}
	}
	assert.True(t, found, "expected netcheck-discovered public IP in addresses")
}

func TestDiscoverWithNetcheckIPv6(t *testing.T) {
	srv := fakeNetcheckServer(t, cloudauth.NetcheckResponse{
		SourceAddress: "2001:db8::1",
	})

	log := testLogger(t)
	discovery, err := Discover(context.Background(), log, Options{
		NetcheckURL: srv.URL,
	})
	require.NoError(t, err)

	var found bool
	for _, addr := range discovery.Addresses {
		if addr.IP == "2001:db8::1" {
			found = true
			assert.Equal(t, "netcheck", addr.Interface)
			assert.True(t, addr.IsIPv6)
			break
		}
	}
	assert.True(t, found, "expected netcheck-discovered IPv6 address in addresses")
}

func TestDiscoverWithNetcheckPrivateIPFiltered(t *testing.T) {
	srv := fakeNetcheckServer(t, cloudauth.NetcheckResponse{
		SourceAddress: "10.0.0.1",
	})

	log := testLogger(t)
	discovery, err := Discover(context.Background(), log, Options{
		NetcheckURL: srv.URL,
	})
	require.NoError(t, err)

	for _, addr := range discovery.Addresses {
		if addr.Interface == "netcheck" {
			t.Errorf("private IP %s should not appear as netcheck address", addr.IP)
		}
	}
}

func TestDiscoverWithNetcheckFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	log := testLogger(t)
	discovery, err := Discover(context.Background(), log, Options{
		NetcheckURL: srv.URL,
	})
	require.NoError(t, err)
	// Should still return local addresses even when netcheck fails
	assert.NotEmpty(t, discovery.Addresses)
	for _, addr := range discovery.Addresses {
		assert.NotEqual(t, "netcheck", addr.Interface)
	}
}

func TestDiscoveryJSON(t *testing.T) {
	// Test that Discovery can be properly marshaled to JSON
	discovery := &Discovery{
		Addresses: []Address{
			{
				Interface: "eth0",
				IP:        "192.168.1.100",
				Network:   "192.168.1.0/24",
				IsIPv6:    false,
			},
			{
				Interface: "eth0",
				IP:        "2001:db8::1",
				Network:   "2001:db8::/64",
				IsIPv6:    true,
			},
		},
	}

	data, err := json.Marshal(discovery)
	require.NoError(t, err)

	var decoded Discovery
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, len(discovery.Addresses), len(decoded.Addresses))
}

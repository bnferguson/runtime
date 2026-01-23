package dns

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"

	compute_v1alpha "miren.dev/runtime/api/compute/compute_v1alpha"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/slogfmt"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	log := slog.New(slogfmt.NewTestHandler(t, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return &Server{
		log:             log,
		ipToApp:         make(map[string]string),
		ipToService:     make(map[string]string),
		appServiceToIPs: make(map[string]map[string][]string),
		entityToIP:      make(map[string]string),
	}
}

func TestIPReuseBetweenSandboxes(t *testing.T) {
	// This test covers a race condition where:
	// 1. Sandbox A is created at IP X
	// 2. Sandbox A dies, but entity cleanup is delayed (1 hour)
	// 3. IP X is reused by new Sandbox B (after 30-min cooldown)
	// 4. Sandbox A's entity is finally deleted
	// 5. The IP should still be resolvable because Sandbox B is using it

	s := newTestServer(t)

	const (
		sharedIP     = "10.8.24.19"
		appName      = "testapp"
		serviceName  = "web"
		oldSandboxID = "sandbox/testapp-web-OLD"
		newSandboxID = "sandbox/testapp-web-NEW"
	)

	// Old sandbox (now STOPPED) was registered at IP
	s.addSandboxMapping(oldSandboxID, sharedIP, appName, serviceName)

	// New sandbox (RUNNING) gets the same IP after cooldown
	s.addSandboxMapping(newSandboxID, sharedIP, appName, serviceName)

	// Old sandbox entity is finally deleted (1 hour cleanup delay)
	s.handleSandboxDeleteByID(oldSandboxID)

	// The IP should still resolve because new sandbox is using it
	assert.Equal(t, appName, s.lookupAppForIP(sharedIP),
		"IP should still resolve after old sandbox deletion because new sandbox is using it")
}

func TestDeleteSandboxCleansUpWhenNoReuse(t *testing.T) {
	// When a sandbox is deleted and no other entity uses the IP,
	// the mappings should be cleaned up.

	s := newTestServer(t)

	const (
		ip        = "10.8.24.20"
		appName   = "testapp"
		service   = "web"
		sandboxID = "sandbox/testapp-web-123"
	)

	s.addSandboxMapping(sandboxID, ip, appName, service)
	assert.Equal(t, appName, s.lookupAppForIP(ip))

	s.handleSandboxDeleteByID(sandboxID)

	assert.Empty(t, s.lookupAppForIP(ip),
		"IP should not resolve after sandbox deletion when no other sandbox uses it")
}

func TestDeleteSandboxWithDifferentIPs(t *testing.T) {
	// Deleting one sandbox should not affect another sandbox with a different IP.

	s := newTestServer(t)

	const (
		ip1        = "10.8.24.20"
		ip2        = "10.8.24.21"
		appName    = "testapp"
		service    = "web"
		sandbox1ID = "sandbox/testapp-web-1"
		sandbox2ID = "sandbox/testapp-web-2"
	)

	s.addSandboxMapping(sandbox1ID, ip1, appName, service)
	s.addSandboxMapping(sandbox2ID, ip2, appName, service)

	s.handleSandboxDeleteByID(sandbox1ID)

	assert.Empty(t, s.lookupAppForIP(ip1), "ip1 should not resolve after deletion")
	assert.Equal(t, appName, s.lookupAppForIP(ip2), "ip2 should still resolve")
}

func TestSandboxStatusTransitionRemovesFromDNS(t *testing.T) {
	// This test verifies that when a sandbox transitions from RUNNING to
	// STOPPED or DEAD, it is immediately removed from DNS mappings.
	// Previously, UPDATE events for non-RUNNING sandboxes were ignored,
	// causing stale IPs to remain in DNS until the entity DELETE (up to 1 hour).

	s := newTestServer(t)
	ctx := context.Background()

	const (
		ip        = "10.8.24.30"
		appName   = "testapp"
		service   = "web"
		sandboxID = "sandbox/testapp-web-status-test"
	)

	// Simulate a sandbox that was previously RUNNING and tracked
	s.addSandboxMapping(sandboxID, ip, appName, service)
	assert.Equal(t, appName, s.lookupAppForIP(ip), "sandbox should be tracked initially")

	// Sandbox transitions to STOPPED (e.g., process exited)
	stoppedSandbox := &compute_v1alpha.Sandbox{
		ID:     entity.Id(sandboxID),
		Status: compute_v1alpha.STOPPED,
	}
	s.handleSandboxUpdate(ctx, stoppedSandbox, nil)

	// The sandbox should be removed from DNS immediately
	assert.Empty(t, s.lookupAppForIP(ip),
		"sandbox should be removed from DNS when status changes to STOPPED")

	// Verify all mappings are cleaned up
	s.mu.RLock()
	_, hasEntityMapping := s.entityToIP[sandboxID]
	_, hasServiceMapping := s.ipToService[ip]
	s.mu.RUnlock()

	assert.False(t, hasEntityMapping, "entityToIP mapping should be removed")
	assert.False(t, hasServiceMapping, "ipToService mapping should be removed")
}

func TestSandboxDeadStatusRemovesFromDNS(t *testing.T) {
	// Same as above but for DEAD status

	s := newTestServer(t)
	ctx := context.Background()

	const (
		ip        = "10.8.24.31"
		appName   = "testapp"
		service   = "api"
		sandboxID = "sandbox/testapp-api-dead-test"
	)

	s.addSandboxMapping(sandboxID, ip, appName, service)
	assert.Equal(t, appName, s.lookupAppForIP(ip))

	// Sandbox marked as DEAD (e.g., health check failed)
	deadSandbox := &compute_v1alpha.Sandbox{
		ID:     entity.Id(sandboxID),
		Status: compute_v1alpha.DEAD,
	}
	s.handleSandboxUpdate(ctx, deadSandbox, nil)

	assert.Empty(t, s.lookupAppForIP(ip),
		"sandbox should be removed from DNS when status changes to DEAD")
}

func TestNonRunningStatusNeverAdded(t *testing.T) {
	// Sandboxes that are not RUNNING should never be added to DNS,
	// even if they have network info.

	s := newTestServer(t)
	ctx := context.Background()

	const (
		ip        = "10.8.24.32"
		sandboxID = "sandbox/testapp-pending"
	)

	// A PENDING sandbox with network info should not be tracked
	pendingSandbox := &compute_v1alpha.Sandbox{
		ID:     entity.Id(sandboxID),
		Status: compute_v1alpha.PENDING,
		Network: []compute_v1alpha.Network{
			{Address: ip + "/24"},
		},
	}
	s.handleSandboxUpdate(ctx, pendingSandbox, nil)

	assert.Empty(t, s.lookupAppForIP(ip),
		"PENDING sandbox should not be added to DNS")
}

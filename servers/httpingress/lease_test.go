package httpingress

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"miren.dev/runtime/api/core/core_v1alpha"
	"miren.dev/runtime/components/activator"
)

// mockActivator implements activator.AppActivator for testing lease behavior.
type mockActivator struct {
	mu           sync.Mutex
	renewCount   int
	releaseCount int
	renewErr     error
	releasedURLs []string
	renewedURLs  []string
}

func (m *mockActivator) AcquireLease(ctx context.Context, ver *core_v1alpha.AppVersion, service string) (*activator.Lease, error) {
	return &activator.Lease{Size: 10, URL: "http://10.0.0.1:3000"}, nil
}

func (m *mockActivator) ReleaseLease(ctx context.Context, lease *activator.Lease) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.releaseCount++
	m.releasedURLs = append(m.releasedURLs, lease.URL)
	return nil
}

func (m *mockActivator) RenewLease(ctx context.Context, lease *activator.Lease) (*activator.Lease, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.renewErr != nil {
		return nil, m.renewErr
	}
	m.renewCount++
	m.renewedURLs = append(m.renewedURLs, lease.URL)
	return lease, nil
}

func newTestServer(aa *mockActivator) *Server {
	return &Server{
		Log:  slog.Default(),
		aa:   aa,
		apps: make(map[string]*appUsage),
	}
}

func TestLeaseKeptAliveWithinTTL(t *testing.T) {
	aa := &mockActivator{}
	srv := newTestServer(aa)
	ctx := context.Background()

	// Retain a lease (simulates a request acquiring one)
	actLease := &activator.Lease{Size: 10, URL: "http://10.0.0.1:3000"}
	ll := srv.retainLease(ctx, "app/myapp", actLease)

	// Simulate the request finishing — Uses goes to 0
	srv.releaseLease(ctx, ll)

	if ll.Uses != 0 {
		t.Fatalf("expected Uses=0 after release, got %d", ll.Uses)
	}

	// Run expireLeases — lease should be renewed, not evicted (within TTL)
	srv.expireLeases(ctx)

	aa.mu.Lock()
	defer aa.mu.Unlock()

	if aa.releaseCount != 0 {
		t.Errorf("expected 0 releases (lease within TTL), got %d", aa.releaseCount)
	}
	if aa.renewCount != 1 {
		t.Errorf("expected 1 renewal, got %d", aa.renewCount)
	}

	// Verify the lease is still in cache
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if _, ok := srv.apps["app/myapp"]; !ok {
		t.Error("expected app to still have cached leases")
	}
}

func TestLeaseEvictedAfterTTL(t *testing.T) {
	aa := &mockActivator{}
	srv := newTestServer(aa)
	ctx := context.Background()

	// Retain and release a lease
	actLease := &activator.Lease{Size: 10, URL: "http://10.0.0.1:3000"}
	ll := srv.retainLease(ctx, "app/myapp", actLease)
	srv.releaseLease(ctx, ll)

	// Backdate LastUsed to beyond the TTL
	srv.mu.Lock()
	ll.LastUsed = time.Now().Add(-(minLeaseTTL + time.Minute))
	srv.mu.Unlock()

	// Run expireLeases — lease should be evicted
	srv.expireLeases(ctx)

	aa.mu.Lock()
	defer aa.mu.Unlock()

	if aa.releaseCount != 1 {
		t.Errorf("expected 1 release (lease past TTL), got %d", aa.releaseCount)
	}
	if aa.renewCount != 0 {
		t.Errorf("expected 0 renewals, got %d", aa.renewCount)
	}

	// Verify the app is removed from cache
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if _, ok := srv.apps["app/myapp"]; ok {
		t.Error("expected app to be removed from cache after eviction")
	}
}

func TestActiveLeaseRenewed(t *testing.T) {
	aa := &mockActivator{}
	srv := newTestServer(aa)
	ctx := context.Background()

	// Retain a lease and keep it active (don't release — simulates in-flight request)
	actLease := &activator.Lease{Size: 10, URL: "http://10.0.0.1:3000"}
	ll := srv.retainLease(ctx, "app/myapp", actLease)

	if ll.Uses != 1 {
		t.Fatalf("expected Uses=1 after retain, got %d", ll.Uses)
	}

	// Run expireLeases — active lease should be renewed
	srv.expireLeases(ctx)

	aa.mu.Lock()
	defer aa.mu.Unlock()

	if aa.releaseCount != 0 {
		t.Errorf("expected 0 releases (lease is active), got %d", aa.releaseCount)
	}
	if aa.renewCount != 1 {
		t.Errorf("expected 1 renewal, got %d", aa.renewCount)
	}
}

func TestFailedRenewalEvictsLease(t *testing.T) {
	aa := &mockActivator{renewErr: errors.New("sandbox gone")}
	srv := newTestServer(aa)
	ctx := context.Background()

	// Retain and release a lease (within TTL, so it would normally be renewed)
	actLease := &activator.Lease{Size: 10, URL: "http://10.0.0.1:3000"}
	ll := srv.retainLease(ctx, "app/myapp", actLease)
	srv.releaseLease(ctx, ll)

	// Run expireLeases — renewal fails, lease should be dropped
	srv.expireLeases(ctx)

	// Verify the app is removed from cache (renewal failed, lease dropped)
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if _, ok := srv.apps["app/myapp"]; ok {
		t.Error("expected app to be removed after failed renewal")
	}
}

func TestUsageResetsAfterRenewal(t *testing.T) {
	aa := &mockActivator{}
	srv := newTestServer(aa)
	ctx := context.Background()

	// Retain a lease — Uses starts at 1
	actLease := &activator.Lease{Size: 10, URL: "http://10.0.0.1:3000"}
	srv.retainLease(ctx, "app/myapp", actLease)

	// Run expireLeases — should renew and reset Uses to 0
	srv.expireLeases(ctx)

	srv.mu.Lock()
	ar := srv.apps["app/myapp"]
	if ar == nil || len(ar.leases) == 0 {
		srv.mu.Unlock()
		t.Fatal("expected lease to still be cached")
	}
	uses := ar.leases[0].Uses
	srv.mu.Unlock()

	if uses != 0 {
		t.Errorf("expected Uses reset to 0 after renewal, got %d", uses)
	}
}

func TestLastUsedUpdatedOnUse(t *testing.T) {
	aa := &mockActivator{}
	srv := newTestServer(aa)
	ctx := context.Background()

	// Retain a lease
	actLease := &activator.Lease{Size: 10, URL: "http://10.0.0.1:3000"}
	ll := srv.retainLease(ctx, "app/myapp", actLease)
	srv.releaseLease(ctx, ll)

	// Record the initial LastUsed
	srv.mu.Lock()
	initial := ll.LastUsed
	srv.mu.Unlock()

	// Small sleep to ensure time advances
	time.Sleep(time.Millisecond)

	// Use the lease again (simulates another request hitting the cache)
	got, err := srv.useLease(ctx, "app/myapp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	switch {
	case got == nil:
		t.Fatal("expected to get a cached lease")
	case !got.LastUsed.After(initial):
		t.Errorf("expected LastUsed to advance on use, initial=%v updated=%v", initial, got.LastUsed)
	}
}

func TestMultipleAppsIndependentTTL(t *testing.T) {
	aa := &mockActivator{}
	srv := newTestServer(aa)
	ctx := context.Background()

	// App A: retain and release, then backdate past TTL
	leaseA := &activator.Lease{Size: 10, URL: "http://10.0.0.1:3000"}
	llA := srv.retainLease(ctx, "app/a", leaseA)
	srv.releaseLease(ctx, llA)
	srv.mu.Lock()
	llA.LastUsed = time.Now().Add(-(minLeaseTTL + time.Minute))
	srv.mu.Unlock()

	// App B: retain and release, still within TTL
	leaseB := &activator.Lease{Size: 10, URL: "http://10.0.0.2:3000"}
	llB := srv.retainLease(ctx, "app/b", leaseB)
	srv.releaseLease(ctx, llB)

	// Run expireLeases
	srv.expireLeases(ctx)

	srv.mu.Lock()
	defer srv.mu.Unlock()

	// App A should be evicted
	if _, ok := srv.apps["app/a"]; ok {
		t.Error("expected app/a to be evicted (past TTL)")
	}

	// App B should be retained
	if _, ok := srv.apps["app/b"]; !ok {
		t.Error("expected app/b to still be cached (within TTL)")
	}
}

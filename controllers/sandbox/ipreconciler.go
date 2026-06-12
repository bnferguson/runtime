package sandbox

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"time"

	"miren.dev/runtime/pkg/netdb"
)

// IPReconciler keeps the netdb IP lease bookkeeping in agreement with the
// addresses actually live on the bridge. The two can diverge — a release path
// that ran while a container kept running, a lease lost across a restart — and a
// divergence where netdb believes a live address is free leads to that address
// being handed to a second sandbox, a duplicate assignment (MIR-1238).
//
// Each cycle it:
//   - re-reserves any address that is live on the bridge but not reserved in
//     netdb (the repair direction — always safe, only ever adds a reservation
//     for an address already in use); and
//   - releases any address reserved in netdb with no live owner, but only after
//     it has been absent for several consecutive cycles (the reap direction —
//     conservative, to avoid racing a sandbox that is mid-create or mid-teardown).
type IPReconciler struct {
	Log    *slog.Logger
	Subnet *netdb.Subnet

	// LiveIPs returns the set of addresses currently assigned to running sandbox
	// containers, determined independently of netdb.
	LiveIPs func(ctx context.Context) (map[netip.Addr]bool, error)

	// CheckInterval is how often a reconcile cycle runs.
	CheckInterval time.Duration
	// ReleaseAfterMisses is how many consecutive cycles an address must be
	// reserved-but-not-live before it is released. With the default interval this
	// is a multi-minute grace window, comfortably longer than any normal sandbox
	// create (whose saga releases the reservation on failure anyway).
	ReleaseAfterMisses int

	misses map[netip.Addr]int
	cancel context.CancelFunc
}

// Start begins the periodic reconcile loop.
func (r *IPReconciler) Start(ctx context.Context) {
	if r.CheckInterval == 0 {
		r.CheckInterval = 5 * time.Minute
	}
	if r.ReleaseAfterMisses == 0 {
		r.ReleaseAfterMisses = 3
	}
	r.misses = make(map[netip.Addr]int)

	r.Log.Info("starting ip reconciler",
		"check_interval", r.CheckInterval,
		"release_after_misses", r.ReleaseAfterMisses)

	ctx, r.cancel = context.WithCancel(ctx)
	go r.monitor(ctx)
}

// Stop gracefully stops the reconciler.
func (r *IPReconciler) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
}

func (r *IPReconciler) monitor(ctx context.Context) {
	ticker := time.NewTicker(r.CheckInterval)
	defer ticker.Stop()

	// Run an initial reconcile on startup. This also serves boot reconciliation:
	// it re-reserves the IPs of every container that survived a restart, using the
	// live containers as the source of truth rather than the entity store.
	if err := r.reconcile(ctx); err != nil {
		r.Log.Error("initial ip reconcile failed", "error", err)
	}

	for {
		select {
		case <-ticker.C:
			if err := r.reconcile(ctx); err != nil {
				r.Log.Error("ip reconcile failed", "error", err)
			}
		case <-ctx.Done():
			r.Log.Info("ip reconciler stopped")
			return
		}
	}
}

func (r *IPReconciler) reconcile(ctx context.Context) error {
	if r.Subnet == nil || r.LiveIPs == nil {
		return nil
	}

	live, err := r.LiveIPs(ctx)
	if err != nil {
		return fmt.Errorf("enumerating live IPs: %w", err)
	}

	reserved, err := r.Subnet.ReservedAddrs()
	if err != nil {
		return fmt.Errorf("listing reserved addresses: %w", err)
	}
	reservedSet := make(map[netip.Addr]bool, len(reserved))
	for _, addr := range reserved {
		reservedSet[addr] = true
	}

	// Repair: an address live on the bridge but not reserved in netdb is the
	// exact divergence behind MIR-1238. Re-reserve it so it can never be handed
	// to a second sandbox.
	for addr := range live {
		if reservedSet[addr] {
			continue
		}
		if err := r.Subnet.ReserveSpecificAddr(addr); err != nil {
			r.Log.Error("ip reconciler failed to re-reserve live address", "addr", addr, "error", err)
			continue
		}
		r.Log.Warn("ip reconciler re-reserved a live address netdb had lost", "addr", addr)
	}

	// Reap: an address reserved in netdb with no live owner is a leaked lease.
	// Release it only after it has been absent for ReleaseAfterMisses consecutive
	// cycles, so a sandbox that has reserved its address but not yet booted its
	// container (or is mid-teardown) is not disturbed.
	for addr := range reservedSet {
		if live[addr] {
			delete(r.misses, addr)
			continue
		}

		r.misses[addr]++
		if r.misses[addr] < r.ReleaseAfterMisses {
			continue
		}

		if err := r.Subnet.ReleaseAddr(addr); err != nil {
			r.Log.Error("ip reconciler failed to release leaked address", "addr", addr, "error", err)
			continue
		}
		delete(r.misses, addr)
		r.Log.Warn("ip reconciler released leaked reservation with no live owner",
			"addr", addr, "after_misses", r.ReleaseAfterMisses)
	}

	// Forget miss counters for addresses that are no longer reserved (e.g. they
	// were released elsewhere) so the map cannot grow without bound.
	for addr := range r.misses {
		if !reservedSet[addr] {
			delete(r.misses, addr)
		}
	}

	return nil
}

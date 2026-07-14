// Package indexgc runs a bounded, best-effort background sweep that removes
// stale entity-store index (collection) entries whose backing entity is gone.
//
// The source of these orphans was fixed upstream (atomic deletes, lease-bound
// session index entries), so this exists to drain any legacy backlog on upgrade
// and to keep a cluster self-healing if a future leak ever reappears, without
// anyone having to run `miren debug reindex` by hand. It deliberately does its
// work off the read path: foreground reads stay pure and never issue deletes.
package indexgc

import (
	"context"
	"log/slog"
	"time"

	"miren.dev/runtime/pkg/entity"
)

const (
	// initialDelay keeps the first sweep clear of the boot storm and any
	// startup-time additive reindex before it begins deleting.
	initialDelay = 1 * time.Minute
)

// GCConfig tunes the background sweep. Defaults are intentionally gentle: this
// is convergence insurance, not active firefighting, so it drains slowly and
// stays out of the way.
type GCConfig struct {
	// CheckInterval is how often to run a sweep.
	CheckInterval time.Duration
	// MaxDeletesPerSweep caps deletions per sweep so a large legacy backlog
	// drains over several sweeps rather than one thundering pass. Zero means
	// unbounded.
	MaxDeletesPerSweep int
	// BatchPause is slept periodically during deletion to rate-limit write
	// pressure. Zero disables pacing.
	BatchPause time.Duration
	// SweepTimeout bounds a single sweep. A truncated sweep is fine: the next
	// one resumes the work, since the pass is idempotent.
	SweepTimeout time.Duration
}

// DefaultGCConfig returns the default (gentle) configuration.
func DefaultGCConfig() GCConfig {
	return GCConfig{
		CheckInterval:      1 * time.Hour,
		MaxDeletesPerSweep: 500,
		BatchPause:         1 * time.Second,
		SweepTimeout:       10 * time.Minute,
	}
}

// GCController periodically removes stale collection entries from the entity
// store. It operates directly on the EtcdStore's CAS-guarded cleanup rather than
// going through the entity-access client, since it works below the index it is
// repairing.
type GCController struct {
	Log    *slog.Logger
	Store  *entity.EtcdStore
	Config GCConfig

	cancel context.CancelFunc
}

// Start begins the periodic sweep.
func (c *GCController) Start(ctx context.Context) {
	c.Log.Info("starting stale index GC controller",
		"check_interval", c.Config.CheckInterval,
		"max_deletes_per_sweep", c.Config.MaxDeletesPerSweep)

	ctx, c.cancel = context.WithCancel(ctx)
	go c.run(ctx)
}

// Stop gracefully stops the controller.
func (c *GCController) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
}

func (c *GCController) run(ctx context.Context) {
	// Wait out the initial delay before the first sweep, then start the
	// periodic ticker. Constructing the ticker after the delay (not before)
	// keeps the first interval a true CheckInterval and avoids a back-to-back
	// double-fire if initialDelay is ever tuned longer than CheckInterval.
	select {
	case <-time.After(initialDelay):
		c.sweep(ctx)
	case <-ctx.Done():
		c.Log.Info("stale index GC controller stopped")
		return
	}

	ticker := time.NewTicker(c.Config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.sweep(ctx)
		case <-ctx.Done():
			c.Log.Info("stale index GC controller stopped")
			return
		}
	}
}

// sweep runs one bounded cleanup pass. It is strictly best-effort: errors are
// logged, never propagated, and never block a foreground operation.
func (c *GCController) sweep(ctx context.Context) {
	sweepCtx := ctx
	if c.Config.SweepTimeout > 0 {
		var cancel context.CancelFunc
		sweepCtx, cancel = context.WithTimeout(ctx, c.Config.SweepTimeout)
		defer cancel()
	}

	stats, err := c.Store.CleanupStaleCollectionEntries(sweepCtx, c.Log, entity.CleanupOptions{
		MaxDeletes: c.Config.MaxDeletesPerSweep,
		BatchPause: c.Config.BatchPause,
	})
	if err != nil {
		// A truncated scan (deadline/shutdown) is expected and not alarming; the
		// next sweep resumes. Log at Warn so a persistent hard error is still
		// visible.
		c.Log.Warn("stale index GC sweep ended early", "error", err,
			"scanned", stats.CollectionEntriesScanned,
			"removed", stats.StaleEntriesRemoved)
		return
	}

	if stats.StaleEntriesFound > 0 || stats.StaleEntriesRemoved > 0 {
		c.Log.Info("stale index GC sweep complete",
			"scanned", stats.CollectionEntriesScanned,
			"found", stats.StaleEntriesFound,
			"removed", stats.StaleEntriesRemoved,
			"cas_conflicts", stats.CASConflicts,
			"by_collection", stats.RemovedByCollection)
	} else {
		c.Log.Debug("stale index GC sweep complete, nothing stale",
			"scanned", stats.CollectionEntriesScanned)
	}
}

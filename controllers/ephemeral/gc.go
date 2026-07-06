package ephemeral

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"miren.dev/runtime/api/core/core_v1alpha"
	"miren.dev/runtime/api/entityserver/entityserver_v1alpha"
	"miren.dev/runtime/pkg/appversion"
	"miren.dev/runtime/pkg/entity"
	ephemeralx "miren.dev/runtime/pkg/ephemeral"
)

type GCConfig struct {
	CheckInterval time.Duration
}

func DefaultGCConfig() GCConfig {
	return GCConfig{
		CheckInterval: 5 * time.Minute,
	}
}

type GCResult struct {
	DeletedVersions int
	FailedVersions  int
	TotalScanned    int
}

type GCController struct {
	Log    *slog.Logger
	EAC    *entityserver_v1alpha.EntityAccessClient
	Config GCConfig

	cancel context.CancelFunc
}

func (c *GCController) Start(ctx context.Context) {
	c.Log.Info("starting ephemeral GC controller",
		"check_interval", c.Config.CheckInterval)

	ctx, c.cancel = context.WithCancel(ctx)
	go c.run(ctx)
}

func (c *GCController) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
}

func (c *GCController) run(ctx context.Context) {
	ticker := time.NewTicker(c.Config.CheckInterval)
	defer ticker.Stop()

	// Run initial GC after a short delay
	select {
	case <-time.After(30 * time.Second):
		c.runGCWithLogging(ctx)
	case <-ctx.Done():
		c.Log.Info("ephemeral GC controller stopped")
		return
	}

	for {
		select {
		case <-ticker.C:
			c.runGCWithLogging(ctx)
		case <-ctx.Done():
			c.Log.Info("ephemeral GC controller stopped")
			return
		}
	}
}

func (c *GCController) runGCWithLogging(ctx context.Context) {
	result, err := c.RunGC(ctx)
	if err != nil {
		c.Log.Error("ephemeral GC failed", "error", err)
		return
	}

	if result.DeletedVersions > 0 || result.FailedVersions > 0 {
		c.Log.Info("ephemeral GC complete",
			"deleted", result.DeletedVersions,
			"failed", result.FailedVersions,
			"scanned", result.TotalScanned)
	} else {
		c.Log.Debug("ephemeral GC complete, no expired versions",
			"scanned", result.TotalScanned)
	}
}

// RunGC scans for expired ephemeral versions and deletes them along with
// their associated sandbox pools.
func (c *GCController) RunGC(ctx context.Context) (*GCResult, error) {
	result := &GCResult{}

	gcCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	// Scan every AppVersion and filter to the ephemeral ones. This set is
	// bounded: non-ephemeral versions are capped by the retention GC
	// (controllers/version) and ephemeral ones by DefaultMaxEphemeral per app,
	// so the scan does not grow with total deploy history. The store's index
	// lookups are equality-only, so there is no range query over
	// ephemeral_expires_at to target expired versions directly. If this scan
	// ever costs too much (roughly thousands of apps, given the 5-minute
	// cadence), the fix is an equality-indexed "ephemeral" marker attr so we can
	// List(ephemeral=true) instead of scanning-and-filtering every version.
	resp, err := c.EAC.List(gcCtx, entity.Ref(entity.EntityKind, core_v1alpha.KindAppVersion))
	if err != nil {
		// Fall back to scanning by iterating through known apps
		return c.runGCByApps(gcCtx)
	}

	now := time.Now()
	for _, e := range resp.Values() {
		var av core_v1alpha.AppVersion
		av.Decode(e.Entity())

		if av.EphemeralLabel == "" {
			continue
		}

		result.TotalScanned++

		if av.EphemeralExpiresAt.IsZero() || now.Before(av.EphemeralExpiresAt) {
			continue
		}

		c.Log.Info("deleting expired ephemeral version",
			"version_id", av.ID,
			"label", av.EphemeralLabel,
			"expired_at", av.EphemeralExpiresAt)

		if err := appversion.DeleteWithPools(gcCtx, c.EAC, &av, c.Log); err != nil {
			c.Log.Error("failed to delete expired ephemeral version",
				"version_id", av.ID, "error", err)
			result.FailedVersions++
		} else {
			result.DeletedVersions++
		}
	}

	return result, nil
}

// runGCByApps is a fallback GC strategy that iterates through apps
func (c *GCController) runGCByApps(ctx context.Context) (*GCResult, error) {
	result := &GCResult{}

	// List all apps
	resp, err := c.EAC.List(ctx, entity.Ref(entity.EntityKind, core_v1alpha.KindApp))
	if err != nil {
		return result, fmt.Errorf("failed to list apps: %w", err)
	}

	for _, e := range resp.Values() {
		var app core_v1alpha.App
		app.Decode(e.Entity())

		deleted, err := ephemeralx.DeleteExpired(ctx, c.EAC, app.ID, c.Log)
		if err != nil {
			c.Log.Error("failed to clean up ephemeral versions for app",
				"app_id", app.ID, "error", err)
		}
		result.DeletedVersions += deleted
	}

	return result, nil
}

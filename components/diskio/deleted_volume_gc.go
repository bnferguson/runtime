package diskio

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"
)

// DeletedVolumeGCConfig holds configuration for deleted volume garbage collection.
type DeletedVolumeGCConfig struct {
	CheckInterval time.Duration
	RetentionDays int
}

// DefaultDeletedVolumeGCConfig returns the default GC configuration.
func DefaultDeletedVolumeGCConfig() DeletedVolumeGCConfig {
	return DeletedVolumeGCConfig{
		CheckInterval: 1 * time.Hour,
		RetentionDays: defaultRetainDays,
	}
}

// DeletedVolumeGCResult contains information about volumes processed during GC.
type DeletedVolumeGCResult struct {
	Purged   int
	Retained int
	Errors   int
}

// DeletedVolumeGC periodically purges soft-deleted disk volumes that have
// exceeded the retention period.
type DeletedVolumeGC struct {
	Log      *slog.Logger
	DataPath string
	Config   DeletedVolumeGCConfig

	cancel context.CancelFunc
}

// Start begins the periodic GC process.
func (g *DeletedVolumeGC) Start(ctx context.Context) {
	g.Log.Info("starting deleted volume GC",
		"check_interval", g.Config.CheckInterval,
		"retention_days", g.Config.RetentionDays)

	ctx, g.cancel = context.WithCancel(ctx)
	go g.run(ctx)
}

// Stop gracefully stops the controller.
func (g *DeletedVolumeGC) Stop() {
	if g.cancel != nil {
		g.cancel()
	}
}

func (g *DeletedVolumeGC) run(ctx context.Context) {
	ticker := time.NewTicker(g.Config.CheckInterval)
	defer ticker.Stop()

	// Run an initial GC after a short delay
	select {
	case <-time.After(30 * time.Second):
		g.runGCWithLogging()
	case <-ctx.Done():
		g.Log.Info("deleted volume GC stopped")
		return
	}

	for {
		select {
		case <-ticker.C:
			g.runGCWithLogging()
		case <-ctx.Done():
			g.Log.Info("deleted volume GC stopped")
			return
		}
	}
}

func (g *DeletedVolumeGC) runGCWithLogging() {
	result, err := g.RunGC()
	if err != nil {
		g.Log.Error("deleted volume GC failed", "error", err)
		return
	}

	if result.Purged > 0 || result.Errors > 0 {
		g.Log.Info("deleted volume GC complete",
			"purged", result.Purged,
			"retained", result.Retained,
			"errors", result.Errors)
	} else {
		g.Log.Debug("deleted volume GC complete, nothing to purge",
			"retained", result.Retained)
	}
}

// RunGC scans the deleted-volumes directory and removes entries that have
// exceeded the retention period.
func (g *DeletedVolumeGC) RunGC() (*DeletedVolumeGCResult, error) {
	result := &DeletedVolumeGCResult{}

	entries, err := ListDeletedVolumes(g.DataPath)
	if err != nil {
		return result, fmt.Errorf("listing deleted volumes: %w", err)
	}

	now := time.Now()
	retentionCutoff := now.Add(-time.Duration(g.Config.RetentionDays) * 24 * time.Hour)

	for _, entry := range entries {
		if entry.Metadata.DeletedAt.After(retentionCutoff) {
			result.Retained++
			continue
		}

		g.Log.Info("purging expired deleted volume",
			"disk_name", entry.Metadata.DiskName,
			"volume_id", entry.Metadata.VolumeID,
			"deleted_at", entry.Metadata.DeletedAt)

		if err := os.RemoveAll(entry.Path); err != nil {
			g.Log.Warn("failed to purge deleted volume",
				"path", entry.Path, "error", err)
			result.Errors++
			continue
		}

		result.Purged++
	}

	return result, nil
}

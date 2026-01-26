package artifact

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"miren.dev/runtime/api/core/core_v1alpha"
	"miren.dev/runtime/api/entityserver/entityserver_v1alpha"
	"miren.dev/runtime/pkg/entity"
)

// GCConfig holds configuration for artifact garbage collection.
type GCConfig struct {
	// CheckInterval is how often to run the GC sweep (default: 1h)
	CheckInterval time.Duration
	// RetentionDays is how many days to keep artifacts regardless of count (default: 30)
	RetentionDays int
	// RetentionCount is how many recent artifacts per app to keep regardless of age (default: 20)
	RetentionCount int
}

// DefaultGCConfig returns the default GC configuration.
func DefaultGCConfig() GCConfig {
	return GCConfig{
		CheckInterval:  1 * time.Hour,
		RetentionDays:  30,
		RetentionCount: 20,
	}
}

// GCResult contains information about artifacts processed during GC.
type GCResult struct {
	// ArchivedArtifacts contains IDs of artifacts transitioned to archived
	ArchivedArtifacts []entity.Id
	// FailedArtifacts contains IDs and errors for artifacts that failed to update
	FailedArtifacts map[entity.Id]error
	// TotalArtifacts is the total number of artifacts evaluated
	TotalArtifacts int
	// RetainedArtifacts is the number of artifacts kept active
	RetainedArtifacts int
}

// GCController periodically applies retention policies to artifacts,
// transitioning old artifacts to "archived" status.
type GCController struct {
	Log    *slog.Logger
	EAC    *entityserver_v1alpha.EntityAccessClient
	Config GCConfig

	cancel context.CancelFunc
}

// Start begins the periodic GC process.
func (c *GCController) Start(ctx context.Context) {
	c.Log.Info("starting artifact GC controller",
		"check_interval", c.Config.CheckInterval,
		"retention_days", c.Config.RetentionDays,
		"retention_count", c.Config.RetentionCount)

	ctx, c.cancel = context.WithCancel(ctx)
	go c.run(ctx)
}

// Stop gracefully stops the controller.
func (c *GCController) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
}

// run executes the periodic GC loop.
func (c *GCController) run(ctx context.Context) {
	ticker := time.NewTicker(c.Config.CheckInterval)
	defer ticker.Stop()

	// Run an initial GC on startup after a short delay
	select {
	case <-time.After(30 * time.Second):
		c.runGCWithLogging(ctx)
	case <-ctx.Done():
		c.Log.Info("artifact GC controller stopped")
		return
	}

	for {
		select {
		case <-ticker.C:
			c.runGCWithLogging(ctx)
		case <-ctx.Done():
			c.Log.Info("artifact GC controller stopped")
			return
		}
	}
}

// runGCWithLogging runs GC and logs results.
func (c *GCController) runGCWithLogging(ctx context.Context) {
	result, err := c.RunGC(ctx)
	if err != nil {
		c.Log.Error("artifact GC failed", "error", err)
		return
	}

	if len(result.ArchivedArtifacts) > 0 || len(result.FailedArtifacts) > 0 {
		c.Log.Info("artifact GC complete",
			"archived", len(result.ArchivedArtifacts),
			"failed", len(result.FailedArtifacts),
			"retained", result.RetainedArtifacts,
			"total", result.TotalArtifacts)

		for _, id := range result.ArchivedArtifacts {
			c.Log.Debug("archived artifact", "artifact", id)
		}
		for id, err := range result.FailedArtifacts {
			c.Log.Warn("failed to archive artifact", "artifact", id, "error", err)
		}
	} else {
		c.Log.Debug("artifact GC complete, no artifacts archived",
			"retained", result.RetainedArtifacts,
			"total", result.TotalArtifacts)
	}
}

// RunGC applies the retention policy to all artifacts.
func (c *GCController) RunGC(ctx context.Context) (*GCResult, error) {
	result := &GCResult{
		ArchivedArtifacts: []entity.Id{},
		FailedArtifacts:   make(map[entity.Id]error),
	}

	gcCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// List only active artifacts - archived and empty-status artifacts are skipped
	resp, err := c.EAC.List(gcCtx, entity.Ref(core_v1alpha.ArtifactStatusId, core_v1alpha.ArtifactStatusActiveId))
	if err != nil {
		return result, fmt.Errorf("failed to list active artifacts: %w", err)
	}

	now := time.Now()
	retentionCutoff := now.Add(-time.Duration(c.Config.RetentionDays) * 24 * time.Hour)

	// Group artifacts by app
	type artifactInfo struct {
		artifact  core_v1alpha.Artifact
		revision  int64
		createdAt time.Time
	}

	artifactsByApp := make(map[entity.Id][]artifactInfo)
	var orphanedArtifacts []artifactInfo

	for _, e := range resp.Values() {
		var art core_v1alpha.Artifact
		art.Decode(e.Entity())

		info := artifactInfo{
			artifact:  art,
			revision:  e.Revision(),
			createdAt: time.UnixMilli(e.CreatedAt()),
		}

		if art.App == "" {
			// Orphaned artifact (no app reference)
			orphanedArtifacts = append(orphanedArtifacts, info)
		} else {
			artifactsByApp[art.App] = append(artifactsByApp[art.App], info)
		}
	}

	result.TotalArtifacts = len(resp.Values())

	// Process artifacts per app
	for _, artifacts := range artifactsByApp {
		// Sort by creation time, newest first
		sort.Slice(artifacts, func(i, j int) bool {
			return artifacts[i].createdAt.After(artifacts[j].createdAt)
		})

		for i, info := range artifacts {
			shouldRetain := false

			// Keep if within retention days
			if info.createdAt.After(retentionCutoff) {
				shouldRetain = true
			}

			// Keep if within retention count (first N in sorted list)
			if i < c.Config.RetentionCount {
				shouldRetain = true
			}

			if shouldRetain {
				result.RetainedArtifacts++
			} else {
				// Archive this artifact
				err := c.archiveArtifact(gcCtx, info.artifact, info.revision)
				if err != nil {
					result.FailedArtifacts[info.artifact.ID] = err
				} else {
					result.ArchivedArtifacts = append(result.ArchivedArtifacts, info.artifact.ID)
				}
			}
		}
	}

	// Handle orphaned artifacts - archive if older than retention days
	for _, info := range orphanedArtifacts {
		if info.createdAt.After(retentionCutoff) {
			result.RetainedArtifacts++
		} else {
			err := c.archiveArtifact(gcCtx, info.artifact, info.revision)
			if err != nil {
				result.FailedArtifacts[info.artifact.ID] = err
			} else {
				result.ArchivedArtifacts = append(result.ArchivedArtifacts, info.artifact.ID)
			}
		}
	}

	return result, nil
}

// archiveArtifact updates an artifact's status to archived.
func (c *GCController) archiveArtifact(ctx context.Context, art core_v1alpha.Artifact, revision int64) error {
	patchAttrs := entity.New(
		entity.Ref(entity.DBId, art.ID),
		(&core_v1alpha.Artifact{
			Status: core_v1alpha.ARCHIVED,
		}).Encode,
	)
	_, err := c.EAC.Patch(ctx, patchAttrs.Attrs(), revision)
	if err != nil {
		return fmt.Errorf("failed to archive artifact: %w", err)
	}

	return nil
}

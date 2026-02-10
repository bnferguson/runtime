package sandbox

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/containerd/containerd/namespaces"
	containerd "github.com/containerd/containerd/v2/client"

	compute "miren.dev/runtime/api/compute/compute_v1alpha"
	"miren.dev/runtime/api/core/core_v1alpha"
	"miren.dev/runtime/api/entityserver/entityserver_v1alpha"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/sysstats"
)

// ImageGCConfig holds configuration for the image garbage collector.
type ImageGCConfig struct {
	// ScheduledGCInterval is how often to run scheduled GC regardless of pressure (default: 168h/weekly)
	ScheduledGCInterval time.Duration
	// PressureCheckInterval is how often to check disk pressure (default: 1h)
	PressureCheckInterval time.Duration
	// DiskPressureThreshold is the disk usage percentage that triggers immediate GC (default: 80%)
	DiskPressureThreshold float64
}

// DefaultImageGCConfig returns the default configuration for image GC.
func DefaultImageGCConfig() ImageGCConfig {
	return ImageGCConfig{
		ScheduledGCInterval:   168 * time.Hour, // Weekly
		PressureCheckInterval: 1 * time.Hour,
		DiskPressureThreshold: 80.0,
	}
}

// ImageGCResult contains information about images cleaned up during GC.
type ImageGCResult struct {
	// DeletedImages contains names of images successfully removed
	DeletedImages []string
	// FailedImages contains names and errors for images that failed to be removed
	FailedImages map[string]error
	// TotalImages is the total number of images before GC
	TotalImages int
	// RetainedImages is the number of images kept
	RetainedImages int
}

// ImageWatchdog periodically garbage collects container images from containerd.
// It uses Artifact entity status to determine which images to remove:
// - Images with no corresponding Artifact are kept (infrastructure images, etc.)
// - Images with Artifact status "active" or empty are kept
// - Images with Artifact status "archived" are deleted
type ImageWatchdog struct {
	Log *slog.Logger
	CC  *containerd.Client
	EAC *entityserver_v1alpha.EntityAccessClient

	Namespace string
	DataPath  string
	Config    ImageGCConfig

	cancel context.CancelFunc
}

// Start begins the periodic image cleanup process.
func (w *ImageWatchdog) Start(ctx context.Context) {
	w.Log.Info("starting image watchdog",
		"scheduled_interval", w.Config.ScheduledGCInterval,
		"pressure_check_interval", w.Config.PressureCheckInterval,
		"pressure_threshold", w.Config.DiskPressureThreshold)

	ctx, w.cancel = context.WithCancel(ctx)
	go w.monitor(ctx)
}

// Stop gracefully stops the watchdog.
func (w *ImageWatchdog) Stop() {
	if w.cancel != nil {
		w.cancel()
	}
}

// monitor runs the periodic GC loops.
func (w *ImageWatchdog) monitor(ctx context.Context) {
	pressureTicker := time.NewTicker(w.Config.PressureCheckInterval)
	scheduledTicker := time.NewTicker(w.Config.ScheduledGCInterval)
	defer pressureTicker.Stop()
	defer scheduledTicker.Stop()

	// Run an initial pressure check on startup
	w.checkAndRunGC(ctx, "startup")

	for {
		select {
		case <-pressureTicker.C:
			w.checkAndRunGC(ctx, "pressure_check")
		case <-scheduledTicker.C:
			w.runScheduledGC(ctx)
		case <-ctx.Done():
			w.Log.Info("image watchdog stopped")
			return
		}
	}
}

// checkAndRunGC checks disk pressure and runs GC if threshold is exceeded.
func (w *ImageWatchdog) checkAndRunGC(ctx context.Context, trigger string) {
	stats := sysstats.CollectSystemStats(w.DataPath)
	w.Log.Debug("checking disk pressure",
		"trigger", trigger,
		"storage_percent", stats.StoragePercent,
		"threshold", w.Config.DiskPressureThreshold)

	if stats.StoragePercent >= w.Config.DiskPressureThreshold {
		w.Log.Info("disk pressure threshold exceeded, running GC",
			"storage_percent", stats.StoragePercent,
			"threshold", w.Config.DiskPressureThreshold)
		w.runGCWithLogging(ctx, "disk_pressure")
	}
}

// runScheduledGC runs the weekly scheduled GC.
func (w *ImageWatchdog) runScheduledGC(ctx context.Context) {
	w.Log.Info("running scheduled image GC")
	w.runGCWithLogging(ctx, "scheduled")
}

// runGCWithLogging runs GC and logs results.
func (w *ImageWatchdog) runGCWithLogging(ctx context.Context, trigger string) {
	result, err := w.RunGC(ctx)
	if err != nil {
		w.Log.Error("image GC failed", "trigger", trigger, "error", err)
	} else if len(result.DeletedImages) > 0 || len(result.FailedImages) > 0 {
		w.Log.Info("image GC complete",
			"trigger", trigger,
			"deleted", len(result.DeletedImages),
			"failed", len(result.FailedImages),
			"retained", result.RetainedImages,
			"total", result.TotalImages)

		for _, img := range result.DeletedImages {
			w.Log.Debug("deleted image", "image", img)
		}
		for img, err := range result.FailedImages {
			w.Log.Warn("failed to delete image", "image", img, "error", err)
		}
	} else {
		w.Log.Debug("image GC complete, no images deleted",
			"trigger", trigger,
			"retained", result.RetainedImages,
			"total", result.TotalImages)
	}

	// Run blob GC independently of image GC
	blobResult, blobErr := w.RunBlobGC(ctx)
	if blobErr != nil {
		w.Log.Error("blob GC failed", "trigger", trigger, "error", blobErr)
	} else if len(blobResult.DeletedBlobs) > 0 || len(blobResult.FailedBlobs) > 0 {
		w.Log.Info("blob GC complete",
			"trigger", trigger,
			"deleted", len(blobResult.DeletedBlobs),
			"failed", len(blobResult.FailedBlobs),
			"retained", blobResult.RetainedBlobs,
			"total", blobResult.TotalBlobs)

		for _, blob := range blobResult.DeletedBlobs {
			w.Log.Debug("deleted blob", "blob", blob)
		}
		for blob, err := range blobResult.FailedBlobs {
			w.Log.Warn("failed to delete blob", "blob", blob, "error", err)
		}
	} else {
		w.Log.Debug("blob GC complete, no blobs deleted",
			"trigger", trigger,
			"retained", blobResult.RetainedBlobs,
			"total", blobResult.TotalBlobs)
	}
}

// RunGC performs garbage collection of unused images.
func (w *ImageWatchdog) RunGC(ctx context.Context) (*ImageGCResult, error) {
	result := &ImageGCResult{
		DeletedImages: []string{},
		FailedImages:  make(map[string]error),
	}

	gcCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	gcCtx = namespaces.WithNamespace(gcCtx, w.Namespace)

	// List all images
	images, err := w.CC.ListImages(gcCtx)
	if err != nil {
		return result, fmt.Errorf("failed to list images: %w", err)
	}

	result.TotalImages = len(images)

	// Get images in use by running sandboxes (these are always kept)
	inUseImages, err := w.collectInUseImages(gcCtx)
	if err != nil {
		return result, fmt.Errorf("failed to collect in-use images: %w", err)
	}

	// Build a map of artifact ID -> status for quick lookup
	artifactStatuses, err := w.collectArtifactStatuses(gcCtx)
	if err != nil {
		return result, fmt.Errorf("failed to collect artifact statuses: %w", err)
	}

	// Process each image
	for _, img := range images {
		imgName := img.Name()

		// Always keep images in use by running sandboxes
		if inUseImages[imgName] {
			result.RetainedImages++
			continue
		}

		// Try to parse the image name to extract artifact ID
		artifactID := w.parseArtifactID(imgName)
		if artifactID == "" {
			// Not a miren-managed image (infrastructure, etc.) - keep it
			result.RetainedImages++
			continue
		}

		// Look up artifact status
		status, found := artifactStatuses[artifactID]
		if !found {
			// No artifact entity found - keep the image (safe default)
			result.RetainedImages++
			continue
		}

		// Only delete if artifact is explicitly archived
		if status != core_v1alpha.ARCHIVED {
			result.RetainedImages++
			continue
		}

		// Artifact is archived - delete the image
		w.Log.Debug("deleting archived image", "image", imgName, "artifact", artifactID)
		err := w.CC.ImageService().Delete(gcCtx, imgName)
		if err != nil {
			result.FailedImages[imgName] = err
		} else {
			result.DeletedImages = append(result.DeletedImages, imgName)
		}
	}

	return result, nil
}

// parseArtifactID extracts the artifact ID from an image name.
// Image format: cluster.local:5000/{app}:{artifact-name}
// Artifact ID format: artifact/{artifact-name}
// Returns empty string if the image doesn't match the expected format.
func (w *ImageWatchdog) parseArtifactID(imageName string) string {
	// Expected format: cluster.local:5000/{app}:{artifact-name}
	if !strings.HasPrefix(imageName, "cluster.local:5000/") {
		return ""
	}

	// Remove the registry prefix
	rest := strings.TrimPrefix(imageName, "cluster.local:5000/")

	// Split on : to get app and artifact name
	parts := strings.SplitN(rest, ":", 2)
	if len(parts) != 2 {
		return ""
	}

	artifactName := parts[1]
	if artifactName == "" {
		return ""
	}

	// Construct the artifact entity ID
	return "artifact/" + artifactName
}

// collectInUseImages returns images referenced by running/pending sandboxes.
func (w *ImageWatchdog) collectInUseImages(ctx context.Context) (map[string]bool, error) {
	images := make(map[string]bool)

	resp, err := w.EAC.List(ctx, entity.Ref(entity.EntityKind, compute.KindSandbox))
	if err != nil {
		return nil, fmt.Errorf("failed to list sandboxes: %w", err)
	}

	for _, e := range resp.Values() {
		var sb compute.Sandbox
		sb.Decode(e.Entity())

		// Only consider sandboxes that are active or booting
		if sb.Status != compute.RUNNING && sb.Status != compute.PENDING && sb.Status != compute.NOT_READY {
			continue
		}

		// Add all container images
		for _, container := range sb.Spec.Container {
			if container.Image != "" {
				images[container.Image] = true
			}
		}
	}

	w.Log.Debug("collected in-use images", "count", len(images))
	return images, nil
}

// collectArtifactStatuses returns a map of artifact ID (string) to status.
func (w *ImageWatchdog) collectArtifactStatuses(ctx context.Context) (map[string]core_v1alpha.ArtifactStatus, error) {
	statuses := make(map[string]core_v1alpha.ArtifactStatus)

	resp, err := w.EAC.List(ctx, entity.Ref(entity.EntityKind, core_v1alpha.KindArtifact))
	if err != nil {
		return nil, fmt.Errorf("failed to list artifacts: %w", err)
	}

	for _, e := range resp.Values() {
		var art core_v1alpha.Artifact
		art.Decode(e.Entity())

		// Store status (may be empty for old artifacts)
		statuses[string(art.ID)] = art.Status
	}

	w.Log.Debug("collected artifact statuses", "count", len(statuses))
	return statuses, nil
}

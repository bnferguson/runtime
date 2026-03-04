package diskio

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"miren.dev/runtime/api/storage/storage_v1alpha"
)

// LogSegmentUploader uploads completed log segments to cloud storage.
type LogSegmentUploader interface {
	// UploadSegment uploads a log segment and returns the cloud segment ID.
	UploadSegment(ctx context.Context, volumeID, segmentPath string) (segmentID string, err error)
}

// LogWatcher monitors accelerator volume log directories for completed segments.
// When an uploader is configured, segments are uploaded then removed.
// When no uploader is configured (cloud not available), segments are simply deleted.
type LogWatcher struct {
	log      *slog.Logger
	state    *State
	uploader LogSegmentUploader // nil when cloud is not configured
	interval time.Duration
}

// NewLogWatcher creates a new LogWatcher that scans at the given interval.
// Pass nil for uploader to just delete logs without uploading.
func NewLogWatcher(log *slog.Logger, state *State, uploader LogSegmentUploader, interval time.Duration) *LogWatcher {
	return &LogWatcher{
		log:      log.With("module", "log-watcher"),
		state:    state,
		uploader: uploader,
		interval: interval,
	}
}

// Run starts the log watcher loop. It blocks until the context is cancelled.
func (w *LogWatcher) Run(ctx context.Context) error {
	if w.interval <= 0 {
		return fmt.Errorf("log watcher interval must be positive, got %s", w.interval)
	}
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			w.scanAndUpload(ctx)
		}
	}
}

func (w *LogWatcher) scanAndUpload(ctx context.Context) {
	for _, vol := range w.state.ListVolumes() {
		if vol.Mode != storage_v1alpha.VM_ACCELERATOR {
			continue
		}

		logDir := filepath.Join(vol.DiskPath, "logs")
		entries, err := os.ReadDir(logDir)
		if err != nil {
			if !os.IsNotExist(err) {
				w.log.Warn("failed to read log directory", "path", logDir, "error", err)
			}
			continue
		}

		for _, e := range entries {
			if e.IsDir() {
				continue
			}

			name := e.Name()

			// Only process completed .log files, skip .log.tmp (in-progress)
			if !strings.HasSuffix(name, ".log") || strings.HasSuffix(name, ".log.tmp") {
				continue
			}

			segPath := filepath.Join(logDir, name)

			if w.uploader != nil {
				_, err := w.uploader.UploadSegment(ctx, vol.VolumeId, segPath)
				if err != nil {
					w.log.Warn("failed to upload segment", "path", segPath, "error", err)
					continue
				}
			}

			// Update the log horizon so replay won't re-apply this segment
			if err := updateLogHorizonFromPath(vol.DiskPath, segPath); err != nil {
				w.log.Warn("failed to update log horizon, keeping segment file", "path", segPath, "error", err)
				continue
			}

			if err := os.Remove(segPath); err != nil {
				w.log.Warn("failed to remove segment", "path", segPath, "error", err)
			}
		}
	}
}

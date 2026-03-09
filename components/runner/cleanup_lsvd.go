package runner

import (
	"log/slog"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"miren.dev/runtime/pkg/outboard"
)

// stopOrphanedLSVDServer checks for a running lsvd-server outboard process
// left over from a previous version and stops it. This handles the upgrade
// case where the old version ran lsvd-server as an outboard process but the
// new version uses universal mode disk I/O with loop devices instead.
func stopOrphanedLSVDServer(log *slog.Logger, dataPath string) {
	configPath := filepath.Join(dataPath, "outboard", "lsvd-server", "outboard.json")

	cfg, err := outboard.ReadConfig(configPath)
	if err != nil {
		// No config file means no old lsvd-server was running
		return
	}

	if cfg.PID == 0 {
		return
	}

	// Check if the process is still running
	if err := syscall.Kill(cfg.PID, 0); err != nil {
		log.Info("old lsvd-server process is not running, cleaning up config", "pid", cfg.PID)
		os.RemoveAll(filepath.Join(dataPath, "outboard", "lsvd-server"))
		return
	}

	log.Info("found orphaned lsvd-server process from previous version, stopping it", "pid", cfg.PID)

	// Send SIGTERM to request graceful shutdown
	if err := syscall.Kill(cfg.PID, syscall.SIGTERM); err != nil {
		log.Warn("failed to send SIGTERM to lsvd-server", "pid", cfg.PID, "error", err)
		return
	}

	// Wait for the process to exit with a timeout
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(cfg.PID, 0); err != nil {
			log.Info("orphaned lsvd-server stopped", "pid", cfg.PID)
			os.RemoveAll(filepath.Join(dataPath, "outboard", "lsvd-server"))
			return
		}
		time.Sleep(250 * time.Millisecond)
	}

	// Process didn't exit in time, force kill
	log.Warn("lsvd-server did not exit in time, sending SIGKILL", "pid", cfg.PID)
	if err := syscall.Kill(cfg.PID, syscall.SIGKILL); err != nil {
		log.Warn("failed to send SIGKILL to lsvd-server", "pid", cfg.PID, "error", err)
		return
	}

	// Brief wait for SIGKILL to take effect
	time.Sleep(500 * time.Millisecond)
	os.RemoveAll(filepath.Join(dataPath, "outboard", "lsvd-server"))
	log.Info("orphaned lsvd-server force-killed", "pid", cfg.PID)
}

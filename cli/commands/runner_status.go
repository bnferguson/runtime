package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"miren.dev/runtime/pkg/runnerconfig"
)

func RunnerStatus(ctx *Context, opts struct {
	ConfigPath string `long:"config" description:"Path to runner config" default:"/var/lib/miren/runner/config.yaml"`
	DataPath   string `long:"data-path" description:"Path to runner data" default:"/var/lib/miren/runner"`
}) error {
	// Check if runner is configured
	cfg, err := runnerconfig.Load(opts.ConfigPath)
	if err != nil {
		ctx.Printf("Runner:       not configured\n")
		ctx.Printf("  (no config at %s)\n", opts.ConfigPath)
		return fmt.Errorf("runner not configured: %w", err)
	}

	ctx.Printf("Runner ID:    %s\n", cfg.RunnerID)
	ctx.Printf("Coordinator:  %s\n", cfg.CoordinatorAddress)

	// Check containerd
	socketPath := filepath.Join(opts.DataPath, "containerd", "containerd.sock")
	if info, err := os.Stat(socketPath); err == nil && info.Mode()&os.ModeSocket != 0 {
		// Try to find the containerd PID from its pidfile or by checking the socket
		ctx.Printf("Containerd:   running (socket %s)\n", socketPath)
	} else {
		ctx.Printf("Containerd:   not running\n")
	}

	// Check etcd connectivity
	if len(cfg.EtcdEndpoints) > 0 {
		ctx.Printf("Etcd:         %s\n", cfg.EtcdEndpoints)
	}

	// Check flannel subnet
	netdbPath := filepath.Join(opts.DataPath, "net.db")
	if _, err := os.Stat(netdbPath); err == nil {
		ctx.Printf("Network DB:   %s\n", netdbPath)
	}

	// Check if the runner process is running by looking for a pidfile
	pidFile := filepath.Join(opts.DataPath, "runner.pid")
	if data, err := os.ReadFile(pidFile); err == nil {
		var pid int
		if _, err := fmt.Sscanf(string(data), "%d", &pid); err == nil {
			process, err := os.FindProcess(pid)
			if err == nil {
				if err := process.Signal(syscall.Signal(0)); err == nil {
					ctx.Printf("Status:       running (pid %d)\n", pid)
				} else {
					ctx.Printf("Status:       stale pid %d (%s)\n", pid, err)
				}
			}
		}
	} else {
		// No pidfile, check if containerd socket exists as a proxy
		if _, err := os.Stat(socketPath); err == nil {
			ctx.Printf("Status:       running (no pidfile, socket present)\n")
		} else {
			ctx.Printf("Status:       stopped\n")
		}
	}

	// Check sandboxes
	sandboxDir := filepath.Join(opts.DataPath, "containerd", "io.containerd.runtime.v2.task", "miren")
	if entries, err := os.ReadDir(sandboxDir); err == nil {
		ctx.Printf("Sandboxes:    %d running\n", len(entries))
	} else {
		ctx.Printf("Sandboxes:    0 running\n")
	}

	return nil
}

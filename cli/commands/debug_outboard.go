package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"miren.dev/runtime/api/outboard/outboard_v1alpha"
	"miren.dev/runtime/pkg/outboard"
	"miren.dev/runtime/pkg/rpc"
	"miren.dev/runtime/pkg/rpc/standard"
)

// DebugOutboardHealth connects to outboard processes and displays health status.
// If no name is given, it reports health for all outboard processes found.
func DebugOutboardHealth(ctx *Context, opts struct {
	DataPath string `short:"d" long:"data-path" description:"Base miren data directory" default:"/var/lib/miren"`
	Name     string `short:"n" long:"name" description:"Outboard process name (omit for all)"`
}) error {
	outboardBase := filepath.Join(opts.DataPath, "outboard")

	if opts.Name != "" {
		return showOutboardHealth(ctx, outboardBase, opts.Name)
	}

	// List all subdirectories under the outboard base
	entries, err := os.ReadDir(outboardBase)
	if err != nil {
		if os.IsNotExist(err) {
			ctx.Info("No outboard processes found at %s", outboardBase)
			return nil
		}
		return fmt.Errorf("failed to read outboard directory: %w", err)
	}

	found := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		configPath := filepath.Join(outboardBase, entry.Name(), "outboard.json")
		if _, err := os.Stat(configPath); err != nil {
			continue
		}

		if found > 0 {
			ctx.Info("")
		}
		found++

		if err := showOutboardHealth(ctx, outboardBase, entry.Name()); err != nil {
			ctx.Warn("Error checking %s: %v", entry.Name(), err)
		}
	}

	if found == 0 {
		ctx.Info("No outboard processes found at %s", outboardBase)
	}

	return nil
}

func showOutboardHealth(ctx *Context, outboardBase, name string) error {
	configPath := filepath.Join(outboardBase, name, "outboard.json")

	cfg, err := outboard.ReadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to read outboard config at %s: %w", configPath, err)
	}

	if !cfg.Ready {
		ctx.Info("%s: not ready", name)
		return nil
	}

	if cfg.RPCAddr == "" {
		ctx.Info("%s: no RPC address (process may not have started)", name)
		return nil
	}

	rpcCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rs, err := rpc.NewState(rpcCtx,
		rpc.WithBearerToken(cfg.Token),
		rpc.WithSkipVerify,
	)
	if err != nil {
		return fmt.Errorf("failed to create RPC state: %w", err)
	}
	defer rs.Close()

	client, err := rs.Connect(cfg.RPCAddr, "outboard-control")
	if err != nil {
		ctx.Info("%s: unreachable (PID %d, addr %s)", name, cfg.PID, cfg.RPCAddr)
		return nil
	}

	controlClient := outboard_v1alpha.NewOutboardControlClient(client)

	result, err := controlClient.Health(rpcCtx)
	if err != nil {
		ctx.Info("%s: health RPC failed: %v", name, err)
		return nil
	}

	status := result.Status()
	if status == nil {
		ctx.Info("%s: nil status from health RPC", name)
		return nil
	}

	ctx.Info("%s:", name)
	ctx.Info("  Healthy:    %v", status.Healthy())
	ctx.Info("  PID:        %d", status.Pid())
	if status.HasUptime() {
		ctx.Info("  Uptime:     %s", standard.FromDuration(status.Uptime()))
	}
	if status.HasLastError() && status.LastError() != "" {
		ctx.Info("  Last Error: %s", status.LastError())
	}

	return nil
}

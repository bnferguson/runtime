package commands

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"miren.dev/runtime/api/lsvd/lsvd_v1alpha"
	"miren.dev/runtime/pkg/outboard"
	"miren.dev/runtime/pkg/rpc"
	"miren.dev/runtime/pkg/rpc/standard"
)

// connectLsvdDebug connects to the lsvd-server outboard process and returns an LsvdDebugClient.
func connectLsvdDebug(ctx *Context, dataPath string) (*lsvd_v1alpha.LsvdDebugClient, func(), error) {
	configPath := filepath.Join(dataPath, "outboard", "lsvd-server", "outboard.json")

	cfg, err := outboard.ReadConfig(configPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read outboard config at %s: %w", configPath, err)
	}

	if !cfg.Ready {
		return nil, nil, fmt.Errorf("lsvd-server is not ready")
	}

	if cfg.RPCAddr == "" {
		return nil, nil, fmt.Errorf("lsvd-server has no RPC address (process may not have started)")
	}

	rpcCtx, cancel := context.WithTimeout(ctx, 5*time.Second)

	rs, err := rpc.NewState(rpcCtx,
		rpc.WithBearerToken(cfg.Token),
		rpc.WithSkipVerify,
	)
	if err != nil {
		cancel()
		return nil, nil, fmt.Errorf("failed to create RPC state: %w", err)
	}

	client, err := rs.Connect(cfg.RPCAddr, "lsvd-debug")
	if err != nil {
		rs.Close()
		cancel()
		return nil, nil, fmt.Errorf("failed to connect to lsvd-server: %w", err)
	}

	debugClient := lsvd_v1alpha.NewLsvdDebugClient(client)

	cleanup := func() {
		rs.Close()
		cancel()
	}

	return debugClient, cleanup, nil
}

func lsvdFormatBytes(b int64) string {
	if b == 0 {
		return "0"
	}

	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
		tb = gb * 1024
	)

	switch {
	case b >= tb:
		return fmt.Sprintf("%.1f TB", float64(b)/float64(tb))
	case b >= gb:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// DebugLsvdVolumes lists volumes managed by the lsvd-server.
func DebugLsvdVolumes(ctx *Context, opts struct {
	DataPath string `short:"d" long:"data-path" description:"Base miren data directory" default:"/var/lib/miren"`
}) error {
	client, cleanup, err := connectLsvdDebug(ctx, opts.DataPath)
	if err != nil {
		return err
	}
	defer cleanup()

	result, err := client.ListVolumes(ctx)
	if err != nil {
		return fmt.Errorf("listVolumes RPC failed: %w", err)
	}

	volumes := result.Volumes()
	if len(volumes) == 0 {
		ctx.Info("No volumes")
		return nil
	}

	headers := []string{"ENTITY ID", "VOLUME ID", "SIZE", "FS", "REMOTE", "DISK PATH"}
	rows := make([][]string, 0, len(volumes))

	for _, v := range volumes {
		rows = append(rows, []string{
			v.EntityId(),
			v.VolumeId(),
			lsvdFormatBytes(v.SizeBytes()),
			v.Filesystem(),
			fmt.Sprintf("%v", v.RemoteOnly()),
			v.DiskPath(),
		})
	}

	ctx.DisplayTable(headers, rows)
	return nil
}

// DebugLsvdMounts lists mounts managed by the lsvd-server.
func DebugLsvdMounts(ctx *Context, opts struct {
	DataPath string `short:"d" long:"data-path" description:"Base miren data directory" default:"/var/lib/miren"`
}) error {
	client, cleanup, err := connectLsvdDebug(ctx, opts.DataPath)
	if err != nil {
		return err
	}
	defer cleanup()

	result, err := client.ListMounts(ctx)
	if err != nil {
		return fmt.Errorf("listMounts RPC failed: %w", err)
	}

	mounts := result.Mounts()
	if len(mounts) == 0 {
		ctx.Info("No mounts")
		return nil
	}

	headers := []string{"ENTITY ID", "VOLUME ID", "NBD", "MOUNTED", "RO", "DEVICE", "MOUNT PATH"}
	rows := make([][]string, 0, len(mounts))

	for _, m := range mounts {
		rows = append(rows, []string{
			m.EntityId(),
			m.VolumeId(),
			fmt.Sprintf("%d", m.NbdIndex()),
			fmt.Sprintf("%v", m.Mounted()),
			fmt.Sprintf("%v", m.ReadOnly()),
			m.DevicePath(),
			m.MountPath(),
		})
	}

	ctx.DisplayTable(headers, rows)
	return nil
}

// DebugLsvdMetrics shows reconciliation metrics from the lsvd-server.
func DebugLsvdMetrics(ctx *Context, opts struct {
	DataPath string `short:"d" long:"data-path" description:"Base miren data directory" default:"/var/lib/miren"`
}) error {
	client, cleanup, err := connectLsvdDebug(ctx, opts.DataPath)
	if err != nil {
		return err
	}
	defer cleanup()

	result, err := client.GetMetrics(ctx)
	if err != nil {
		return fmt.Errorf("getMetrics RPC failed: %w", err)
	}

	metrics := result.Metrics()
	if metrics == nil {
		return fmt.Errorf("nil metrics from RPC")
	}

	ctx.Info("LSVD Reconciliation Metrics:")
	ctx.Info("  Volume reconciliations: %d", metrics.VolumeReconcileCount())
	ctx.Info("  Volume errors:          %d", metrics.VolumeErrorCount())
	ctx.Info("  Mount reconciliations:  %d", metrics.MountReconcileCount())
	ctx.Info("  Mount errors:           %d", metrics.MountErrorCount())

	if metrics.HasLastVolumeDuration() {
		ctx.Info("  Last volume duration:   %s", standard.FromDuration(metrics.LastVolumeDuration()))
	}
	if metrics.HasLastMountDuration() {
		ctx.Info("  Last mount duration:    %s", standard.FromDuration(metrics.LastMountDuration()))
	}

	return nil
}

// DebugLsvdInfo shows a combined view of lsvd-server volumes, mounts, and metrics.
func DebugLsvdInfo(ctx *Context, opts struct {
	DataPath string `short:"d" long:"data-path" description:"Base miren data directory" default:"/var/lib/miren"`
}) error {
	client, cleanup, err := connectLsvdDebug(ctx, opts.DataPath)
	if err != nil {
		return err
	}
	defer cleanup()

	// Volumes
	volResult, err := client.ListVolumes(ctx)
	if err != nil {
		return fmt.Errorf("listVolumes RPC failed: %w", err)
	}

	volumes := volResult.Volumes()
	ctx.Info("Volumes (%d):", len(volumes))
	if len(volumes) > 0 {
		headers := []string{"ENTITY ID", "VOLUME ID", "SIZE", "FS", "REMOTE", "DISK PATH"}
		rows := make([][]string, 0, len(volumes))
		for _, v := range volumes {
			rows = append(rows, []string{
				v.EntityId(),
				v.VolumeId(),
				lsvdFormatBytes(v.SizeBytes()),
				v.Filesystem(),
				fmt.Sprintf("%v", v.RemoteOnly()),
				v.DiskPath(),
			})
		}
		ctx.DisplayTable(headers, rows)
	}
	fmt.Fprintln(ctx.Stdout)

	// Mounts
	mntResult, err := client.ListMounts(ctx)
	if err != nil {
		return fmt.Errorf("listMounts RPC failed: %w", err)
	}

	mounts := mntResult.Mounts()
	ctx.Info("Mounts (%d):", len(mounts))
	if len(mounts) > 0 {
		headers := []string{"ENTITY ID", "VOLUME ID", "NBD", "MOUNTED", "RO", "DEVICE", "MOUNT PATH"}
		rows := make([][]string, 0, len(mounts))
		for _, m := range mounts {
			rows = append(rows, []string{
				m.EntityId(),
				m.VolumeId(),
				fmt.Sprintf("%d", m.NbdIndex()),
				fmt.Sprintf("%v", m.Mounted()),
				fmt.Sprintf("%v", m.ReadOnly()),
				m.DevicePath(),
				m.MountPath(),
			})
		}
		ctx.DisplayTable(headers, rows)
	}
	fmt.Fprintln(ctx.Stdout)

	// Metrics
	metricsResult, err := client.GetMetrics(ctx)
	if err != nil {
		return fmt.Errorf("getMetrics RPC failed: %w", err)
	}

	metrics := metricsResult.Metrics()
	if metrics != nil {
		ctx.Info("Metrics:")
		ctx.Info("  Volume reconciliations: %d (%d errors)", metrics.VolumeReconcileCount(), metrics.VolumeErrorCount())
		ctx.Info("  Mount reconciliations:  %d (%d errors)", metrics.MountReconcileCount(), metrics.MountErrorCount())
		if metrics.HasLastVolumeDuration() {
			ctx.Info("  Last volume duration:   %s", standard.FromDuration(metrics.LastVolumeDuration()))
		}
		if metrics.HasLastMountDuration() {
			ctx.Info("  Last mount duration:    %s", standard.FromDuration(metrics.LastMountDuration()))
		}
	}

	return nil
}

package commands

import (
	"fmt"
	"os"
	"time"

	"miren.dev/runtime/api/entityserver/entityserver_v1alpha"
	"miren.dev/runtime/pkg/snapshot"
)

// DiskBackup backs up a disk to a compressed snapshot file.
func DiskBackup(ctx *Context, opts struct {
	ConfigCentric
	Name     string `short:"n" long:"name" description:"Disk name to backup" required:"true"`
	Output   string `short:"o" long:"output" description:"Output snapshot path (default: DISK-YYYYMMDD-HHMMSS.miren.zst)"`
	DataPath string `long:"data-path" description:"Path to miren data directory" default:"/var/lib/miren"`
}) (retErr error) {
	client, err := ctx.RPCClient("entities")
	if err != nil {
		return err
	}

	eac := entityserver_v1alpha.NewEntityAccessClient(client)
	resolver := newEntityDiskResolver(eac)

	target, err := snapshot.PrepareBackup(ctx, resolver, opts.Name, opts.DataPath)
	if err != nil {
		return err
	}

	imgInfo, err := os.Stat(target.ImagePath)
	if err != nil {
		return fmt.Errorf("disk image not found at %s: %w", target.ImagePath, err)
	}

	if target.IsAttached {
		ctx.Warn("Disk is currently attached — backup will be crash-consistent")
	}

	outputPath := opts.Output
	if outputPath == "" {
		outputPath = fmt.Sprintf("%s-%s.miren.zst", opts.Name, time.Now().Format("20060102-150405"))
	}

	ctx.Info("Backing up disk %q (%s)", opts.Name, formatBytes(imgInfo.Size()))
	ctx.Info("Image: %s", target.ImagePath)
	ctx.Info("Output: %s", outputPath)

	start := time.Now()

	imgFile, err := os.Open(target.ImagePath)
	if err != nil {
		return fmt.Errorf("opening image file: %w", err)
	}
	defer imgFile.Close()

	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer func() {
		if cerr := outFile.Close(); cerr != nil && retErr == nil {
			retErr = fmt.Errorf("closing output file: %w", cerr)
		}
		if retErr != nil {
			os.Remove(outputPath)
		}
	}()

	ctx.Info("Computing checksum and compressing...")

	checksum, err := snapshot.Backup(outFile, imgFile, opts.Name, imgInfo.Size(), target.Filesystem)
	if err != nil {
		return err
	}

	outInfo, err := outFile.Stat()
	if err != nil {
		return fmt.Errorf("stat output file: %w", err)
	}

	duration := time.Since(start)
	ratio := float64(outInfo.Size()) / float64(imgInfo.Size()) * 100

	ctx.Info("Backup complete")
	ctx.Info("  Original size:   %s", formatBytes(imgInfo.Size()))
	ctx.Info("  Compressed size: %s (%.1f%%)", formatBytes(outInfo.Size()), ratio)
	ctx.Info("  Checksum:        %s", checksum)
	ctx.Info("  Duration:        %s", duration.Truncate(time.Millisecond))
	ctx.Info("  Snapshot:        %s", outputPath)

	return nil
}

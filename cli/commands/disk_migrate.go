package commands

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"miren.dev/runtime/lsvd"
)

// DiskMigrate reads an LSVD volume and writes a raw disk image for universal mode.
func DiskMigrate(ctx *Context, opts struct {
	DataPath   string `long:"data-path" description:"Path to LSVD data directory" required:"true"`
	VolumeName string `long:"volume-name" description:"LSVD volume name" required:"true"`
	Output     string `short:"o" long:"output" description:"Output raw disk image path" required:"true"`
}) error {
	log := slog.Default()

	disk, err := lsvd.NewDisk(context.Background(), log, opts.DataPath,
		lsvd.WithVolumeName(opts.VolumeName),
		lsvd.ReadOnly(),
		lsvd.AutoCreate(false),
	)
	if err != nil {
		return fmt.Errorf("opening LSVD volume %q: %w", opts.VolumeName, err)
	}
	defer disk.Close(context.Background()) //nolint:errcheck

	totalSize := disk.Size()
	if totalSize == 0 {
		return fmt.Errorf("LSVD volume %q has zero size", opts.VolumeName)
	}

	totalBlocks := totalSize / int64(lsvd.BlockSize)

	out, err := os.Create(opts.Output)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer out.Close()

	// Pre-allocate sparse file
	if err := out.Truncate(totalSize); err != nil {
		return fmt.Errorf("truncating output file to %d bytes: %w", totalSize, err)
	}

	// Read in chunks of 1024 blocks (4MB)
	const chunkBlocks = 1024
	zeros := make([]byte, chunkBlocks*lsvd.BlockSize)

	lsvdCtx := lsvd.NewContext(context.Background())
	defer lsvdCtx.Close()

	var written int64

	for lba := int64(0); lba < totalBlocks; lba += chunkBlocks {
		blocks := chunkBlocks
		if lba+int64(blocks) > totalBlocks {
			blocks = int(totalBlocks - lba)
		}

		lsvdCtx.Reset()

		data, err := disk.ReadExtent(lsvdCtx, lsvd.Extent{
			LBA:    lsvd.LBA(lba),
			Blocks: uint32(blocks),
		})
		if err != nil {
			return fmt.Errorf("reading extent at LBA %d: %w", lba, err)
		}

		raw := data.ReadData()

		// Skip all-zero chunks to preserve sparseness
		if isZero(raw, zeros[:len(raw)]) {
			continue
		}

		offset := lba * int64(lsvd.BlockSize)
		if _, err := out.WriteAt(raw, offset); err != nil {
			return fmt.Errorf("writing at offset %d: %w", offset, err)
		}
		written += int64(len(raw))
	}

	ctx.Info("Migrated %s: %d bytes total, %d bytes written (%.1f%% sparse)",
		opts.VolumeName, totalSize, written,
		float64(totalSize-written)/float64(totalSize)*100)

	return nil
}

// isZero checks if data is all zeros by comparing against a pre-allocated zero buffer.
func isZero(data, zeros []byte) bool {
	for i := range data {
		if data[i] != zeros[i] {
			return false
		}
	}
	return true
}

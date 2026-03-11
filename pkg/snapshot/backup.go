package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"time"

	"github.com/klauspost/compress/zstd"
)

// Backup reads the disk image from src, computes its SHA-256 checksum,
// and writes a complete snapshot (header + zstd-compressed data) to dst.
// It uses a TeeReader to hash and compress in a single pass, then seeks
// back to rewrite the header with the final checksum.
// It returns the checksum of the uncompressed image data.
func Backup(dst io.WriteSeeker, src io.Reader, name string, sizeBytes int64, filesystem string) (checksum string, err error) {
	// Write a placeholder header (checksum will be filled in after the data pass)
	meta := &Meta{
		Name:       name,
		SizeBytes:  sizeBytes,
		Filesystem: filesystem,
		Timestamp:  time.Now().UTC(),
		Checksum:   hex.EncodeToString(make([]byte, sha256.Size)),
		Version:    FormatVersion,
	}

	if err := WriteHeader(dst, meta); err != nil {
		return "", fmt.Errorf("writing snapshot header: %w", err)
	}

	// Record where compressed data starts so we can compute header size
	dataStart, err := dst.Seek(0, io.SeekCurrent)
	if err != nil {
		return "", fmt.Errorf("getting data start offset: %w", err)
	}

	// Single pass: TeeReader feeds source data to both hasher and compressor
	hasher := sha256.New()
	tee := io.TeeReader(src, hasher)

	encoder, err := zstd.NewWriter(dst)
	if err != nil {
		return "", fmt.Errorf("creating zstd encoder: %w", err)
	}

	buf := make([]byte, 4*1024*1024) // 4MB buffer
	if _, err = io.CopyBuffer(encoder, tee, buf); err != nil {
		encoder.Close()
		return "", fmt.Errorf("compressing image data: %w", err)
	}

	if err = encoder.Close(); err != nil {
		return "", fmt.Errorf("finalizing compression: %w", err)
	}

	checksum = hex.EncodeToString(hasher.Sum(nil))

	// Seek back and rewrite the header with the real checksum
	meta.Checksum = checksum
	if _, err := dst.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("seeking to rewrite header: %w", err)
	}
	if err := WriteHeader(dst, meta); err != nil {
		return "", fmt.Errorf("rewriting snapshot header: %w", err)
	}

	// Verify the header didn't change size (checksum is fixed-length hex)
	newDataStart, err := dst.Seek(0, io.SeekCurrent)
	if err != nil {
		return "", fmt.Errorf("verifying header size: %w", err)
	}
	if newDataStart != dataStart {
		return "", fmt.Errorf("header size changed after checksum update (%d -> %d)", dataStart, newDataStart)
	}

	// Seek to end so caller sees correct file position
	if _, err := dst.Seek(0, io.SeekEnd); err != nil {
		return "", fmt.Errorf("seeking to end: %w", err)
	}

	return checksum, nil
}

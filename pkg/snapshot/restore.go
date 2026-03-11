package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/klauspost/compress/zstd"
)

// RestoreImage reads zstd-compressed data from src, decompresses it, and
// writes to dst using sparse-aware writes. The src stream should be
// positioned after the snapshot header (i.e., at the start of compressed
// data). The SHA-256 checksum of the decompressed data is verified against
// meta.Checksum.
func RestoreImage(dst *os.File, src io.Reader, meta *Meta) error {
	decoder, err := zstd.NewReader(src)
	if err != nil {
		return fmt.Errorf("creating zstd decoder: %w", err)
	}
	defer decoder.Close()

	hasher := sha256.New()
	buf := make([]byte, 4*1024*1024) // 4MB buffer

	for {
		n, readErr := io.ReadFull(decoder, buf)
		if n > 0 {
			chunk := buf[:n]
			hasher.Write(chunk)

			if err := sparseWrite(dst, chunk); err != nil {
				return fmt.Errorf("writing image data: %w", err)
			}
		}
		if readErr != nil {
			if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
				break
			}
			return fmt.Errorf("decompressing image data: %w", readErr)
		}
	}

	checksum := hex.EncodeToString(hasher.Sum(nil))
	if checksum != meta.Checksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", meta.Checksum, checksum)
	}

	return nil
}

// sparseWrite writes data to f, seeking past block-aligned leading and
// trailing zero regions to keep the file sparse.
func sparseWrite(f *os.File, data []byte) error {
	const blockSize = 4096

	// Find first non-zero block
	start := 0
	for start+blockSize <= len(data) {
		if !allZero(data[start : start+blockSize]) {
			break
		}
		start += blockSize
	}

	// All zeros — just seek past
	if start >= len(data) {
		_, err := f.Seek(int64(len(data)), io.SeekCurrent)
		return err
	}

	// Find last non-zero block (scan backwards)
	end := len(data)
	for end-blockSize >= start {
		if !allZero(data[end-blockSize : end]) {
			break
		}
		end -= blockSize
	}

	// Seek past leading zeros
	if start > 0 {
		if _, err := f.Seek(int64(start), io.SeekCurrent); err != nil {
			return err
		}
	}

	// Write the non-zero interior
	if _, err := f.Write(data[start:end]); err != nil {
		return err
	}

	// Seek past trailing zeros
	if end < len(data) {
		if _, err := f.Seek(int64(len(data)-end), io.SeekCurrent); err != nil {
			return err
		}
	}

	return nil
}

func allZero(b []byte) bool {
	for _, v := range b {
		if v != 0 {
			return false
		}
	}
	return true
}

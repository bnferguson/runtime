package snapshot

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bufferSeeker wraps bytes.Buffer to implement io.WriteSeeker for tests.
type bufferSeeker struct {
	buf []byte
	pos int
}

func (b *bufferSeeker) Write(p []byte) (int, error) {
	end := b.pos + len(p)
	if end > len(b.buf) {
		b.buf = append(b.buf, make([]byte, end-len(b.buf))...)
	}
	copy(b.buf[b.pos:], p)
	b.pos = end
	return len(p), nil
}

func (b *bufferSeeker) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		b.pos = int(offset)
	case io.SeekCurrent:
		b.pos += int(offset)
	case io.SeekEnd:
		b.pos = len(b.buf) + int(offset)
	}
	return int64(b.pos), nil
}

func (b *bufferSeeker) Bytes() []byte { return b.buf }

func TestBackup(t *testing.T) {
	t.Run("produces valid snapshot", func(t *testing.T) {
		srcData := make([]byte, 64*1024)
		for i := range srcData {
			srcData[i] = byte(i % 251)
		}

		dst := &bufferSeeker{}
		checksum, err := Backup(dst, bytes.NewReader(srcData), "test-disk", int64(len(srcData)), "ext4")
		require.NoError(t, err)

		hasher := sha256.New()
		hasher.Write(srcData)
		expectedChecksum := hex.EncodeToString(hasher.Sum(nil))
		assert.Equal(t, expectedChecksum, checksum)

		// Read back header
		reader := bytes.NewReader(dst.Bytes())
		meta, err := ReadHeader(reader)
		require.NoError(t, err)
		assert.Equal(t, "test-disk", meta.Name)
		assert.Equal(t, int64(len(srcData)), meta.SizeBytes)
		assert.Equal(t, "ext4", meta.Filesystem)
		assert.Equal(t, expectedChecksum, meta.Checksum)
		assert.Equal(t, FormatVersion, meta.Version)

		// Decompress and verify data matches
		decoder, err := zstd.NewReader(reader)
		require.NoError(t, err)
		defer decoder.Close()

		restored, err := io.ReadAll(decoder)
		require.NoError(t, err)
		assert.Equal(t, srcData, restored)
	})

	t.Run("checksum computed correctly", func(t *testing.T) {
		srcData := []byte("hello world, this is a disk image")

		dst := &bufferSeeker{}
		checksum, err := Backup(dst, bytes.NewReader(srcData), "chk", int64(len(srcData)), "xfs")
		require.NoError(t, err)

		hasher := sha256.New()
		hasher.Write(srcData)
		assert.Equal(t, hex.EncodeToString(hasher.Sum(nil)), checksum)
	})

	t.Run("handles empty input", func(t *testing.T) {
		dst := &bufferSeeker{}
		checksum, err := Backup(dst, bytes.NewReader(nil), "empty", 0, "ext4")
		require.NoError(t, err)

		hasher := sha256.New()
		assert.Equal(t, hex.EncodeToString(hasher.Sum(nil)), checksum)

		// Snapshot should still have a valid header
		reader := bytes.NewReader(dst.Bytes())
		meta, err := ReadHeader(reader)
		require.NoError(t, err)
		assert.Equal(t, "empty", meta.Name)
		assert.Equal(t, int64(0), meta.SizeBytes)
	})

	t.Run("preserves metadata fields", func(t *testing.T) {
		srcData := make([]byte, 1024)

		dst := &bufferSeeker{}
		_, err := Backup(dst, bytes.NewReader(srcData), "myapp-db", 1024, "btrfs")
		require.NoError(t, err)

		reader := bytes.NewReader(dst.Bytes())
		meta, err := ReadHeader(reader)
		require.NoError(t, err)
		assert.Equal(t, "myapp-db", meta.Name)
		assert.Equal(t, int64(1024), meta.SizeBytes)
		assert.Equal(t, "btrfs", meta.Filesystem)
		assert.False(t, meta.Timestamp.IsZero())
	})
}

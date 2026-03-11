package snapshot

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHeaderRoundtrip(t *testing.T) {
	ts := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	original := &Meta{
		Name:       "myapp-db",
		SizeBytes:  10737418240,
		Filesystem: "ext4",
		Timestamp:  ts,
		Checksum:   "abc123def456",
		Version:    1,
	}

	var buf bytes.Buffer
	require.NoError(t, WriteHeader(&buf, original))

	got, err := ReadHeader(&buf)
	require.NoError(t, err)

	assert.Equal(t, original.Name, got.Name)
	assert.Equal(t, original.SizeBytes, got.SizeBytes)
	assert.Equal(t, original.Filesystem, got.Filesystem)
	assert.True(t, original.Timestamp.Equal(got.Timestamp))
	assert.Equal(t, original.Checksum, got.Checksum)
	assert.Equal(t, original.Version, got.Version)
}

func TestInvalidMagic(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte("NOPE"))

	_, err := ReadHeader(&buf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid magic")
}

func TestUnsupportedVersion(t *testing.T) {
	var buf bytes.Buffer
	buf.Write(Magic[:])
	binary.Write(&buf, binary.BigEndian, uint32(99))

	_, err := ReadHeader(&buf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported format version")
}

func TestTruncatedMagic(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte("MI"))

	_, err := ReadHeader(&buf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading magic")
}

func TestTruncatedVersion(t *testing.T) {
	var buf bytes.Buffer
	buf.Write(Magic[:])
	buf.Write([]byte{0x00}) // only 1 byte of the 4-byte version

	_, err := ReadHeader(&buf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading version")
}

func TestTruncatedMetadata(t *testing.T) {
	var buf bytes.Buffer
	buf.Write(Magic[:])
	binary.Write(&buf, binary.BigEndian, FormatVersion)
	binary.Write(&buf, binary.BigEndian, uint32(100)) // claims 100 bytes
	buf.Write([]byte("short"))                        // only 5 bytes

	_, err := ReadHeader(&buf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading metadata")
}

func TestMetadataTooLarge(t *testing.T) {
	var buf bytes.Buffer
	buf.Write(Magic[:])
	binary.Write(&buf, binary.BigEndian, FormatVersion)
	binary.Write(&buf, binary.BigEndian, uint32(2<<20)) // 2MB, over the 1MB limit

	_, err := ReadHeader(&buf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "metadata too large")
}

func TestInvalidJSON(t *testing.T) {
	var buf bytes.Buffer
	buf.Write(Magic[:])
	binary.Write(&buf, binary.BigEndian, FormatVersion)
	badJSON := []byte("{not valid json")
	binary.Write(&buf, binary.BigEndian, uint32(len(badJSON)))
	buf.Write(badJSON)

	_, err := ReadHeader(&buf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshaling metadata")
}

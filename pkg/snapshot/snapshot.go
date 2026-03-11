package snapshot

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// Magic is the file magic for miren snapshot files.
var Magic = [4]byte{'M', 'I', 'R', 'N'}

// FormatVersion is the current snapshot format version.
const FormatVersion uint32 = 1

// Meta holds the metadata stored in a snapshot header.
type Meta struct {
	Name       string    `json:"name"`
	SizeBytes  int64     `json:"size_bytes"`
	Filesystem string    `json:"filesystem"`
	Timestamp  time.Time `json:"timestamp"`
	Checksum   string    `json:"checksum"` // SHA-256 hex
	Version    uint32    `json:"version"`
}

// WriteHeader writes the snapshot header (magic + version + length-prefixed JSON metadata) to w.
func WriteHeader(w io.Writer, meta *Meta) error {
	// Write magic
	if _, err := w.Write(Magic[:]); err != nil {
		return fmt.Errorf("writing magic: %w", err)
	}

	// Write format version
	if err := binary.Write(w, binary.BigEndian, FormatVersion); err != nil {
		return fmt.Errorf("writing version: %w", err)
	}

	// Marshal metadata
	data, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}

	// Write metadata length
	if err := binary.Write(w, binary.BigEndian, uint32(len(data))); err != nil {
		return fmt.Errorf("writing metadata length: %w", err)
	}

	// Write metadata
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("writing metadata: %w", err)
	}

	return nil
}

// ReadHeader reads and validates the snapshot header from r, returning the metadata.
func ReadHeader(r io.Reader) (*Meta, error) {
	// Read and validate magic
	var magic [4]byte
	if _, err := io.ReadFull(r, magic[:]); err != nil {
		return nil, fmt.Errorf("reading magic: %w", err)
	}
	if magic != Magic {
		return nil, fmt.Errorf("invalid magic: expected %q, got %q", Magic, magic)
	}

	// Read and validate version
	var version uint32
	if err := binary.Read(r, binary.BigEndian, &version); err != nil {
		return nil, fmt.Errorf("reading version: %w", err)
	}
	if version != FormatVersion {
		return nil, fmt.Errorf("unsupported format version: %d (expected %d)", version, FormatVersion)
	}

	// Read metadata length
	var metaLen uint32
	if err := binary.Read(r, binary.BigEndian, &metaLen); err != nil {
		return nil, fmt.Errorf("reading metadata length: %w", err)
	}

	// Sanity check on metadata length (max 1MB)
	if metaLen > 1<<20 {
		return nil, fmt.Errorf("metadata too large: %d bytes", metaLen)
	}

	// Read metadata
	metaBytes := make([]byte, metaLen)
	if _, err := io.ReadFull(r, metaBytes); err != nil {
		return nil, fmt.Errorf("reading metadata: %w", err)
	}

	var meta Meta
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		return nil, fmt.Errorf("unmarshaling metadata: %w", err)
	}

	return &meta, nil
}

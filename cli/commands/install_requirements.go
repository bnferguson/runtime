package commands

// Shared system-requirement thresholds for the miren server, used by both the
// native install (server install, Linux-only) and the containerized install
// (server container install, cross-platform). They live here, untagged, so the
// cross-platform container path can reference them too — the native path's own
// file is Linux-tagged.
const (
	minMemoryBytes         = 4 * 1024 * 1024 * 1024 // 4 GB
	recommendedMemoryBytes = 8 * 1024 * 1024 * 1024 // 8 GB
)

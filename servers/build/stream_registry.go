package build

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"miren.dev/runtime/pkg/tarx"
)

// ErrStreamUnavailable is returned by Stage when neither an active reader
// nor a previously staged path exists for the given stream ID. This is the
// expected failure when recovering a saga whose tar stream was lost in a
// crash before the first stage attempt.
var ErrStreamUnavailable = errors.New("stream unavailable")

// StreamRegistry bridges non-serializable tar streams into the saga world by
// staging them to durable filesystem paths. The pattern, per RFD-35:
//
//  1. Entry point registers an io.Reader with a generated stream ID and
//     starts a saga carrying only the ID.
//  2. The first saga action calls Stage to read the registered stream and
//     extract its tar contents to a fresh subdirectory.
//  3. If the saga later resumes after a crash, the stream is gone but the
//     staged path is durable — Stage returns the existing path without
//     touching any reader.
//
// Stage and Cleanup are idempotent. Cleanup is safe to call from action
// compensation (on failure) and from a terminal saga step (on success);
// running it twice is a no-op.
type StreamRegistry struct {
	baseDir string
	log     *slog.Logger

	mu      sync.Mutex
	streams map[string]io.Reader
	staged  map[string]string
}

// NewStreamRegistry creates a registry that stages streams under baseDir.
// Each registered stream gets its own subdirectory so Cleanup is a single
// RemoveAll.
func NewStreamRegistry(baseDir string, log *slog.Logger) *StreamRegistry {
	if log == nil {
		log = slog.Default()
	}
	return &StreamRegistry{
		baseDir: baseDir,
		log:     log.With("component", "stream-registry"),
		streams: make(map[string]io.Reader),
		staged:  make(map[string]string),
	}
}

// Register associates an io.Reader with a stream ID. The reader is held
// in memory until Stage consumes it or Cleanup drops it. Re-registering an
// ID with an active reader replaces it; re-registering after staging is a
// no-op so recovery flows don't accidentally restart staging.
func (s *StreamRegistry) Register(streamID string, r io.Reader) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, alreadyStaged := s.staged[streamID]; alreadyStaged {
		return
	}
	s.streams[streamID] = r
}

// Stage extracts the registered stream's tar contents to a per-stream
// subdirectory and returns the path. On recovery, if the original reader
// is gone but a staged path exists, returns that path unchanged. Returns
// ErrStreamUnavailable when neither is available.
func (s *StreamRegistry) Stage(streamID string) (string, error) {
	s.mu.Lock()
	if path, ok := s.staged[streamID]; ok {
		s.mu.Unlock()
		s.log.Debug("stream already staged", "id", streamID, "path", path)
		return path, nil
	}

	reader, ok := s.streams[streamID]
	if !ok {
		s.mu.Unlock()
		return "", fmt.Errorf("%w: %s", ErrStreamUnavailable, streamID)
	}
	// Remove the reader from the map so two concurrent Stage callers
	// can't both consume it. If MkdirTemp fails the reader is
	// untouched and we can safely restore it for a retry; TarFS failure
	// past that point has consumed bytes and isn't recoverable.
	delete(s.streams, streamID)
	s.mu.Unlock()

	path, err := os.MkdirTemp(s.baseDir, "stream-"+streamID+"-")
	if err != nil {
		s.mu.Lock()
		s.streams[streamID] = reader
		s.mu.Unlock()
		return "", fmt.Errorf("creating staging dir for %s: %w", streamID, err)
	}

	if _, err := tarx.TarFS(reader, path); err != nil {
		// Best-effort cleanup; the partial directory is small and the
		// outer saga will fail the action anyway.
		_ = os.RemoveAll(path)
		return "", fmt.Errorf("extracting tar for %s: %w", streamID, err)
	}

	s.mu.Lock()
	s.staged[streamID] = path
	s.mu.Unlock()

	s.log.Debug("staged stream", "id", streamID, "path", path)
	return path, nil
}

// StagedPath returns the staged directory if Stage previously succeeded
// for this ID. Useful for diagnostics and tests; the saga path itself
// flows through Stage's return value.
func (s *StreamRegistry) StagedPath(streamID string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	path, ok := s.staged[streamID]
	return path, ok
}

// MarkStaged records an externally-prepared path as already staged for
// streamID. Used by BuildFromPrepared, where the source is already on
// disk from a PrepareUpload session and the saga's receive-tar action
// just needs to discover that path. Refuses to overwrite an existing
// entry: re-marking is silently dropped (with a warn-log) so a stale
// session ID can't blow away a legitimately-staged directory and leak
// it on disk.
func (s *StreamRegistry) MarkStaged(streamID, path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.streams, streamID)
	if existing, ok := s.staged[streamID]; ok {
		s.log.Warn("MarkStaged called for already-staged id",
			"id", streamID, "existing", existing, "new", path)
		return
	}
	s.staged[streamID] = path
}

// Cleanup removes any staged directory and forgets the stream ID.
// Idempotent: safe to call from both action compensation (failure path)
// and a terminal cleanup action (success path), and safe to call when
// nothing was ever staged. Keeps the registry entry until RemoveAll
// succeeds so a transient filesystem error doesn't leak the path —
// the next call can retry with the same ID.
func (s *StreamRegistry) Cleanup(streamID string) error {
	s.mu.Lock()
	path, hadStaged := s.staged[streamID]
	delete(s.streams, streamID)
	s.mu.Unlock()

	if !hadStaged {
		return nil
	}

	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("removing staged dir %s: %w", path, err)
	}

	s.mu.Lock()
	delete(s.staged, streamID)
	s.mu.Unlock()

	s.log.Debug("cleaned up staged stream", "id", streamID, "path", path)
	return nil
}

// stagedFileExists is used by tests to confirm the staging directory was
// actually written and later removed. Kept package-private; saga code uses
// the path returned from Stage directly.
func stagedFileExists(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}

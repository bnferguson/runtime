package outboard

import (
	"log/slog"
	"os"
	"syscall"
)

// createFIFO creates a named pipe (FIFO) at the given path.
// If the path already exists, it is removed first.
func createFIFO(path string) error {
	os.Remove(path)
	return syscall.Mkfifo(path, 0600)
}

// forwardFIFO opens a FIFO for reading and replays slog JSON records through
// the logger's handler with full fidelity. It blocks until the FIFO is opened
// for writing by another process, then reads until EOF. The done channel is
// closed when forwarding finishes.
func forwardFIFO(path string, logger *slog.Logger, done chan struct{}) {
	defer close(done)

	f, err := os.Open(path)
	if err != nil {
		logger.Warn("failed to open FIFO for reading", "path", path, "error", err)
		return
	}
	defer f.Close()

	replay := newSlogReplay(logger.Handler())
	replay.Forward(f)
}

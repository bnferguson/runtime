package observability

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

// mockLogWriter captures WriteEntry calls for testing.
type mockLogWriter struct {
	entries []struct {
		entity string
		entry  LogEntry
	}
}

func (m *mockLogWriter) WriteEntry(entity string, le LogEntry) error {
	m.entries = append(m.entries, struct {
		entity string
		entry  LogEntry
	}{entity, le})
	return nil
}

func TestSystemLogHandler(t *testing.T) {
	writer := &mockLogWriter{}
	inner := slog.NewTextHandler(discardWriter{}, &slog.HandlerOptions{Level: slog.LevelDebug})

	handler := NewSystemLogHandler(inner, writer)
	logger := slog.New(handler)

	t.Run("info message maps to stdout", func(t *testing.T) {
		writer.entries = nil
		logger.Info("server started", "port", "8443")

		if len(writer.entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(writer.entries))
		}

		e := writer.entries[0]
		if e.entity != SystemLogEntityID {
			t.Errorf("entity = %q, want %q", e.entity, SystemLogEntityID)
		}
		if e.entry.Stream != Stdout {
			t.Errorf("stream = %q, want %q", e.entry.Stream, Stdout)
		}
		if e.entry.Body != "server started" {
			t.Errorf("body = %q, want %q", e.entry.Body, "server started")
		}
		if e.entry.Attributes["source"] != "system" {
			t.Errorf("source attr = %q, want %q", e.entry.Attributes["source"], "system")
		}
	})

	t.Run("error message maps to stderr", func(t *testing.T) {
		writer.entries = nil
		logger.Error("connection failed")

		if len(writer.entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(writer.entries))
		}

		if writer.entries[0].entry.Stream != Stderr {
			t.Errorf("stream = %q, want %q", writer.entries[0].entry.Stream, Stderr)
		}
	})

	t.Run("WithAttrs preserves attributes", func(t *testing.T) {
		writer.entries = nil
		moduleLogger := slog.New(handler.WithAttrs([]slog.Attr{
			slog.String("module", "etcd"),
		}))
		moduleLogger.Info("peer connected")

		if len(writer.entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(writer.entries))
		}

		if writer.entries[0].entry.Attributes["module"] != "etcd" {
			t.Errorf("module attr = %q, want %q", writer.entries[0].entry.Attributes["module"], "etcd")
		}
	})

	t.Run("timestamp is set", func(t *testing.T) {
		writer.entries = nil
		before := time.Now().UTC()
		logger.Info("test")
		after := time.Now().UTC()

		if len(writer.entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(writer.entries))
		}

		ts := writer.entries[0].entry.Timestamp
		if ts.Before(before) || ts.After(after) {
			t.Errorf("timestamp %v not between %v and %v", ts, before, after)
		}
	})

	t.Run("reserved attr keys are remapped", func(t *testing.T) {
		writer.entries = nil
		logger.Info("Processing event", "entity", "app/myapp", "rev", "42")

		if len(writer.entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(writer.entries))
		}

		attrs := writer.entries[0].entry.Attributes
		// "entity" collides with VictoriaLogs routing field, should be remapped
		if _, ok := attrs["entity"]; ok {
			t.Error("reserved key 'entity' was not remapped")
		}
		if attrs["log_entity"] != "app/myapp" {
			t.Errorf("log_entity = %q, want %q", attrs["log_entity"], "app/myapp")
		}
		// Non-reserved keys should pass through unchanged
		if attrs["rev"] != "42" {
			t.Errorf("rev = %q, want %q", attrs["rev"], "42")
		}
	})

	t.Run("Enabled delegates to inner handler", func(t *testing.T) {
		if !handler.Enabled(context.Background(), slog.LevelDebug) {
			t.Error("expected debug to be enabled")
		}
	})
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

package outboard

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// collectHandler collects slog records for inspection.
type collectHandler struct {
	records []slog.Record
}

func (h *collectHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *collectHandler) Handle(_ context.Context, r slog.Record) error {
	h.records = append(h.records, r)
	return nil
}

func (h *collectHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *collectHandler) WithGroup(_ string) slog.Handler       { return h }

func TestSlogReplayBasic(t *testing.T) {
	h := &collectHandler{}
	replay := newSlogReplay(h)

	input := `{"time":"2025-01-15T10:30:00Z","level":"INFO","msg":"hello world","key":"value"}` + "\n"
	replay.Forward(strings.NewReader(input))

	require.Len(t, h.records, 1)
	rec := h.records[0]

	assert.Equal(t, "hello world", rec.Message)
	assert.Equal(t, slog.LevelInfo, rec.Level)
	assert.Equal(t, time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC), rec.Time)

	var attrs []slog.Attr
	rec.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, a)
		return true
	})
	require.Len(t, attrs, 1)
	assert.Equal(t, "key", attrs[0].Key)
	assert.Equal(t, "value", attrs[0].Value.String())
}

func TestSlogReplayLevels(t *testing.T) {
	tests := []struct {
		level    string
		expected slog.Level
	}{
		{"DEBUG", slog.LevelDebug},
		{"INFO", slog.LevelInfo},
		{"WARN", slog.LevelWarn},
		{"ERROR", slog.LevelError},
		{"DEBUG-4", slog.LevelDebug - 4},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			h := &collectHandler{}
			replay := newSlogReplay(h)

			input := `{"time":"2025-01-15T10:30:00Z","level":"` + tt.level + `","msg":"test"}` + "\n"
			replay.Forward(strings.NewReader(input))

			require.Len(t, h.records, 1)
			assert.Equal(t, tt.expected, h.records[0].Level)
		})
	}
}

func TestSlogReplayTypes(t *testing.T) {
	h := &collectHandler{}
	replay := newSlogReplay(h)

	input := `{"time":"2025-01-15T10:30:00Z","level":"INFO","msg":"types","str":"hello","num":42,"float":3.14,"bool":true,"null_val":null}` + "\n"
	replay.Forward(strings.NewReader(input))

	require.Len(t, h.records, 1)

	attrs := map[string]slog.Value{}
	h.records[0].Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value
		return true
	})

	assert.Equal(t, "hello", attrs["str"].String())
	assert.Equal(t, int64(42), attrs["num"].Int64())
	assert.Equal(t, 3.14, attrs["float"].Float64())
	assert.Equal(t, true, attrs["bool"].Bool())
	assert.Equal(t, slog.AnyValue(nil), attrs["null_val"])
}

func TestSlogReplayNestedGroup(t *testing.T) {
	h := &collectHandler{}
	replay := newSlogReplay(h)

	input := `{"time":"2025-01-15T10:30:00Z","level":"INFO","msg":"grouped","outer":{"inner":"deep","count":5}}` + "\n"
	replay.Forward(strings.NewReader(input))

	require.Len(t, h.records, 1)

	var attrs []slog.Attr
	h.records[0].Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, a)
		return true
	})

	require.Len(t, attrs, 1)
	assert.Equal(t, "outer", attrs[0].Key)
	assert.Equal(t, slog.KindGroup, attrs[0].Value.Kind())

	group := attrs[0].Value.Group()
	groupMap := map[string]slog.Value{}
	for _, a := range group {
		groupMap[a.Key] = a.Value
	}
	assert.Equal(t, "deep", groupMap["inner"].String())
	assert.Equal(t, int64(5), groupMap["count"].Int64())
}

func TestSlogReplaySource(t *testing.T) {
	h := &collectHandler{}
	replay := newSlogReplay(h)

	input := `{"time":"2025-01-15T10:30:00Z","level":"INFO","msg":"with source","source":{"function":"main.run","file":"/app/main.go","line":42}}` + "\n"
	replay.Forward(strings.NewReader(input))

	require.Len(t, h.records, 1)

	var attrs []slog.Attr
	h.records[0].Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, a)
		return true
	})

	require.Len(t, attrs, 1)
	assert.Equal(t, "source", attrs[0].Key)
	assert.Equal(t, slog.KindGroup, attrs[0].Value.Kind())

	group := attrs[0].Value.Group()
	groupMap := map[string]slog.Value{}
	for _, a := range group {
		groupMap[a.Key] = a.Value
	}
	assert.Equal(t, "main.run", groupMap["function"].String())
	assert.Equal(t, "/app/main.go", groupMap["file"].String())
	assert.Equal(t, int64(42), groupMap["line"].Int64())
}

func TestSlogReplayNonJSON(t *testing.T) {
	h := &collectHandler{}
	replay := newSlogReplay(h)

	input := "this is not json\n"
	replay.Forward(strings.NewReader(input))

	require.Len(t, h.records, 1)
	assert.Equal(t, "this is not json", h.records[0].Message)
	assert.Equal(t, slog.LevelInfo, h.records[0].Level)
}

func TestSlogReplayMultipleLines(t *testing.T) {
	h := &collectHandler{}
	replay := newSlogReplay(h)

	input := `{"time":"2025-01-15T10:30:00Z","level":"INFO","msg":"first"}
{"time":"2025-01-15T10:30:01Z","level":"WARN","msg":"second"}
{"time":"2025-01-15T10:30:02Z","level":"ERROR","msg":"third"}
`
	replay.Forward(strings.NewReader(input))

	require.Len(t, h.records, 3)
	assert.Equal(t, "first", h.records[0].Message)
	assert.Equal(t, slog.LevelInfo, h.records[0].Level)
	assert.Equal(t, "second", h.records[1].Message)
	assert.Equal(t, slog.LevelWarn, h.records[1].Level)
	assert.Equal(t, "third", h.records[2].Message)
	assert.Equal(t, slog.LevelError, h.records[2].Level)
}

func TestSlogReplayRoundTrip(t *testing.T) {
	// Write a record using slog.JSONHandler, then replay it and verify
	// the replayed record matches the original.
	var buf bytes.Buffer
	writer := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	writer.Info("round trip test",
		"string_attr", "hello",
		"int_attr", 42,
		"bool_attr", true,
		slog.Group("grp", "nested", "value"),
	)

	// Replay through collector
	h := &collectHandler{}
	replay := newSlogReplay(h)
	replay.Forward(&buf)

	require.Len(t, h.records, 1)
	rec := h.records[0]

	assert.Equal(t, "round trip test", rec.Message)
	assert.Equal(t, slog.LevelInfo, rec.Level)

	attrs := map[string]slog.Attr{}
	rec.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a
		return true
	})

	assert.Equal(t, "hello", attrs["string_attr"].Value.String())
	assert.Equal(t, int64(42), attrs["int_attr"].Value.Int64())
	assert.Equal(t, true, attrs["bool_attr"].Value.Bool())
	assert.Equal(t, slog.KindGroup, attrs["grp"].Value.Kind())

	group := attrs["grp"].Value.Group()
	assert.Len(t, group, 1)
	assert.Equal(t, "nested", group[0].Key)
	assert.Equal(t, "value", group[0].Value.String())
}

func TestSlogReplayEmptyLines(t *testing.T) {
	h := &collectHandler{}
	replay := newSlogReplay(h)

	input := "\n\n" + `{"time":"2025-01-15T10:30:00Z","level":"INFO","msg":"after blanks"}` + "\n\n"
	replay.Forward(strings.NewReader(input))

	require.Len(t, h.records, 1)
	assert.Equal(t, "after blanks", h.records[0].Message)
}

package outboard

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"time"
)

// slogReplay reads lines of slog JSON handler output and replays each record
// through the destination handler with full fidelity: exact timestamp, level,
// message, source, and all attributes including nested groups.
type slogReplay struct {
	handler slog.Handler
}

// newSlogReplay creates a new replay writer that parses slog JSON lines and
// emits them through handler.
func newSlogReplay(handler slog.Handler) *slogReplay {
	return &slogReplay{handler: handler}
}

// Forward reads from r, parsing each line as slog JSON output and replaying it
// through the handler. Non-JSON lines are emitted as plain INFO messages.
func (s *slogReplay) Forward(r io.Reader) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		s.processLine(line)
	}
}

// slog JSON handler keys
const (
	slogTimeKey   = "time"
	slogLevelKey  = "level"
	slogMsgKey    = "msg"
	slogSourceKey = "source"
)

func (s *slogReplay) processLine(line []byte) {
	// Use a streaming decoder to preserve JSON key order.
	// map[string]json.RawMessage would lose ordering since Go maps are unordered.
	orderedKeys, rawValues, err := decodeOrderedJSON(line)
	if err != nil {
		// Not valid JSON — emit as plain text
		rec := slog.NewRecord(time.Now(), slog.LevelInfo, string(line), 0)
		s.handler.Handle(context.Background(), rec)
		return
	}

	// Extract built-in slog fields
	var t time.Time
	level := slog.LevelInfo
	var msg string
	var sourceAttr *slog.Attr

	for i, key := range orderedKeys {
		rawVal := rawValues[i]
		switch key {
		case slogTimeKey:
			if err := json.Unmarshal(rawVal, &t); err != nil {
				t = time.Time{}
			}
		case slogLevelKey:
			var levelStr string
			if err := json.Unmarshal(rawVal, &levelStr); err == nil {
				var l slog.Level
				if err := l.UnmarshalText([]byte(levelStr)); err == nil {
					level = l
				}
			}
		case slogMsgKey:
			json.Unmarshal(rawVal, &msg)
		case slogSourceKey:
			var src struct {
				Function string `json:"function"`
				File     string `json:"file"`
				Line     int    `json:"line"`
			}
			if err := json.Unmarshal(rawVal, &src); err == nil {
				attr := slog.Group(slogSourceKey,
					slog.String("function", src.Function),
					slog.String("file", src.File),
					slog.Int("line", src.Line),
				)
				sourceAttr = &attr
			}
		}
	}

	if t.IsZero() {
		t = time.Now()
	}

	// Build the record
	rec := slog.NewRecord(t, level, msg, 0)

	if sourceAttr != nil {
		rec.AddAttrs(*sourceAttr)
	}

	// Add remaining attributes in original JSON key order
	for i, key := range orderedKeys {
		switch key {
		case slogTimeKey, slogLevelKey, slogMsgKey, slogSourceKey:
			continue
		}
		rec.AddAttrs(parseAttr(key, rawValues[i]))
	}

	s.handler.Handle(context.Background(), rec)
}

// decodeOrderedJSON parses a JSON object preserving key order.
// Returns parallel slices of keys and raw values in the order they appear.
func decodeOrderedJSON(data []byte) ([]string, []json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(data))

	// Read opening '{'
	tok, err := dec.Token()
	if err != nil {
		return nil, nil, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, nil, &json.SyntaxError{}
	}

	var keys []string
	var values []json.RawMessage

	for dec.More() {
		// Read key
		tok, err := dec.Token()
		if err != nil {
			return nil, nil, err
		}
		key, ok := tok.(string)
		if !ok {
			return nil, nil, &json.SyntaxError{}
		}

		// Read value
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, nil, err
		}

		keys = append(keys, key)
		values = append(values, raw)
	}

	return keys, values, nil
}

// parseAttr converts a JSON key/value into an slog.Attr, preserving types
// as faithfully as possible.
func parseAttr(key string, raw json.RawMessage) slog.Attr {
	// Try to determine the JSON type from the first byte
	if len(raw) == 0 {
		return slog.String(key, "")
	}

	switch raw[0] {
	case '"':
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return slog.String(key, s)
		}

	case 't', 'f':
		var b bool
		if err := json.Unmarshal(raw, &b); err == nil {
			return slog.Bool(key, b)
		}

	case 'n':
		// null
		return slog.Any(key, nil)

	case '{':
		// Object — could be a group. Use ordered decoding to preserve key order.
		keys, values, err := decodeOrderedJSON(raw)
		if err == nil {
			attrs := make([]any, 0, len(keys))
			for i, k := range keys {
				attrs = append(attrs, parseAttr(k, values[i]))
			}
			return slog.Group(key, attrs...)
		}

	case '[':
		// Array — decode as []any
		var arr []any
		if err := json.Unmarshal(raw, &arr); err == nil {
			return slog.Any(key, arr)
		}

	default:
		// Number — try int64 first, then float64
		var n json.Number
		if err := json.Unmarshal(raw, &n); err == nil {
			if i, err := n.Int64(); err == nil {
				return slog.Int64(key, i)
			}
			if f, err := n.Float64(); err == nil {
				return slog.Float64(key, f)
			}
		}
	}

	// Fallback: use raw string
	return slog.String(key, string(raw))
}

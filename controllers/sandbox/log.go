package sandbox

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"

	"miren.dev/runtime/observability"
)

var traceIdRegx = regexp.MustCompile(`trace_id"?\s*[=:]\s*\"?(\w+)`)

type SandboxLogs struct {
	log    *slog.Logger
	entity string
	attrs  map[string]string
	extra  map[string]string
	buf    bytes.Buffer
	stream observability.LogStream
	lw     observability.LogWriter
}

func NewSandboxLogs(
	log *slog.Logger,
	entity string,
	attrs map[string]string,
	lw observability.LogWriter,
) *SandboxLogs {
	return &SandboxLogs{
		log:    log,
		entity: entity,
		attrs:  attrs,
		extra:  make(map[string]string),
		stream: observability.Stdout,
		lw:     lw,
	}
}

func (s *SandboxLogs) Write(p []byte) (n int, err error) {
	n = len(p)

	if s.buf.Len() > 0 {
		s.buf.Write(p)
		p = s.buf.Bytes()
	}

	for len(p) > 0 {
		nl := bytes.IndexByte(p, '\n')
		if nl == -1 {
			s.buf.Write(p)
			break
		}

		s.processLine(string(p[:nl]))

		p = p[nl+1:]
	}

	return
}

var jsonLevelToStream = map[string]observability.LogStream{
	"ERROR": observability.Stderr,
	"error": observability.Stderr,
	"WARN":  observability.Stderr,
	"warn":  observability.Stderr,
}

func (s *SandboxLogs) processLine(line string) {
	ts := time.Now()

	line = strings.TrimRight(line, "\t\n\r")

	stream := s.stream

	if strings.HasPrefix(line, "!USER ") {
		line = strings.TrimPrefix(line, "!USER ")
		stream = observability.UserOOB
	} else if strings.HasPrefix(line, "!ERROR ") {
		line = strings.TrimPrefix(line, "!ERROR ")
		stream = observability.Error
	}

	traceId := ""
	if matches := traceIdRegx.FindStringSubmatch(line); len(matches) > 1 {
		traceId = matches[1]
	}

	var extra map[string]string
	if body, lvlStream, ok := s.scanJSON(line); ok {
		s.extra["user.orig_msg"] = line
		line = body
		if lvlStream != "" {
			stream = lvlStream
		}
		extra = s.extra
	}

	err := s.lw.WriteEntry(s.entity, observability.LogEntry{
		Timestamp:  ts,
		Stream:     stream,
		Body:       line,
		TraceID:    traceId,
		Attributes: s.attrs,
		Extra:      extra,
	})
	if err != nil {
		s.log.Error("failed to write log entry", "error", err, "line", line)
	}
}

// scanJSON uses the json tokenizer to extract structured fields from a JSON log
// line directly into s.extra, avoiding a full unmarshal. Returns the message,
// an optional stream override, and whether parsing succeeded.
func (s *SandboxLogs) scanJSON(line string) (string, observability.LogStream, bool) {
	if len(line) == 0 || line[0] != '{' {
		return "", "", false
	}

	dec := json.NewDecoder(strings.NewReader(line))

	// Expect opening brace
	t, err := dec.Token()
	if err != nil || t != json.Delim('{') {
		return "", "", false
	}

	clear(s.extra)

	var msg string
	var stream observability.LogStream

	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return "", "", false
		}
		key, ok := keyTok.(string)
		if !ok {
			return "", "", false
		}

		valTok, err := dec.Token()
		if err != nil {
			return "", "", false
		}

		// If value is a nested object or array, skip it
		if delim, ok := valTok.(json.Delim); ok {
			if !skipNestedJSON(dec, delim) {
				return "", "", false
			}
			continue
		}

		switch key {
		case "msg", "message":
			if v, ok := valTok.(string); ok {
				msg = v
			}
		case "level":
			if v, ok := valTok.(string); ok {
				stream = jsonLevelToStream[v]
			}
		case "time":
			// skip
		default:
			switch v := valTok.(type) {
			case string:
				s.extra[key] = v
			case float64:
				s.extra[key] = strconv.FormatFloat(v, 'f', -1, 64)
			case bool:
				s.extra[key] = strconv.FormatBool(v)
			case nil:
				// skip nulls
			}
		}
	}

	if msg == "" {
		return "", "", false
	}

	return msg, stream, true
}

// skipNestedJSON consumes a balanced JSON object or array from the decoder.
func skipNestedJSON(dec *json.Decoder, open json.Delim) bool {
	depth := 1
	for depth > 0 {
		t, err := dec.Token()
		if err != nil {
			return false
		}
		if d, ok := t.(json.Delim); ok {
			switch d {
			case '{', '[':
				depth++
			case '}', ']':
				depth--
			}
		}
	}
	return true
}

func (s *SandboxLogs) Stderr() *SandboxLogs {
	x := *s
	x.stream = observability.Stderr
	x.extra = make(map[string]string)

	return &x
}

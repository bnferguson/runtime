package observability

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"
)

const (
	defaultFlushInterval = 250 * time.Millisecond
	defaultFlushCount    = 50
)

// BatchLogWriter implements LogWriter by buffering entries and flushing them
// as a single NDJSON HTTP POST to VictoriaLogs. This reduces write pressure
// compared to one POST per log record.
type BatchLogWriter struct {
	writer *PersistentLogWriter

	mu    sync.Mutex
	buf   bytes.Buffer
	count int

	flushCh   chan struct{}
	done      chan struct{}
	wg        sync.WaitGroup
	closeOnce sync.Once
}

// NewBatchLogWriter wraps a PersistentLogWriter with batching. Entries are
// buffered and flushed either every 250ms or when 50 entries accumulate,
// whichever comes first.
func NewBatchLogWriter(writer *PersistentLogWriter) *BatchLogWriter {
	b := &BatchLogWriter{
		writer:  writer,
		flushCh: make(chan struct{}, 1),
		done:    make(chan struct{}),
	}
	b.wg.Add(1)
	go b.run()
	return b
}

// WriteEntry marshals the entry to NDJSON and appends it to the internal
// buffer. It never blocks the caller — writes are best-effort.
func (b *BatchLogWriter) WriteEntry(entity string, le LogEntry) error {
	msg := le.Body
	if msg == "" {
		msg = " "
	}

	logData := map[string]any{
		"_msg":     msg,
		"_time":    le.Timestamp.UTC().Format(time.RFC3339Nano),
		"entity":   entity,
		"stream":   string(le.Stream),
		"trace_id": le.TraceID,
	}
	for k, v := range le.Attributes {
		if isReservedLogField(k) {
			continue
		}
		logData[k] = v
	}
	for k, v := range le.Extra {
		if isReservedLogField(k) {
			continue
		}
		logData[k] = v
	}

	jsonData, err := json.Marshal(logData)
	if err != nil {
		return fmt.Errorf("failed to marshal log entry: %w", err)
	}

	b.mu.Lock()
	b.buf.Write(jsonData)
	b.buf.WriteByte('\n')
	b.count++
	shouldFlush := b.count >= defaultFlushCount
	b.mu.Unlock()

	if shouldFlush {
		select {
		case b.flushCh <- struct{}{}:
		default:
		}
	}

	return nil
}

// Close signals the background goroutine to perform a final flush and stop.
// It is safe to call multiple times.
func (b *BatchLogWriter) Close() {
	b.closeOnce.Do(func() {
		close(b.done)
	})
	b.wg.Wait()
}

func (b *BatchLogWriter) run() {
	defer b.wg.Done()
	ticker := time.NewTicker(defaultFlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			b.flush()
		case <-b.flushCh:
			b.flush()
		case <-b.done:
			b.flush()
			return
		}
	}
}

func (b *BatchLogWriter) flush() {
	b.mu.Lock()
	if b.count == 0 {
		b.mu.Unlock()
		return
	}
	data := make([]byte, b.buf.Len())
	copy(data, b.buf.Bytes())
	b.buf.Reset()
	b.count = 0
	b.mu.Unlock()

	baseURL := normalizeBaseURL(b.writer.Address)
	insertURL := baseURL + "/insert/jsonline"

	resp, err := b.writer.Client().Post(insertURL, "application/x-ndjson", bytes.NewReader(data))
	if err != nil {
		return
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
}

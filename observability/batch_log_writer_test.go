package observability

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestBatchLogWriter_FlushOnTimer(t *testing.T) {
	var mu sync.Mutex
	var received []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		received = append(received, string(body))
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	plw := NewPersistentLogWriter(srv.URL, 5*time.Second)
	bw := NewBatchLogWriter(plw)

	// Write a single entry — below the count threshold
	err := bw.WriteEntry("test-entity", LogEntry{
		Timestamp: time.Now(),
		Stream:    Stdout,
		Body:      "hello timer",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Wait for the timer to flush (defaultFlushInterval is 250ms)
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(received) == 0 {
		t.Fatal("expected at least one flush from timer, got none")
	}

	// Verify the NDJSON content
	lines := strings.Split(strings.TrimSpace(received[0]), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	var logData map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &logData); err != nil {
		t.Fatal(err)
	}
	if logData["_msg"] != "hello timer" {
		t.Fatalf("expected _msg=hello timer, got %v", logData["_msg"])
	}
	if logData["entity"] != "test-entity" {
		t.Fatalf("expected entity=test-entity, got %v", logData["entity"])
	}

	bw.Close()
}

func TestBatchLogWriter_FlushOnThreshold(t *testing.T) {
	var mu sync.Mutex
	var received []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		received = append(received, string(body))
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	plw := NewPersistentLogWriter(srv.URL, 5*time.Second)
	bw := NewBatchLogWriter(plw)

	// Write exactly defaultFlushCount entries to trigger threshold flush
	for i := range defaultFlushCount {
		err := bw.WriteEntry("test-entity", LogEntry{
			Timestamp: time.Now(),
			Stream:    Stdout,
			Body:      "entry " + time.Now().Format(time.RFC3339Nano),
		})
		if err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}

	// Give the background goroutine a moment to process
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	count := len(received)
	mu.Unlock()

	if count == 0 {
		t.Fatal("expected threshold flush, got no POSTs")
	}

	// The first POST should contain at least defaultFlushCount lines
	mu.Lock()
	lines := strings.Split(strings.TrimSpace(received[0]), "\n")
	mu.Unlock()
	if len(lines) < defaultFlushCount {
		t.Fatalf("expected at least %d lines in first POST, got %d", defaultFlushCount, len(lines))
	}

	bw.Close()
}

func TestBatchLogWriter_CloseFlushesRemaining(t *testing.T) {
	var mu sync.Mutex
	var received []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		received = append(received, string(body))
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	plw := NewPersistentLogWriter(srv.URL, 5*time.Second)
	bw := NewBatchLogWriter(plw)

	// Write a few entries (below threshold)
	for i := range 3 {
		err := bw.WriteEntry("drain-entity", LogEntry{
			Timestamp: time.Now(),
			Stream:    Stdout,
			Body:      "drain entry",
		})
		if err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}

	// Close immediately — should drain the buffer
	bw.Close()

	mu.Lock()
	defer mu.Unlock()

	if len(received) == 0 {
		t.Fatal("expected Close() to flush remaining entries, got none")
	}

	// Count total lines across all POSTs
	total := 0
	for _, body := range received {
		lines := strings.Split(strings.TrimSpace(body), "\n")
		total += len(lines)
	}
	if total != 3 {
		t.Fatalf("expected 3 entries flushed on close, got %d", total)
	}
}

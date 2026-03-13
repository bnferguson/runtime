package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"miren.dev/runtime/api/app/app_v1alpha"
	"miren.dev/runtime/pkg/rpc/standard"
)

func TestBuildFilterWithService(t *testing.T) {
	tests := []struct {
		name       string
		userFilter string
		service    string
		want       string
	}{
		{
			name:       "no service, no filter",
			userFilter: "",
			service:    "",
			want:       "",
		},
		{
			name:       "service only",
			userFilter: "",
			service:    "web",
			want:       `service:"web"`,
		},
		{
			name:       "filter only",
			userFilter: "error",
			service:    "",
			want:       "error",
		},
		{
			name:       "service and filter",
			userFilter: "error",
			service:    "web",
			want:       `service:"web" error`,
		},
		{
			name:       "service with complex filter",
			userFilter: `error -debug /timeout/`,
			service:    "worker",
			want:       `service:"worker" error -debug /timeout/`,
		},
		{
			name:       "service with spaces needs quoting",
			userFilter: "",
			service:    "my service",
			want:       `service:"my service"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildFilterWithService(tt.userFilter, tt.service)
			if got != tt.want {
				t.Errorf("buildFilterWithService(%q, %q) = %q, want %q",
					tt.userFilter, tt.service, got, tt.want)
			}
		})
	}
}

// TestBuildFilterWithService_LegacyCompatibility verifies the contract that
// dispatchLogs depends on: when no --service flag is passed, combinedFilter
// must equal rawFilter so the legacy capability check (rawFilter != combinedFilter)
// doesn't falsely trip. If this test fails, older servers without streamLogChunks
// will reject plain `miren logs` requests.
func TestBuildFilterWithService_LegacyCompatibility(t *testing.T) {
	for _, userFilter := range []string{"", "error", `"exact phrase"`, `error -debug /timeout/`} {
		combined := buildFilterWithService(userFilter, "")
		if combined != userFilter {
			t.Errorf("buildFilterWithService(%q, \"\") = %q, want %q — "+
				"no-service filter must equal rawFilter for legacy server compat",
				userFilter, combined, userFilter)
		}
	}
}

func TestNormalizeSandboxID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"abc123", "sandbox/abc123"},
		{"sandbox/abc123", "sandbox/abc123"},
		{"", "sandbox/"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeSandboxID(tt.input)
			if got != tt.want {
				t.Errorf("normalizeSandboxID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildBuildFilter(t *testing.T) {
	tests := []struct {
		name       string
		version    string
		userFilter string
		want       string
	}{
		{
			name:    "version only",
			version: "v3",
			want:    `source:build version:"v3"`,
		},
		{
			name:       "version with filter",
			version:    "v3",
			userFilter: "error",
			want:       `source:build version:"v3" error`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildBuildFilter(tt.version, tt.userFilter)
			if got != tt.want {
				t.Errorf("buildBuildFilter(%q, %q) = %q, want %q",
					tt.version, tt.userFilter, got, tt.want)
			}
		})
	}
}

func TestPrintLogEntryJSON(t *testing.T) {
	ts := time.Date(2026, 3, 13, 16, 30, 0, 0, time.UTC)

	t.Run("full entry", func(t *testing.T) {
		var buf bytes.Buffer
		ctx := &Context{Context: context.Background(), Stdout: &buf}

		entry := &app_v1alpha.LogEntry{}
		entry.SetTimestamp(standard.ToTimestamp(ts))
		entry.SetStream("stdout")
		entry.SetSource("sandbox/abc123")
		entry.SetLine("Hello world")
		entry.SetAttributes(map[string]string{"service": "web"})

		printLogEntryJSON(ctx, entry)

		var got logEntryJSON
		if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
			t.Fatalf("invalid JSON output: %v\nraw: %s", err, buf.String())
		}

		if got.Timestamp != "2026-03-13T16:30:00Z" {
			t.Errorf("timestamp = %q, want %q", got.Timestamp, "2026-03-13T16:30:00Z")
		}
		if got.Stream != "stdout" {
			t.Errorf("stream = %q, want %q", got.Stream, "stdout")
		}
		if got.Source != "sandbox/abc123" {
			t.Errorf("source = %q, want %q", got.Source, "sandbox/abc123")
		}
		if got.Message != "Hello world" {
			t.Errorf("message = %q, want %q", got.Message, "Hello world")
		}
		if got.Attributes["service"] != "web" {
			t.Errorf("attributes[service] = %q, want %q", got.Attributes["service"], "web")
		}
	})

	t.Run("minimal entry omits empty fields", func(t *testing.T) {
		var buf bytes.Buffer
		ctx := &Context{Context: context.Background(), Stdout: &buf}

		entry := &app_v1alpha.LogEntry{}
		entry.SetTimestamp(standard.ToTimestamp(ts))
		entry.SetStream("stderr")
		entry.SetLine("error occurred")

		printLogEntryJSON(ctx, entry)

		raw := strings.TrimSpace(buf.String())
		if strings.Contains(raw, "source") {
			t.Errorf("expected source to be omitted, got: %s", raw)
		}
		if strings.Contains(raw, "attributes") {
			t.Errorf("expected attributes to be omitted, got: %s", raw)
		}
	})

	t.Run("multiple entries produce JSONL", func(t *testing.T) {
		var buf bytes.Buffer
		ctx := &Context{Context: context.Background(), Stdout: &buf}

		for i := 0; i < 3; i++ {
			entry := &app_v1alpha.LogEntry{}
			entry.SetTimestamp(standard.ToTimestamp(ts))
			entry.SetStream("stdout")
			entry.SetLine("line")
			printLogEntryJSON(ctx, entry)
		}

		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		if len(lines) != 3 {
			t.Fatalf("expected 3 lines, got %d", len(lines))
		}
		for i, line := range lines {
			var obj map[string]any
			if err := json.Unmarshal([]byte(line), &obj); err != nil {
				t.Errorf("line %d is not valid JSON: %v", i, err)
			}
		}
	})
}

func TestBuildSystemFilter(t *testing.T) {
	tests := []struct {
		name       string
		component  string
		userFilter string
		want       string
	}{
		{
			name: "no component, no filter",
			want: `source:"system"`,
		},
		{
			name:      "component only",
			component: "etcd",
			want:      `source:"system" module:"etcd"`,
		},
		{
			name:       "filter only",
			userFilter: "error",
			want:       `source:"system" error`,
		},
		{
			name:       "component and filter",
			component:  "scheduler",
			userFilter: "error",
			want:       `source:"system" module:"scheduler" error`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildSystemFilter(tt.component, tt.userFilter)
			if got != tt.want {
				t.Errorf("buildSystemFilter(%q, %q) = %q, want %q",
					tt.component, tt.userFilter, got, tt.want)
			}
		})
	}
}

package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"miren.dev/runtime/clientconfig"
)

// fakeAccessInfo implements accessInfoLike so deployURLs can be exercised
// without constructing generated proto messages.
type fakeAccessInfo struct {
	hasHostnames    bool
	hostnames       []string
	defaultRoute    bool
	clusterHostname string
}

func (f fakeAccessInfo) HasHostnames() bool      { return f.hasHostnames }
func (f fakeAccessInfo) Hostnames() *[]string    { return &f.hostnames }
func (f fakeAccessInfo) DefaultRoute() bool      { return f.defaultRoute }
func (f fakeAccessInfo) ClusterHostname() string { return f.clusterHostname }

func summaryTestCtx() *Context {
	return &Context{
		Context: context.Background(),
		Log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func TestDeployURLs(t *testing.T) {
	tests := []struct {
		name           string
		accessInfo     accessInfoLike
		ephemeralLabel string
		clusterConfig  *clientconfig.ClusterConfig
		want           []string
	}{
		{
			name:       "nil access info",
			accessInfo: nil,
			want:       nil,
		},
		{
			name:       "stable with hostnames dedups",
			accessInfo: fakeAccessInfo{hasHostnames: true, hostnames: []string{"a.example.com", "b.example.com", "a.example.com"}},
			want:       []string{"https://a.example.com", "https://b.example.com"},
		},
		{
			name:       "stable default route uses cluster hostname",
			accessInfo: fakeAccessInfo{defaultRoute: true, clusterHostname: "c.org.miren.systems"},
			want:       []string{"https://c.org.miren.systems"},
		},
		{
			name:          "stable default route falls back to cluster config, stripping port",
			accessInfo:    fakeAccessInfo{defaultRoute: true},
			clusterConfig: &clientconfig.ClusterConfig{Hostname: "cluster.example.com:8443"},
			want:          []string{"https://cluster.example.com"},
		},
		{
			name:       "stable no hostnames and no default route yields nothing",
			accessInfo: fakeAccessInfo{},
			want:       nil,
		},
		{
			name:           "ephemeral adds label subdomain alongside hostnames",
			accessInfo:     fakeAccessInfo{hasHostnames: true, hostnames: []string{"x.example.com"}, clusterHostname: "cl.miren.systems"},
			ephemeralLabel: "pr-1",
			want:           []string{"https://x.example.com", "https://pr-1.cl.miren.systems"},
		},
		{
			name:           "ephemeral without cluster hostname keeps only hostnames",
			accessInfo:     fakeAccessInfo{hasHostnames: true, hostnames: []string{"x.example.com"}},
			ephemeralLabel: "pr-1",
			want:           []string{"https://x.example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := summaryTestCtx()
			ctx.ClusterConfig = tt.clusterConfig
			got := deployURLs(ctx, tt.accessInfo, tt.ephemeralLabel)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("deployURLs() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestWriteDeploySummary(t *testing.T) {
	t.Run("stable deploy writes all fields", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "summary.json")
		writeDeploySummary(summaryTestCtx(), path, "dpl_abc", "ver_123", []string{"https://a.example.com", "https://b.example.com"})

		var got deploySummary
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading summary: %v", err)
		}
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("unmarshaling summary: %v", err)
		}
		want := deploySummary{
			DeployID:   "dpl_abc",
			AppVersion: "ver_123",
			URLs:       []string{"https://a.example.com", "https://b.example.com"},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("summary = %#v, want %#v", got, want)
		}
	})

	t.Run("no urls still emits a stable schema", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "summary.json")
		writeDeploySummary(summaryTestCtx(), path, "dpl_x", "ver_x", nil)

		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading summary: %v", err)
		}
		// urls must serialize as [] (never null), and there is no url field.
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("unmarshaling summary: %v", err)
		}
		if _, ok := raw["url"]; ok {
			t.Fatalf("did not expect a url field, got: %s", data)
		}
		if string(raw["urls"]) != "[]" {
			t.Fatalf("expected urls to be [], got: %s", raw["urls"])
		}
	})

	t.Run("ephemeral deploy leaves deploy_id empty", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "summary.json")
		writeDeploySummary(summaryTestCtx(), path, "", "ver_eph", []string{"https://pr-1.cl.miren.systems"})

		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading summary: %v", err)
		}
		// deploy_id must be present-but-empty (a stable schema for consumers),
		// not omitted.
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("unmarshaling summary: %v", err)
		}
		if _, ok := raw["deploy_id"]; !ok {
			t.Fatalf("expected deploy_id key to be present, got: %s", data)
		}
		var got deploySummary
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("unmarshaling summary: %v", err)
		}
		if got.DeployID != "" {
			t.Fatalf("expected empty deploy_id, got %q", got.DeployID)
		}
		if got.AppVersion != "ver_eph" {
			t.Fatalf("expected app_version ver_eph, got %q", got.AppVersion)
		}
	})

	t.Run("empty path is a silent no-op", func(t *testing.T) {
		// A capturing logger lets us observe that the empty-path early return
		// neither writes nor logs (a successful write emits a Debug line).
		var logs bytes.Buffer
		ctx := &Context{
			Context: context.Background(),
			Log:     slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
		}
		writeDeploySummary(ctx, "", "dpl_x", "ver_x", []string{"https://x.example.com"})
		if logs.Len() != 0 {
			t.Fatalf("expected no log output for empty path, got: %s", logs.String())
		}
	})
}

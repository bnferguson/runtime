package sandbox

import (
	"testing"
	"time"
)

func TestResolvePortWaitTimeout(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want time.Duration
	}{
		{"empty", "", defaultPortWaitTimeout},
		{"valid 30s", "30s", 30 * time.Second},
		{"valid 1m", "1m", time.Minute},
		{"valid 500ms", "500ms", 500 * time.Millisecond},
		{"zero", "0s", defaultPortWaitTimeout},
		{"negative", "-5s", defaultPortWaitTimeout},
		{"garbage", "not-a-duration", defaultPortWaitTimeout},
		{"bare number", "30", defaultPortWaitTimeout},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolvePortWaitTimeout(tc.in)
			if got != tc.want {
				t.Fatalf("resolvePortWaitTimeout(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

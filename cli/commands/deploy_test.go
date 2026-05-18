package commands

import (
	"sync/atomic"
	"testing"
	"time"

	"miren.dev/runtime/pkg/progress/upload"
)

func TestEnrichUploadProgress(t *testing.T) {
	const total int64 = 1_000_000

	cases := []struct {
		name         string
		written      int64
		bytesRead    int64
		elapsed      time.Duration
		wantFraction float64
		wantETA      time.Duration
	}{
		{
			name:         "zero total skips entirely",
			written:      100,
			bytesRead:    50,
			elapsed:      5 * time.Second,
			wantFraction: 0,
			wantETA:      0,
		},
		{
			name:         "warmup period suppresses ETA",
			written:      10_000,
			bytesRead:    5_000,
			elapsed:      4 * time.Second,
			wantFraction: 0.01,
			wantETA:      0,
		},
		{
			name:         "steady-state ETA extrapolates from elapsed",
			written:      100_000,
			bytesRead:    50_000,
			elapsed:      10 * time.Second,
			wantFraction: 0.1,
			wantETA:      90 * time.Second,
		},
		{
			name:         "completed upload reports no ETA",
			written:      total,
			bytesRead:    500_000,
			elapsed:      60 * time.Second,
			wantFraction: 1.0,
			wantETA:      0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var written atomic.Int64
			written.Store(tc.written)

			p := upload.Progress{
				BytesRead: tc.bytesRead,
				Duration:  tc.elapsed,
			}
			totalArg := total
			if tc.name == "zero total skips entirely" {
				totalArg = 0
			}
			enrichUploadProgress(&p, &written, int64(totalArg))

			if p.Fraction != tc.wantFraction {
				t.Errorf("Fraction = %v, want %v", p.Fraction, tc.wantFraction)
			}
			if p.ETA != tc.wantETA {
				t.Errorf("ETA = %v, want %v", p.ETA, tc.wantETA)
			}
		})
	}
}

package etcd

import (
	"slices"
	"testing"
)

func TestComputeTuning(t *testing.T) {
	tests := []struct {
		name    string
		ramGiB  float64
		want    etcdTuning
		wantRAM int64
	}{
		{
			name:   "1GB",
			ramGiB: 1,
			want: etcdTuning{
				BudgetBytes: 107374182, QuotaBackendBytes: 32212254, GoMemLimitBytes: 69793218,
				GOGC: 25, AutoCompactionReten: "30m", SnapshotCount: 1000,
				SnapshotCatchupEntries: 500, MaxConcurrentStreams: 51, CompactionBatchLimit: 100,
			},
		},
		{
			name:   "2GB",
			ramGiB: 2,
			want: etcdTuning{
				BudgetBytes: 214748364, QuotaBackendBytes: 64424509, GoMemLimitBytes: 139586436,
				GOGC: 25, AutoCompactionReten: "30m", SnapshotCount: 1000,
				SnapshotCatchupEntries: 500, MaxConcurrentStreams: 102, CompactionBatchLimit: 100,
			},
		},
		{
			name:   "4GB",
			ramGiB: 4,
			want: etcdTuning{
				BudgetBytes: 429496729, QuotaBackendBytes: 128849018, GoMemLimitBytes: 279172873,
				GOGC: 50, AutoCompactionReten: "1h", SnapshotCount: 5000,
				SnapshotCatchupEntries: 2500, MaxConcurrentStreams: 204, CompactionBatchLimit: 500,
			},
		},
		{
			name:   "8GB",
			ramGiB: 8,
			want: etcdTuning{
				BudgetBytes: 858993459, QuotaBackendBytes: 257698037, GoMemLimitBytes: 558345748,
				GOGC: 50, AutoCompactionReten: "1h", SnapshotCount: 5000,
				SnapshotCatchupEntries: 2500, MaxConcurrentStreams: 409, CompactionBatchLimit: 500,
			},
		},
		{
			name:   "16GB",
			ramGiB: 16,
			want: etcdTuning{
				BudgetBytes: 1717986918, QuotaBackendBytes: 515396075, GoMemLimitBytes: 1116691496,
				GOGC: 75, AutoCompactionReten: "2h", SnapshotCount: 10000,
				SnapshotCatchupEntries: 5000, MaxConcurrentStreams: 819, CompactionBatchLimit: 1000,
			},
		},
		{
			name:   "32GB",
			ramGiB: 32,
			want: etcdTuning{
				BudgetBytes: 3435973836, QuotaBackendBytes: 1030792150, GoMemLimitBytes: 2233382993,
				GOGC: 100, AutoCompactionReten: "3h", SnapshotCount: 10000,
				SnapshotCatchupEntries: 5000, MaxConcurrentStreams: 1638, CompactionBatchLimit: 1000,
			},
		},
		{
			name:   "64GB",
			ramGiB: 64,
			want: etcdTuning{
				BudgetBytes: 6871947673, QuotaBackendBytes: 2061584301, GoMemLimitBytes: 4466765987,
				GOGC: 100, AutoCompactionReten: "3h", SnapshotCount: 10000,
				SnapshotCatchupEntries: 5000, MaxConcurrentStreams: 3276, CompactionBatchLimit: 1000,
			},
		},
		{
			name:   "sub-1GB floors to 100MiB",
			ramGiB: 0.5,
			want: etcdTuning{
				BudgetBytes: 104857600, QuotaBackendBytes: 31457280, GoMemLimitBytes: 68157440,
				GOGC: 25, AutoCompactionReten: "30m", SnapshotCount: 1000,
				SnapshotCatchupEntries: 500, MaxConcurrentStreams: 50, CompactionBatchLimit: 100,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ram := int64(tc.ramGiB * gib)
			got := computeTuning(ram)

			tc.want.SystemRAMBytes = ram
			tc.want.AutoCompactionMode = "periodic"

			if got != tc.want {
				t.Errorf("computeTuning(%d):\n got  %+v\n want %+v", ram, got, tc.want)
			}
		})
	}
}

func TestComputeTuningZeroRAMUsesFloor(t *testing.T) {
	got := computeTuning(0)
	if got.BudgetBytes != memoryFloorBytes {
		t.Errorf("budget = %d, want floor %d", got.BudgetBytes, memoryFloorBytes)
	}
	if got.GOGC != 25 || got.SnapshotCount != 1000 || got.MaxConcurrentStreams != 50 {
		t.Errorf("zero RAM should yield smallest tier, got %+v", got)
	}
}

func TestBudgetIsMonotonicInRAM(t *testing.T) {
	var prev int64
	for gigs := 1; gigs <= 128; gigs++ {
		b := computeTuning(int64(gigs) * gib).BudgetBytes
		if b < prev {
			t.Fatalf("budget decreased at %d GiB: %d < %d", gigs, b, prev)
		}
		prev = b
	}
}

func TestTuningEnvVars(t *testing.T) {
	env := computeTuning(8 * gib).envVars()

	want := []string{
		"GOMEMLIMIT=558345748B",
		"GOGC=50",
		"ETCD_AUTO_COMPACTION_MODE=periodic",
		"ETCD_AUTO_COMPACTION_RETENTION=1h",
		"ETCD_EXPERIMENTAL_BACKEND_BBOLT_FREELIST_TYPE=map",
	}
	if !slices.Equal(env, want) {
		t.Errorf("envVars() = %v, want %v", env, want)
	}
}

func TestTuningSignature(t *testing.T) {
	// Same RAM yields a stable, non-empty signature.
	sig := computeTuning(8 * gib).signature()
	if sig == "" {
		t.Fatal("signature is empty")
	}
	if again := computeTuning(8 * gib).signature(); again != sig {
		t.Errorf("signature not stable for equal RAM: %q != %q", sig, again)
	}

	// A RAM change that moves the node to a different tier changes the signature,
	// which is what forces the container to be recreated on restart.
	if other := computeTuning(32 * gib).signature(); other == sig {
		t.Errorf("signature did not change across tiers: %q", sig)
	}

	// A RAM change that stays within the same tier (5 GiB and 8 GiB are both the
	// medium tier) but alters the continuous formulas (quota/GOMEMLIMIT/streams)
	// also changes the signature.
	if other := computeTuning(5 * gib).signature(); other == sig {
		t.Errorf("signature did not change for a within-tier RAM change: %q", sig)
	}
}

func TestTuningArgs(t *testing.T) {
	args := computeTuning(8 * gib).args()

	want := []string{
		"--quota-backend-bytes", "257698037",
		"--snapshot-count", "5000",
		"--experimental-snapshot-catchup-entries", "2500",
		"--max-concurrent-streams", "409",
		"--experimental-compaction-batch-limit", "500",
	}
	if !slices.Equal(args, want) {
		t.Errorf("args() = %v, want %v", args, want)
	}
}

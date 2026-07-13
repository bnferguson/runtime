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
				BudgetBytes: 107374182, QuotaBackendBytes: 4294967296, GoMemLimitBytes: 69793218,
				GOGC: 25, AutoCompactionReten: "30m", SnapshotCount: 1000,
				SnapshotCatchupEntries: 500, MaxConcurrentStreams: 51, CompactionBatchLimit: 100,
			},
		},
		{
			name:   "2GB",
			ramGiB: 2,
			want: etcdTuning{
				BudgetBytes: 214748364, QuotaBackendBytes: 4294967296, GoMemLimitBytes: 139586436,
				GOGC: 25, AutoCompactionReten: "30m", SnapshotCount: 1000,
				SnapshotCatchupEntries: 500, MaxConcurrentStreams: 102, CompactionBatchLimit: 100,
			},
		},
		{
			name:   "4GB",
			ramGiB: 4,
			want: etcdTuning{
				BudgetBytes: 429496729, QuotaBackendBytes: 4294967296, GoMemLimitBytes: 279172873,
				GOGC: 50, AutoCompactionReten: "1h", SnapshotCount: 5000,
				SnapshotCatchupEntries: 2500, MaxConcurrentStreams: 204, CompactionBatchLimit: 500,
			},
		},
		{
			name:   "8GB",
			ramGiB: 8,
			want: etcdTuning{
				BudgetBytes: 858993459, QuotaBackendBytes: 4294967296, GoMemLimitBytes: 558345748,
				GOGC: 50, AutoCompactionReten: "1h", SnapshotCount: 5000,
				SnapshotCatchupEntries: 2500, MaxConcurrentStreams: 409, CompactionBatchLimit: 500,
			},
		},
		{
			name:   "16GB",
			ramGiB: 16,
			want: etcdTuning{
				BudgetBytes: 1717986918, QuotaBackendBytes: 4294967296, GoMemLimitBytes: 1116691496,
				GOGC: 75, AutoCompactionReten: "2h", SnapshotCount: 10000,
				SnapshotCatchupEntries: 5000, MaxConcurrentStreams: 819, CompactionBatchLimit: 1000,
			},
		},
		{
			name:   "32GB",
			ramGiB: 32,
			want: etcdTuning{
				BudgetBytes: 3435973836, QuotaBackendBytes: 4294967296, GoMemLimitBytes: 2233382993,
				GOGC: 100, AutoCompactionReten: "3h", SnapshotCount: 10000,
				SnapshotCatchupEntries: 5000, MaxConcurrentStreams: 1638, CompactionBatchLimit: 1000,
			},
		},
		{
			name:   "64GB",
			ramGiB: 64,
			want: etcdTuning{
				BudgetBytes: 6871947673, QuotaBackendBytes: 4294967296, GoMemLimitBytes: 4466765987,
				GOGC: 100, AutoCompactionReten: "3h", SnapshotCount: 10000,
				SnapshotCatchupEntries: 5000, MaxConcurrentStreams: 3276, CompactionBatchLimit: 1000,
			},
		},
		{
			name:   "sub-1GB floors to 100MiB",
			ramGiB: 0.5,
			want: etcdTuning{
				BudgetBytes: 104857600, QuotaBackendBytes: 4294967296, GoMemLimitBytes: 68157440,
				GOGC: 25, AutoCompactionReten: "30m", SnapshotCount: 1000,
				SnapshotCatchupEntries: 500, MaxConcurrentStreams: 50, CompactionBatchLimit: 100,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ram := int64(tc.ramGiB * gib)
			got := computeTuning(ram, 0)

			tc.want.SystemRAMBytes = ram
			tc.want.AutoCompactionMode = "periodic"

			if got != tc.want {
				t.Errorf("computeTuning(%d):\n got  %+v\n want %+v", ram, got, tc.want)
			}
		})
	}
}

func TestQuotaFloorAndCap(t *testing.T) {
	// quota-backend-bytes is never set below the 4 GiB floor, so the tuning is purely
	// additive backend headroom and never a NOSPACE write-outage regression. For every
	// node up to 64 GB, 3% of RAM is under 4 GiB, so the floor applies.
	for _, ramGiB := range []int64{1, 2, 4, 8, 16, 32, 64} {
		got := computeTuning(ramGiB*gib, 0).QuotaBackendBytes
		if got != quotaFloorBytes {
			t.Errorf("%d GiB: quota = %d, want floor %d", ramGiB, got, quotaFloorBytes)
		}
	}

	// A very large node (3% of RAM above the 4 GiB floor) gets the RAM-scaled quota.
	// 200 GiB → budget 20 GiB → 30% = 6 GiB, between floor and cap.
	if got := computeTuning(200*gib, 0).QuotaBackendBytes; got != 200*gib/10*30/100 {
		t.Errorf("200 GiB: quota = %d, want RAM-scaled %d", got, 200*gib/10*30/100)
	}

	// An enormous node is capped at etcd's 8 GiB maximum.
	if got := computeTuning(300*gib, 0).QuotaBackendBytes; got != quotaCapBytes {
		t.Errorf("300 GiB: quota = %d, want cap %d", got, quotaCapBytes)
	}

	// An explicit override wins over the RAM-scaled value (even below the floor).
	if got := computeTuning(64*gib, 6*gib).QuotaBackendBytes; got != 6*gib {
		t.Errorf("override: quota = %d, want %d", got, int64(6*gib))
	}
	if got := computeTuning(64*gib, 1*gib).QuotaBackendBytes; got != 1*gib {
		t.Errorf("override below floor: quota = %d, want %d", got, int64(1*gib))
	}
}

func TestComputeTuningZeroRAMUsesFloor(t *testing.T) {
	got := computeTuning(0, 0)
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
		b := computeTuning(int64(gigs)*gib, 0).BudgetBytes
		if b < prev {
			t.Fatalf("budget decreased at %d GiB: %d < %d", gigs, b, prev)
		}
		prev = b
	}
}

func TestTuningEnvVars(t *testing.T) {
	env := computeTuning(8*gib, 0).envVars()

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
	sig := computeTuning(8*gib, 0).signature()
	if sig == "" {
		t.Fatal("signature is empty")
	}
	if again := computeTuning(8*gib, 0).signature(); again != sig {
		t.Errorf("signature not stable for equal RAM: %q != %q", sig, again)
	}

	// A RAM change that moves the node to a different tier changes the signature,
	// which is what forces the container to be recreated on restart.
	if other := computeTuning(32*gib, 0).signature(); other == sig {
		t.Errorf("signature did not change across tiers: %q", sig)
	}

	// A RAM change that stays within the same tier (5 GiB and 8 GiB are both the
	// medium tier) but alters the continuous soft knobs (GOMEMLIMIT/streams) also
	// changes the signature.
	if other := computeTuning(5*gib, 0).signature(); other == sig {
		t.Errorf("signature did not change for a within-tier RAM change: %q", sig)
	}

	// An explicit quota override changes the signature (so a config change recreates
	// the container on restart).
	if other := computeTuning(8*gib, 6*gib).signature(); other == sig {
		t.Errorf("signature did not change for a quota override: %q", sig)
	}
}

func TestTuningArgs(t *testing.T) {
	args := computeTuning(8*gib, 0).args()

	want := []string{
		"--quota-backend-bytes", "4294967296",
		"--snapshot-count", "5000",
		"--experimental-snapshot-catchup-entries", "2500",
		"--max-concurrent-streams", "409",
		"--experimental-compaction-batch-limit", "500",
	}
	if !slices.Equal(args, want) {
		t.Errorf("args() = %v, want %v", args, want)
	}
}

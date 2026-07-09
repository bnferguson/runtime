package etcd

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// etcd memory tuning derives etcd's runtime configuration from the amount of RAM
// on the node it runs on. etcd is a small co-tenant here: it should hold to roughly
// 10% of system RAM rather than grow to fill the node. We compute a per-node budget
//
//	B = max(0.10 * system_RAM, 100 MiB)
//
// and scale etcd's advisory config (Go GC pressure via GOMEMLIMIT/GOGC, backend quota,
// compaction/snapshot cadence, concurrent streams) off it. These are advisory only:
// no cgroup limit is imposed, so etcd is never OOM-killed by us; GOMEMLIMIT simply
// drives the Go runtime to GC harder as it approaches the budget.
const (
	kib = 1024
	mib = 1024 * 1024
	gib = 1024 * 1024 * 1024

	// memoryFloorBytes is etcd's practical resident floor: below ~100 MiB it cannot
	// hold a working keyspace, so the budget never drops under this even on tiny nodes.
	memoryFloorBytes = 100 * mib

	// quotaCapBytes caps the backend quota regardless of how large the node is.
	quotaCapBytes = 8 * gib

	// Tier boundaries, expressed against the budget B. They line up with the standard
	// node sizes: nodes up to ~2 GB, ~8 GB, and ~16 GB of RAM respectively (B = RAM/10
	// above the floor), so a 2 GB node lands in the smallest tier, an 8 GB node in the
	// next, and so on.
	tierSmallMaxBytes  = 2 * gib / 10  // ~204.8 MiB
	tierMediumMaxBytes = 8 * gib / 10  // ~819.2 MiB
	tierLargeMaxBytes  = 16 * gib / 10 // ~1638.4 MiB
)

// etcdTuning is the set of etcd config values computed for a given node size.
type etcdTuning struct {
	SystemRAMBytes int64
	BudgetBytes    int64

	QuotaBackendBytes      int64
	GoMemLimitBytes        int64
	GOGC                   int
	AutoCompactionMode     string
	AutoCompactionReten    string
	SnapshotCount          int
	SnapshotCatchupEntries int
	MaxConcurrentStreams   int
	CompactionBatchLimit   int
}

// detectSystemRAMBytes reads total system memory from /proc/meminfo. It returns 0 when
// the value cannot be determined so callers fall back to the memory floor.
func detectSystemRAMBytes() int64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "MemTotal:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0
		}
		kb, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return 0
		}
		return kb * kib // /proc/meminfo reports kB
	}
	return 0
}

// computeTuning derives etcd config from the node's RAM. A systemRAMBytes of 0 (unknown)
// yields the smallest tier via the budget floor.
func computeTuning(systemRAMBytes int64) etcdTuning {
	budget := systemRAMBytes / 10
	if budget < memoryFloorBytes {
		budget = memoryFloorBytes
	}

	quota := budget * 30 / 100
	if quota > quotaCapBytes {
		quota = quotaCapBytes
	}

	streams := budget / (2 * mib)
	switch {
	case streams < 50:
		streams = 50
	case streams > 8000:
		streams = 8000
	}

	t := etcdTuning{
		SystemRAMBytes:       systemRAMBytes,
		BudgetBytes:          budget,
		QuotaBackendBytes:    quota,
		GoMemLimitBytes:      budget * 65 / 100,
		AutoCompactionMode:   "periodic",
		MaxConcurrentStreams: int(streams),
	}

	switch {
	case budget <= tierSmallMaxBytes:
		t.GOGC = 25
		t.AutoCompactionReten = "30m"
		t.SnapshotCount = 1000
		t.SnapshotCatchupEntries = 500
		t.CompactionBatchLimit = 100
	case budget <= tierMediumMaxBytes:
		t.GOGC = 50
		t.AutoCompactionReten = "1h"
		t.SnapshotCount = 5000
		t.SnapshotCatchupEntries = 2500
		t.CompactionBatchLimit = 500
	case budget <= tierLargeMaxBytes:
		t.GOGC = 75
		t.AutoCompactionReten = "2h"
		t.SnapshotCount = 10000
		t.SnapshotCatchupEntries = 5000
		t.CompactionBatchLimit = 1000
	default:
		t.GOGC = 100
		t.AutoCompactionReten = "3h"
		t.SnapshotCount = 10000
		t.SnapshotCatchupEntries = 5000
		t.CompactionBatchLimit = 1000
	}

	return t
}

// envVars renders the Go-runtime and etcd env for the container. GOMEMLIMIT/GOGC steer the
// Go runtime; the ETCD_* vars configure compaction. The bbolt freelist type is kept from the
// prior fixed config.
func (t etcdTuning) envVars() []string {
	return []string{
		fmt.Sprintf("GOMEMLIMIT=%dB", t.GoMemLimitBytes),
		fmt.Sprintf("GOGC=%d", t.GOGC),
		fmt.Sprintf("ETCD_AUTO_COMPACTION_MODE=%s", t.AutoCompactionMode),
		fmt.Sprintf("ETCD_AUTO_COMPACTION_RETENTION=%s", t.AutoCompactionReten),
		"ETCD_EXPERIMENTAL_BACKEND_BBOLT_FREELIST_TYPE=map",
	}
}

// signature is a stable fingerprint of the tuning-derived spec (env + args). It is
// persisted in etcdState so a restart on a node whose RAM has changed regenerates the
// container instead of reusing a spec built for the old budget.
func (t etcdTuning) signature() string {
	return strings.Join(append(t.envVars(), t.args()...), " ")
}

// args renders the etcd command-line flags for the tuning. Flag names target etcd v3.5,
// where snapshot-catchup-entries and compaction-batch-limit are experimental-prefixed
// (both graduated to stable names in 3.6).
func (t etcdTuning) args() []string {
	return []string{
		"--quota-backend-bytes", strconv.FormatInt(t.QuotaBackendBytes, 10),
		"--snapshot-count", strconv.Itoa(t.SnapshotCount),
		"--experimental-snapshot-catchup-entries", strconv.Itoa(t.SnapshotCatchupEntries),
		"--max-concurrent-streams", strconv.Itoa(t.MaxConcurrentStreams),
		"--experimental-compaction-batch-limit", strconv.Itoa(t.CompactionBatchLimit),
	}
}

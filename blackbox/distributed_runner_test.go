//go:build blackbox

package blackbox

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"miren.dev/runtime/blackbox/harness"
)

func skipIfNotDistributed(t *testing.T, c *harness.Cluster) {
	t.Helper()
	if !c.IsPeers() {
		t.Skip("skipping: requires distributed environment (BLACKBOX_MODE=peers)")
	}
}

func TestDistributedRunnerList(t *testing.T) {
	c := harness.NewCluster(t)
	skipIfNotDistributed(t, c)
	m := harness.NewMiren(t, c)

	harness.Poll(t, "at least 2 ready runners", 30*time.Second, 3*time.Second,
		func() (bool, string) {
			r := m.Run("runner", "list", "--format", "json")
			if !r.Success() {
				return false, "runner list failed"
			}

			var runners []struct {
				Name   string `json:"name"`
				Status string `json:"status"`
			}
			if err := json.Unmarshal([]byte(r.Stdout), &runners); err != nil {
				return false, "failed to parse runner list JSON"
			}

			readyCount := 0
			for _, runner := range runners {
				if runner.Status == "ready" || runner.Status == "status.ready" {
					readyCount++
				}
			}
			if readyCount < 2 {
				return false, fmt.Sprintf("only %d ready runners", readyCount)
			}
			return true, ""
		},
	)
}

func TestDistributedRunnerMetrics(t *testing.T) {
	c := harness.NewCluster(t)
	skipIfNotDistributed(t, c)
	m := harness.NewMiren(t, c)

	harness.DeployApp(t, m, harness.AppOptions{
		Testdata: "go-server",
	})

	// Wait for metrics to be collected (the monitor runs every ~10s)
	harness.Poll(t, "metrics in VictoriaMetrics", 60*time.Second, 5*time.Second,
		func() (bool, string) {
			r := m.PeerExec("coordinator", "curl", "-sf",
				"http://localhost:8428/api/v1/label/__name__/values")
			if !r.Success() {
				return false, "VictoriaMetrics query failed"
			}

			var resp struct {
				Data []string `json:"data"`
			}
			if err := json.Unmarshal([]byte(r.Stdout), &resp); err != nil {
				return false, "failed to parse response"
			}

			hasCPU := false
			hasMem := false
			for _, name := range resp.Data {
				if name == "cpu_usage_seconds_total" {
					hasCPU = true
				}
				if name == "memory_usage_bytes" {
					hasMem = true
				}
			}

			if !hasCPU || !hasMem {
				return false, "waiting for cpu_usage_seconds_total and memory_usage_bytes"
			}
			return true, ""
		},
	)
}

func TestDistributedRunnerLogs(t *testing.T) {
	c := harness.NewCluster(t)
	skipIfNotDistributed(t, c)
	m := harness.NewMiren(t, c)

	name := harness.DeployApp(t, m, harness.AppOptions{
		Testdata: "go-server",
	})

	// App logs should flow from the runner through VictoriaLogs and be
	// queryable via the miren logs command on the coordinator.
	harness.Poll(t, "app logs available", 60*time.Second, 3*time.Second,
		func() (bool, string) {
			r := m.Run("logs", "-a", name)
			if r.OutputContains("starting on port") || r.OutputContains("Server starting") {
				return true, ""
			}
			return false, "no app startup log yet"
		},
	)
}

// TestDistributedRunnerNodePort guards the MIR-1032 fix: NodePort DNAT rules
// must install on every node that runs the service controller (coordinator
// and all runners), not only on the node hosting the sandbox. The tcp-echo
// testdata app declares node_port = 7000, so after deploy every peer's nft
// service_nodeports map must contain an entry for tcp/7000.
// runnerListEntry mirrors the JSON emitted by `runner list --format json` for
// the fields the cordon/drain tests care about.
type runnerListEntry struct {
	RunnerID   string   `json:"runner_id"`
	Name       string   `json:"name"`
	Status     string   `json:"status"`
	Scheduling string   `json:"scheduling"`
	Cordoned   bool     `json:"cordoned"`
	Labels     []string `json:"labels"`
}

func listRunners(t *testing.T, m *harness.Miren) []runnerListEntry {
	t.Helper()
	r := m.Run("runner", "list", "--format", "json")
	r.RequireSuccess(t)
	var runners []runnerListEntry
	if err := json.Unmarshal([]byte(r.Stdout), &runners); err != nil {
		t.Fatalf("failed to parse runner list JSON: %v", err)
	}
	return runners
}

func isCoordinator(r runnerListEntry) bool {
	for _, l := range r.Labels {
		if l == "role=coordinator" {
			return true
		}
	}
	return false
}

// findRunnerNode returns the stable runner_id of a non-coordinator (distributed
// runner) node, waiting until at least one is ready. runner_id is used rather
// than the mutable Name so lookups stay unambiguous even for unnamed or
// duplicate-named runners; the miren CLI accepts it wherever a runner is named.
func findRunnerNode(t *testing.T, m *harness.Miren) string {
	t.Helper()
	var runnerID string
	harness.Poll(t, "a ready distributed runner node", 30*time.Second, 3*time.Second,
		func() (bool, string) {
			for _, r := range listRunners(t, m) {
				if isCoordinator(r) {
					continue
				}
				if r.Status == "ready" || r.Status == "status.ready" {
					runnerID = r.RunnerID
					return true, ""
				}
			}
			return false, "no ready runner node yet"
		},
	)
	return runnerID
}

func runnerEntry(t *testing.T, m *harness.Miren, runnerID string) runnerListEntry {
	t.Helper()
	for _, r := range listRunners(t, m) {
		if r.RunnerID == runnerID {
			return r
		}
	}
	t.Fatalf("runner %q not found in runner list", runnerID)
	return runnerListEntry{}
}

// TestDistributedRunnerCordon verifies cordon/uncordon toggle a distributed
// runner's schedulability from the coordinator without going through SIGUSR2.
func TestDistributedRunnerCordon(t *testing.T) {
	c := harness.NewCluster(t)
	skipIfNotDistributed(t, c)
	m := harness.NewMiren(t, c)

	runner := findRunnerNode(t, m)

	m.Run("runner", "cordon", runner, "--reason", "blackbox cordon test").RequireSuccess(t)

	harness.Poll(t, "runner reports cordoned", 15*time.Second, 2*time.Second,
		func() (bool, string) {
			e := runnerEntry(t, m, runner)
			if !e.Cordoned || e.Scheduling != "cordoned" {
				return false, "runner not yet cordoned"
			}
			return true, ""
		},
	)

	m.Run("runner", "uncordon", runner).RequireSuccess(t)

	harness.Poll(t, "runner reports uncordoned", 15*time.Second, 2*time.Second,
		func() (bool, string) {
			if runnerEntry(t, m, runner).Cordoned {
				return false, "runner still cordoned"
			}
			return true, ""
		},
	)
}

// TestDistributedRunnerDrain deploys a stateless app (which prefers the runner
// node), drains that runner from the coordinator, and verifies the app recovers
// (rescheduled onto the coordinator) while the runner ends up cordoned. It then
// uncordons to restore the cluster.
func TestDistributedRunnerDrain(t *testing.T) {
	c := harness.NewCluster(t)
	skipIfNotDistributed(t, c)
	m := harness.NewMiren(t, c)

	runner := findRunnerNode(t, m)

	name := harness.DeployApp(t, m, harness.AppOptions{
		Testdata: "go-server",
	})

	// Drain the runner. This cordons it and evicts its sandboxes; the drain
	// command blocks until the node is empty (or times out).
	m.Run("runner", "drain", runner, "--reason", "blackbox drain test", "--timeout", "120").RequireSuccess(t)

	// The runner must end up cordoned and still present (drain does not remove
	// the node).
	harness.Poll(t, "runner cordoned after drain", 15*time.Second, 2*time.Second,
		func() (bool, string) {
			if !runnerEntry(t, m, runner).Cordoned {
				return false, "runner not cordoned after drain"
			}
			return true, ""
		},
	)

	// The app must recover on another node now that the runner is drained.
	harness.WaitForAppReady(t, m, name, 120*time.Second)

	// Restore the cluster for subsequent tests.
	m.Run("runner", "uncordon", runner).RequireSuccess(t)
}

func TestDistributedRunnerNodePort(t *testing.T) {
	c := harness.NewCluster(t)
	skipIfNotDistributed(t, c)
	m := harness.NewMiren(t, c)

	harness.DeployApp(t, m, harness.AppOptions{
		Testdata: "tcp-echo",
	})

	for _, peer := range []string{"coordinator", "runner1"} {
		harness.Poll(t, fmt.Sprintf("nodeport rule on %s", peer), 60*time.Second, 2*time.Second,
			func() (bool, string) {
				r := m.PeerExec(peer, "nft", "list", "map", "inet", "miren", "service_nodeports")
				if !r.Success() {
					return false, fmt.Sprintf("nft list map failed (exit %d): %s", r.ExitCode, strings.TrimSpace(r.Stderr))
				}
				if !strings.Contains(r.Stdout, "tcp . 7000") {
					return false, fmt.Sprintf("service_nodeports has no entry for tcp/7000 on %s yet", peer)
				}
				return true, ""
			},
		)
	}
}

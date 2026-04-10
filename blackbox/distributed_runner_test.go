//go:build blackbox

package blackbox

import (
	"encoding/json"
	"fmt"
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

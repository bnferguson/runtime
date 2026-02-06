package compute

import (
	"testing"

	"miren.dev/runtime/api/compute/compute_v1alpha"
)

func TestSandboxStatusCoverage(t *testing.T) {
	// Every SandboxStatus must be covered by exactly one of
	// SandboxActive or SandboxDead. If a new status is added to the
	// schema without updating these helpers, this test will fail.
	allStatuses := []compute_v1alpha.SandboxStatus{
		compute_v1alpha.PENDING,
		compute_v1alpha.NOT_READY,
		compute_v1alpha.RUNNING,
		compute_v1alpha.STOPPED,
		compute_v1alpha.DEAD,
	}

	for _, s := range allStatuses {
		active := SandboxActive(s)
		dead := SandboxDead(s)

		if !active && !dead {
			t.Errorf("status %q is neither Active nor Dead", s)
		}
		if active && dead {
			t.Errorf("status %q is both Active and Dead", s)
		}
	}
}

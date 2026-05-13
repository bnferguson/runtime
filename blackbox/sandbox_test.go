//go:build blackbox

package blackbox

import (
	"testing"

	"miren.dev/runtime/blackbox/harness"
)

func TestSandboxList(t *testing.T) {
	c := harness.NewCluster(t)
	m := harness.NewMiren(t, c)

	name := harness.DeployApp(t, m, harness.AppOptions{
		Testdata: "go-server",
	})

	// List sandboxes — our app's sandbox should appear in the output.
	// Use JSON format since the table view shows short IDs, not app names.
	r := m.MustRun("sandbox", "list", "--format", "json")
	r.RequireContains(t, name)
}

func TestSandboxExec(t *testing.T) {
	c := harness.NewCluster(t)
	m := harness.NewMiren(t, c)

	name := harness.DeployApp(t, m, harness.AppOptions{
		Testdata: "go-server",
	})

	sandboxID := harness.GetSandboxID(t, m, name)

	t.Run("positional", func(t *testing.T) {
		r := m.MustRun("sandbox", "exec", sandboxID, "--", "echo", "hello-from-sandbox")
		r.RequireContains(t, "hello-from-sandbox")
	})

	t.Run("id-flag", func(t *testing.T) {
		r := m.MustRun("sandbox", "exec", "-i", sandboxID, "--", "echo", "hello-from-sandbox")
		r.RequireContains(t, "hello-from-sandbox")
	})
}

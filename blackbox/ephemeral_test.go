//go:build blackbox

package blackbox

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"miren.dev/runtime/blackbox/harness"
)

func TestEphemeralDeploy(t *testing.T) {
	c := harness.NewCluster(t)
	m := harness.NewMiren(t, c)

	// Deploy an app normally first
	name := harness.DeployApp(t, m, harness.AppOptions{
		Testdata: "go-server",
	})

	// Capture the active version before ephemeral deploy
	activeVersion := getEphemeralAppVersion(t, m, name)
	if activeVersion == "" {
		t.Fatal("expected a non-empty active version after deploy")
	}
	t.Logf("active version after initial deploy: %s", activeVersion)

	// Set a normal route — ephemeral subdomains work automatically without a wildcard route
	baseHost := name + ".test.local"
	m.MustRun("route", "set", baseHost, name)
	t.Cleanup(func() {
		m.Run("route", "remove", baseHost)
	})

	// Verify the active version is reachable on the base host
	harness.Poll(t, "active version reachable", 30*time.Second, 2*time.Second, func() (bool, string) {
		code, body, err := harness.HTTPGet(m, baseHost, "/")
		if err != nil {
			return false, fmt.Sprintf("HTTP error: %v", err)
		}
		if code != 200 {
			return false, fmt.Sprintf("HTTP status %d: %s", code, body)
		}
		return true, ""
	})

	// Deploy an ephemeral version
	hostDir := filepath.Join(c.TestdataDir, "go-server")
	containerDir := m.ContainerPath(hostDir)

	r := m.MustRun("deploy", "-a", name, "-d", containerDir, "-f",
		"--ephemeral", "feat-preview", "--ttl", "1h")
	r.RequireContains(t, "Ephemeral version")
	r.RequireContains(t, "feat-preview")

	// Verify the active version did NOT change
	currentVersion := getEphemeralAppVersion(t, m, name)
	if currentVersion != activeVersion {
		t.Errorf("active version changed after ephemeral deploy: was %s, now %s", activeVersion, currentVersion)
	}

	// Verify ephemeral version shows in app versions
	r = m.MustRun("app", "versions", "-a", name)
	r.RequireContains(t, "ephemeral")
	r.RequireContains(t, "feat-preview")

	// Verify ephemeral-only filter works
	r = m.MustRun("app", "versions", "-a", name, "--ephemeral")
	r.RequireContains(t, "feat-preview")

	// Verify JSON output works
	r = m.MustRun("app", "versions", "-a", name, "--format", "json")
	r.RequireContains(t, "feat-preview")
	r.RequireContains(t, "ephemeral")

	// Verify the ephemeral version activates and serves traffic via subdomain
	ephemeralHost := "feat-preview." + baseHost
	harness.Poll(t, "ephemeral version serving traffic", 2*time.Minute, 3*time.Second, func() (bool, string) {
		code, body, err := harness.HTTPGet(m, ephemeralHost, "/")
		if err != nil {
			return false, fmt.Sprintf("HTTP error: %v", err)
		}
		if code != 200 {
			return false, fmt.Sprintf("HTTP status %d: %s", code, body)
		}
		return true, ""
	})
	t.Logf("ephemeral version responding at %s", ephemeralHost)

	// Deploy another ephemeral version with same label (replace)
	r = m.MustRun("deploy", "-a", name, "-d", containerDir, "-f",
		"--ephemeral", "feat-preview", "--ttl", "2h")
	r.RequireContains(t, "Ephemeral version")

	// Active version still unchanged
	currentVersion = getEphemeralAppVersion(t, m, name)
	if currentVersion != activeVersion {
		t.Errorf("active version changed after second ephemeral deploy: was %s, now %s", activeVersion, currentVersion)
	}

	// Verify the replaced ephemeral version also activates
	harness.Poll(t, "replaced ephemeral version serving traffic", 2*time.Minute, 3*time.Second, func() (bool, string) {
		code, body, err := harness.HTTPGet(m, ephemeralHost, "/")
		if err != nil {
			return false, fmt.Sprintf("HTTP error: %v", err)
		}
		if code != 200 {
			return false, fmt.Sprintf("HTTP status %d: %s", code, body)
		}
		return true, ""
	})
}

// getEphemeralAppVersion returns the current version of an app from "app list --format json".
func getEphemeralAppVersion(t *testing.T, m *harness.Miren, appName string) string {
	t.Helper()
	r := m.MustRun("app", "list", "--format", "json")

	var apps []struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal([]byte(r.Stdout), &apps); err != nil {
		t.Fatalf("failed to parse app list JSON: %v", err)
	}
	for _, app := range apps {
		if app.Name == appName {
			return app.Version
		}
	}
	t.Fatalf("app %s not found in app list", appName)
	return ""
}

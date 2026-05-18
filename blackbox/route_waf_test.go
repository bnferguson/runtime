//go:build blackbox

package blackbox

import (
	"fmt"
	"testing"
	"time"

	"miren.dev/runtime/blackbox/harness"
)

func TestRouteWafEnableDisable(t *testing.T) {
	c := harness.NewCluster(t)
	m := harness.NewMiren(t, c)

	name := harness.DeployApp(t, m, harness.AppOptions{
		Testdata: "go-server",
	})

	host := name + ".waf.test.local"

	// Set a route
	m.MustRun("route", "set", host, name).RequireSuccess(t)

	// Enable WAF with default level
	r := m.MustRun("route", "waf", host)
	r.RequireSuccess(t)
	r.RequireContains(t, "WAF Level")

	// Verify WAF shows in route show
	r = m.MustRun("route", "show", host)
	r.RequireSuccess(t)
	r.RequireContains(t, "WAF Level")

	// Verify WAF column in route list
	r = m.MustRun("route", "list")
	r.RequireSuccess(t)
	r.RequireContains(t, "WAF")

	// Enable WAF with explicit level
	r = m.MustRun("route", "waf", host, "--level", "2")
	r.RequireSuccess(t)

	// Disable WAF
	r = m.MustRun("route", "waf", host, "--disable")
	r.RequireSuccess(t)
	r.RequireContains(t, "WAF disabled")

	// Disable again — should be a no-op
	r = m.MustRun("route", "waf", host, "--disable")
	r.RequireSuccess(t)
	r.RequireContains(t, "not enabled")
}

func TestRouteWafBlocksMaliciousRequest(t *testing.T) {
	c := harness.NewCluster(t)
	m := harness.NewMiren(t, c)

	name := harness.DeployApp(t, m, harness.AppOptions{
		Testdata: "go-server",
	})

	host := name + ".wafblock.test.local"

	// Set route and wait for app to be reachable
	m.MustRun("route", "set", host, name).RequireSuccess(t)

	harness.Poll(t, "app reachable via route", 90*time.Second, 3*time.Second, func() (bool, string) {
		code, _, err := harness.HTTPGet(m, host, "/")
		if err != nil {
			return false, err.Error()
		}
		if code != 200 {
			return false, fmt.Sprintf("status %d", code)
		}
		return true, ""
	})

	// Enable WAF
	m.MustRun("route", "waf", host, "--level", "1").RequireSuccess(t)

	// SQL injection should be blocked
	harness.Poll(t, "WAF blocks SQL injection", 30*time.Second, 2*time.Second, func() (bool, string) {
		code, _, err := harness.HTTPGet(m, host, "/?id=1%20OR%201=1--")
		if err != nil {
			return false, err.Error()
		}
		if code == 403 {
			return true, ""
		}
		return false, fmt.Sprintf("expected 403, got %d", code)
	})

	// Clean request should still work
	code, _, err := harness.HTTPGet(m, host, "/")
	if err != nil {
		t.Fatalf("clean request failed: %v", err)
	}
	if code != 200 {
		t.Fatalf("expected 200 for clean request, got %d", code)
	}

	// Disable WAF — previously blocked request should now pass
	m.MustRun("route", "waf", host, "--disable").RequireSuccess(t)

	harness.Poll(t, "WAF disabled allows previously blocked request", 30*time.Second, 2*time.Second, func() (bool, string) {
		code, _, err := harness.HTTPGet(m, host, "/?id=1%20OR%201=1--")
		if err != nil {
			return false, err.Error()
		}
		if code == 200 {
			return true, ""
		}
		return false, fmt.Sprintf("expected 200, got %d", code)
	})
}

func TestRouteWafJsonOutput(t *testing.T) {
	c := harness.NewCluster(t)
	m := harness.NewMiren(t, c)

	name := harness.DeployApp(t, m, harness.AppOptions{
		Testdata: "go-server",
	})

	host := name + ".wafjson.test.local"

	m.MustRun("route", "set", host, name).RequireSuccess(t)

	// Enable WAF with JSON output
	r := m.MustRun("route", "waf", host, "--level", "2", "--format", "json")
	r.RequireSuccess(t)
	r.RequireContains(t, `"waf_level"`)
	r.RequireContains(t, `"route"`)

	// Route show with JSON should include waf_level
	r = m.MustRun("route", "show", host, "--format", "json")
	r.RequireSuccess(t)
	r.RequireContains(t, `"waf_level"`)
}

func TestRouteWafDefaultRoute(t *testing.T) {
	c := harness.NewCluster(t)
	m := harness.NewMiren(t, c)

	name := harness.DeployApp(t, m, harness.AppOptions{
		Testdata: "go-server",
	})

	// Set a default route
	m.MustRun("route", "set-default", name).RequireSuccess(t)
	defer m.MustRun("route", "unset-default")

	// Enable WAF on default route
	r := m.MustRun("route", "waf", "--default", "--level", "1")
	r.RequireSuccess(t)

	// Disable WAF on default route
	r = m.MustRun("route", "waf", "--default", "--disable")
	r.RequireSuccess(t)
}

func TestRouteWafInvalidLevel(t *testing.T) {
	c := harness.NewCluster(t)
	m := harness.NewMiren(t, c)

	name := harness.DeployApp(t, m, harness.AppOptions{
		Testdata: "go-server",
	})

	host := name + ".wafinvalid.test.local"
	m.MustRun("route", "set", host, name).RequireSuccess(t)

	// Level 0 should fail (use --disable instead)
	r := m.Run("route", "waf", host, "--level", "0")
	if r.Success() {
		t.Fatal("expected route waf --level 0 to fail")
	}

	// Level 5 should fail
	r = m.Run("route", "waf", host, "--level", "5")
	if r.Success() {
		t.Fatal("expected route waf --level 5 to fail")
	}
}

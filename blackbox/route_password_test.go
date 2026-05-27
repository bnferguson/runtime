//go:build blackbox

package blackbox

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"miren.dev/runtime/blackbox/harness"
)

func TestRoutePasswordProtection(t *testing.T) {
	c := harness.NewCluster(t)
	m := harness.NewMiren(t, c)

	// Deploy a simple app
	name := harness.DeployApp(t, m, harness.AppOptions{
		Testdata: "go-server",
	})

	host := name + ".test.local"
	m.MustRun("route", "set", host, name)

	// Wait for app to be reachable
	harness.Poll(t, "app responds via route", 30*time.Second, 2*time.Second, func() (bool, string) {
		code, _, err := harness.HTTPGet(m, host, "/")
		if err != nil {
			return false, err.Error()
		}
		if code != 200 {
			return false, fmt.Sprintf("HTTP %d", code)
		}
		return true, ""
	})

	// Create a password provider and protect the route
	m.MustRun("auth", "provider", "add", "password", "test-pw", "--password", "s3cret")
	t.Cleanup(func() {
		m.Run("auth", "provider", "remove", "test-pw", "--force")
	})

	m.MustRun("route", "protect", host, "--provider", "test-pw")

	// Verify route show displays password protection
	r := m.MustRun("route", "show", host)
	r.RequireContains(t, "password")

	// Verify unauthenticated request returns the login form (HTTP 200 with HTML)
	harness.Poll(t, "login form served", 10*time.Second, 1*time.Second, func() (bool, string) {
		code, body, err := harness.HTTPGet(m, host, "/")
		if err != nil {
			return false, err.Error()
		}
		if code != 200 {
			return false, fmt.Sprintf("HTTP %d", code)
		}
		if !strings.Contains(body, "Password Required") {
			return false, "response does not contain login form"
		}
		return true, ""
	})

	// POST the wrong password — should get the form back with error
	code, body := httpPostPassword(t, m, host, "wrongpassword")
	if code != 200 {
		t.Fatalf("expected 200 for wrong password, got %d", code)
	}
	if !strings.Contains(body, "Incorrect password") {
		t.Fatalf("expected error message in form, got: %s", body)
	}

	// POST the correct password — should get a redirect with a session cookie
	cookie := httpLoginAndGetCookie(t, m, host, "s3cret")
	if cookie == "" {
		t.Fatal("expected session cookie after successful login")
	}

	// Use the session cookie to access the app
	code, body = httpGetWithCookie(t, m, host, "/", cookie)
	if code != 200 {
		t.Fatalf("expected 200 with valid session cookie, got %d: %s", code, body)
	}
	// Should get the actual app response, not the login form
	if strings.Contains(body, "Password Required") {
		t.Fatal("expected app content, got login form despite valid cookie")
	}

	// Unprotect the route
	m.MustRun("route", "unprotect", host)

	// Verify unauthenticated access works again
	harness.Poll(t, "route is unprotected", 10*time.Second, 1*time.Second, func() (bool, string) {
		code, body, err := harness.HTTPGet(m, host, "/")
		if err != nil {
			return false, err.Error()
		}
		if code != 200 {
			return false, fmt.Sprintf("HTTP %d", code)
		}
		if strings.Contains(body, "Password Required") {
			return false, "still getting login form"
		}
		return true, ""
	})
}

func TestRoutePasswordProviderLifecycle(t *testing.T) {
	c := harness.NewCluster(t)
	m := harness.NewMiren(t, c)

	// Create a password provider
	m.MustRun("auth", "provider", "add", "password", "lifecycle-pw", "--password", "test123")
	t.Cleanup(func() {
		m.Run("auth", "provider", "remove", "lifecycle-pw", "--force")
	})

	// List should show it with type "password"
	r := m.MustRun("auth", "provider", "list")
	r.RequireContains(t, "lifecycle-pw")
	r.RequireContains(t, "password")

	// Show should display it
	r = m.MustRun("auth", "provider", "show", "lifecycle-pw")
	r.RequireContains(t, "lifecycle-pw")
	r.RequireContains(t, "password")

	// Update the password
	m.MustRun("auth", "provider", "add", "password", "lifecycle-pw", "--password", "newpass", "--update")

	// Remove
	m.MustRun("auth", "provider", "remove", "lifecycle-pw")

	// Should be gone
	r = m.MustRun("auth", "provider", "list")
	if r.OutputContains("lifecycle-pw") {
		t.Fatal("provider still listed after removal")
	}
}

// httpPostPassword POSTs a password to the login endpoint and returns status code and body.
func httpPostPassword(t *testing.T, m *harness.Miren, hostname, password string) (int, string) {
	t.Helper()

	r := m.RunCmd("curl", "-sk", "-w", "\n%{http_code}",
		"-H", fmt.Sprintf("Host: %s", hostname),
		"-d", fmt.Sprintf("password=%s&return=/", password),
		"--max-time", "10",
		// Don't follow redirects so we can inspect the response
		fmt.Sprintf("https://localhost:443/.well-known/miren/auth/login"))

	if !r.Success() {
		t.Fatalf("curl POST failed (exit %d): %s", r.ExitCode, r.Stderr)
	}

	lines := strings.Split(strings.TrimRight(r.Stdout, "\n"), "\n")
	if len(lines) < 1 {
		t.Fatalf("unexpected curl output: %q", r.Stdout)
	}

	statusStr := lines[len(lines)-1]
	var code int
	fmt.Sscanf(statusStr, "%d", &code)
	body := strings.Join(lines[:len(lines)-1], "\n")
	return code, body
}

// httpLoginAndGetCookie POSTs the correct password and extracts the session cookie.
func httpLoginAndGetCookie(t *testing.T, m *harness.Miren, hostname, password string) string {
	t.Helper()

	r := m.RunCmd("curl", "-sk", "-i",
		"-H", fmt.Sprintf("Host: %s", hostname),
		"-d", fmt.Sprintf("password=%s&return=/", password),
		"--max-time", "10",
		fmt.Sprintf("https://localhost:443/.well-known/miren/auth/login"))

	if !r.Success() {
		t.Fatalf("curl POST for login failed (exit %d): %s", r.ExitCode, r.Stderr)
	}

	for _, line := range strings.Split(r.Stdout, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), "set-cookie:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				cookieVal := strings.TrimSpace(parts[1])
				if idx := strings.Index(cookieVal, ";"); idx > 0 {
					cookieVal = cookieVal[:idx]
				}
				if strings.HasPrefix(cookieVal, "miren_pw_session=") {
					return cookieVal
				}
			}
		}
	}

	return ""
}

// httpGetWithCookie makes an HTTP GET with a cookie header.
func httpGetWithCookie(t *testing.T, m *harness.Miren, hostname, path, cookie string) (int, string) {
	t.Helper()

	r := m.RunCmd("curl", "-sk", "-w", "\n%{http_code}",
		"-H", fmt.Sprintf("Host: %s", hostname),
		"-b", cookie,
		"--max-time", "10",
		fmt.Sprintf("https://localhost:443%s", path))

	if !r.Success() {
		t.Fatalf("curl GET with cookie failed (exit %d): %s", r.ExitCode, r.Stderr)
	}

	lines := strings.Split(strings.TrimRight(r.Stdout, "\n"), "\n")
	if len(lines) < 1 {
		t.Fatalf("unexpected curl output: %q", r.Stdout)
	}

	statusStr := lines[len(lines)-1]
	var code int
	fmt.Sscanf(statusStr, "%d", &code)
	body := strings.Join(lines[:len(lines)-1], "\n")
	return code, body
}

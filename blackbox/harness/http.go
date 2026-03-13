package harness

import (
	"fmt"
	"strconv"
	"strings"
)

// HTTPGet makes an HTTP GET request from inside the dev container via curl.
// The host header is set to the given hostname so ingress routing works,
// while the actual request goes to localhost:80.
func HTTPGet(m *Miren, hostname, path string) (statusCode int, body string, err error) {
	r := m.RunCmd("curl", "-s", "-o", "/dev/stdout", "-w", "\n%{http_code}",
		"-H", fmt.Sprintf("Host: %s", hostname),
		"--max-time", "10",
		fmt.Sprintf("http://localhost:80%s", path))

	if !r.Success() {
		return 0, "", fmt.Errorf("curl failed (exit %d): %s", r.ExitCode, strings.TrimSpace(r.Stderr))
	}

	// Output format: body\nstatus_code
	lines := strings.Split(strings.TrimRight(r.Stdout, "\n"), "\n")
	if len(lines) < 1 {
		return 0, "", fmt.Errorf("unexpected curl output: %q", r.Stdout)
	}

	statusStr := lines[len(lines)-1]
	code, parseErr := strconv.Atoi(strings.TrimSpace(statusStr))
	if parseErr != nil {
		return 0, "", fmt.Errorf("failed to parse status code %q: %v", statusStr, parseErr)
	}

	bodyStr := strings.Join(lines[:len(lines)-1], "\n")
	return code, bodyStr, nil
}

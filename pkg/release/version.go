package release

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// VersionInfo contains version information for a binary
type VersionInfo struct {
	Version   string    `json:"version"`
	Commit    string    `json:"commit"`
	BuildDate time.Time `json:"build_date"`
}

// GetCurrentVersion gets the version info of the currently installed binary
func GetCurrentVersion(binaryPath string) (VersionInfo, error) {
	// Try with --format=json first (new versions)
	cmd := exec.Command(binaryPath, "version", "--format=json")
	output, err := cmd.Output()
	if err == nil {
		// Try to parse as JSON
		var info VersionInfo
		if err := json.Unmarshal(output, &info); err == nil {
			return info, nil
		}
	}

	// Fall back to text parsing for older versions
	cmd = exec.Command(binaryPath, "version")
	output, err = cmd.Output()
	if err != nil {
		return VersionInfo{}, fmt.Errorf("failed to get version: %w", err)
	}

	return parseVersionText(string(output)), nil
}

// parseVersionText parses version output text
func parseVersionText(output string) VersionInfo {
	info := VersionInfo{
		Version: "unknown",
		Commit:  "unknown",
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "Version:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				info.Version = strings.TrimSpace(parts[1])
			}
		} else if strings.Contains(line, "Commit:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				info.Commit = strings.TrimSpace(parts[1])
			}
		} else if strings.Contains(line, "Built:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				dateStr := strings.TrimSpace(parts[1])
				// Try to parse the date
				if t, err := time.Parse("2006-01-02 15:04:05 MST", dateStr); err == nil {
					info.BuildDate = t
				} else if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
					info.BuildDate = t
				}
			}
		}
	}

	// Handle simple single-line version output (just the version string)
	if info.Version == "unknown" && len(lines) > 0 && lines[0] != "" && !strings.Contains(lines[0], ":") {
		info.Version = strings.TrimSpace(lines[0])
	}

	return info
}

// ShortCommit returns the 7-char display prefix of v's commit, or "" if the
// commit is missing or "unknown". 7 chars matches git's default short-sha
// length and what users see elsewhere.
func (v VersionInfo) ShortCommit() string {
	c := v.Commit
	if c == "" || c == "unknown" {
		return ""
	}
	if len(c) <= 7 {
		return c
	}
	return c[:7]
}

// Display formats v as "version (commit)" for human-facing output. If the
// commit is missing, just the version string is returned.
func (v VersionInfo) Display() string {
	if sc := v.ShortCommit(); sc != "" {
		return v.Version + " (" + sc + ")"
	}
	return v.Version
}

// Equivalent returns true if v represents the same build as other.
// Commits are the strongest signal: if both sides have a known commit, that
// alone decides. Otherwise we fall back to comparing version string and build
// date. Used by drift detection to ask "is the running daemon actually a
// different build than what's on disk?".
func (v VersionInfo) Equivalent(other VersionInfo) bool {
	if v.Commit != "" && v.Commit != "unknown" &&
		other.Commit != "" && other.Commit != "unknown" {
		return v.Commit == other.Commit
	}
	if v.Version != other.Version {
		return false
	}
	if !v.BuildDate.IsZero() && !other.BuildDate.IsZero() {
		return v.BuildDate.Equal(other.BuildDate)
	}
	return true
}

// GetRunningServiceVersion returns the version info for the binary currently
// executing for the given systemd service. It looks up the service's MainPID
// via systemctl and execs /proc/<pid>/exe.
//
// On Linux this returns the *actually-running* version regardless of what the
// on-disk binary at the install path says. The kernel keeps the original inode
// alive as long as the process holds an exec mapping to it, so /proc/<pid>/exe
// continues to point at the binary the daemon was started from even after an
// atomic rename of the install path.
func GetRunningServiceVersion(serviceName string) (VersionInfo, error) {
	pid, err := getServiceMainPID(serviceName)
	if err != nil {
		return VersionInfo{}, err
	}
	if pid == 0 {
		return VersionInfo{}, fmt.Errorf("service %s is not running", serviceName)
	}
	return GetCurrentVersion(fmt.Sprintf("/proc/%d/exe", pid))
}

func getServiceMainPID(serviceName string) (int, error) {
	cmd := exec.Command("systemctl", "show", "-p", "MainPID", "--value", serviceName)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("systemctl show failed for %s: %w", serviceName, err)
	}
	pidStr := strings.TrimSpace(string(output))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0, fmt.Errorf("parse MainPID %q for %s: %w", pidStr, serviceName, err)
	}
	return pid, nil
}

// IsNewer returns true if this version is newer than the other
func (v VersionInfo) IsNewer(other VersionInfo) bool {
	// Try to parse both as semver
	thisSemver, thisErr := ParseSemVer(v.Version)
	otherSemver, otherErr := ParseSemVer(other.Version)

	// If both are valid semver, use semver comparison
	if thisErr == nil && otherErr == nil {
		return thisSemver.IsNewer(otherSemver)
	}

	// Fall back to existing logic for non-semver versions

	// If both have commits, compare commits first
	// Same commit means identical code regardless of build time
	if v.Commit != "" && v.Commit != "unknown" &&
		other.Commit != "" && other.Commit != "unknown" {
		if v.Commit == other.Commit {
			return false
		}
		// Different commits - fall through to build date comparison
	}

	// If both have build dates, use those for comparison
	if !v.BuildDate.IsZero() && !other.BuildDate.IsZero() {
		return v.BuildDate.After(other.BuildDate)
	}

	// If only one has a build date, consider it newer
	if !v.BuildDate.IsZero() && other.BuildDate.IsZero() {
		return true
	}

	// Fall back to version string comparison
	// Don't upgrade if versions are the same
	if v.Version == other.Version {
		return false
	}

	// Otherwise, consider it newer if different
	return true
}

// GetBinaryPath returns the path to the miren binary
func GetBinaryPath() string {
	// Check if we're running from a release directory
	exe, err := os.Executable()
	if err == nil {
		return exe
	}

	// Default to system path
	return "/var/lib/miren/release/miren"
}

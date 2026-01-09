package commands

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

const (
	// Default timeout for quick commands like df, free, ps
	cmdTimeoutShort = 30 * time.Second
	// Timeout for longer operations like log gathering
	cmdTimeoutLong = 60 * time.Second
)

// runWithTimeout executes a command with the given timeout.
// Returns the combined stdout/stderr output and any error.
func runWithTimeout(timeout time.Duration, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return output, fmt.Errorf("command timed out after %s", timeout)
	}

	return output, err
}

func DebugBundle(ctx *Context, opts struct {
	OutputFile      string `short:"o" long:"output" description:"Output file path" default:"miren-debug.tar.gz"`
	Since           string `short:"s" long:"since" description:"Include logs since this time" default:"1 day ago"`
	Namespace       string `long:"namespace" description:"containerd namespace" default:"miren"`
	Socket          string `long:"socket" description:"path to containerd socket"`
	DockerContainer string `short:"d" long:"docker-container" description:"Docker container name to get logs from" default:"miren"`
}) error {
	ctx.Info("Gathering system debug information...")

	// Create output file and stream directly to disk to minimize memory usage
	// on potentially distressed systems
	outFile, err := os.Create(opts.OutputFile)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}

	gw := gzip.NewWriter(outFile)
	tw := tar.NewWriter(gw)

	// Track success to determine if we should clean up on exit
	success := false
	defer func() {
		tw.Close()
		gw.Close()
		outFile.Close()
		if !success {
			os.Remove(opts.OutputFile)
		}
	}()

	// Gather system info
	ctx.Info("Collecting system information...")
	infoData := gatherSystemInfo()
	if err := writeToArchive(tw, "miren-debug/info.txt", infoData); err != nil {
		return fmt.Errorf("writing info.txt: %w", err)
	}

	// Gather processes
	ctx.Info("Collecting process information...")
	processData, err := gatherProcesses()
	if err != nil {
		ctx.Warn("Failed to gather processes: %v", err)
		processData = []byte(fmt.Sprintf("Error gathering processes: %v\n", err))
	}
	if err := writeToArchive(tw, "miren-debug/processes.txt", processData); err != nil {
		return fmt.Errorf("writing processes.txt: %w", err)
	}

	// Gather Docker container processes (if Docker container exists and is running)
	dockerProcData := gatherDockerProcesses(opts.DockerContainer)
	if len(dockerProcData) > 0 {
		if err := writeToArchive(tw, "miren-debug/docker-processes.txt", dockerProcData); err != nil {
			return fmt.Errorf("writing docker-processes.txt: %w", err)
		}
	}

	// Gather Docker inspect output
	dockerInspectData := gatherDockerInspect(opts.DockerContainer)
	if len(dockerInspectData) > 0 {
		if err := writeToArchive(tw, "miren-debug/docker-inspect.json", dockerInspectData); err != nil {
			return fmt.Errorf("writing docker-inspect.json: %w", err)
		}
	}

	// Gather containers
	ctx.Info("Collecting container information...")
	containerData, err := gatherContainers(ctx, opts.Socket, opts.Namespace)
	if err != nil {
		ctx.Warn("Failed to gather containers: %v", err)
		containerData = []byte(fmt.Sprintf("Error gathering containers: %v\n", err))
	}
	if err := writeToArchive(tw, "miren-debug/containers.txt", containerData); err != nil {
		return fmt.Errorf("writing containers.txt: %w", err)
	}

	// Gather server logs
	ctx.Info("Collecting server logs...")
	logData, err := gatherServerLogs(opts.Since, opts.DockerContainer)
	if err != nil {
		ctx.Warn("Failed to gather server logs: %v", err)
		logData = []byte(fmt.Sprintf("Error gathering server logs: %v\n", err))
	}
	if err := writeToArchive(tw, "miren-debug/server-logs.txt", logData); err != nil {
		return fmt.Errorf("writing server-logs.txt: %w", err)
	}

	// Close archive writers in correct order (defer will also close, but we need
	// to check for errors here)
	if err := tw.Close(); err != nil {
		return fmt.Errorf("closing tar writer: %w", err)
	}
	if err := gw.Close(); err != nil {
		return fmt.Errorf("closing gzip writer: %w", err)
	}

	success = true
	ctx.Completed("Debug information written to %s", opts.OutputFile)
	return nil
}

func writeToArchive(tw *tar.Writer, name string, data []byte) error {
	hdr := &tar.Header{
		Name:    name,
		Mode:    0644,
		Size:    int64(len(data)),
		ModTime: time.Now(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}

func gatherSystemInfo() []byte {
	var buf bytes.Buffer

	hostname, _ := os.Hostname()
	now := time.Now()

	fmt.Fprintf(&buf, "Miren System Debug Information\n")
	fmt.Fprintf(&buf, "Generated: %s\n", now.Format(time.RFC3339))
	fmt.Fprintf(&buf, "Hostname: %s\n", hostname)

	// Add disk usage (df)
	fmt.Fprintf(&buf, "\n%s\n", strings.Repeat("=", 80))
	fmt.Fprintf(&buf, "DISK USAGE (df -h)\n")
	fmt.Fprintf(&buf, "%s\n", strings.Repeat("=", 80))
	dfOutput, err := runWithTimeout(cmdTimeoutShort, "df", "-h")
	if err != nil {
		fmt.Fprintf(&buf, "Error running df: %v\n", err)
	} else {
		buf.Write(dfOutput)
	}

	// Add memory usage (free)
	fmt.Fprintf(&buf, "\n%s\n", strings.Repeat("=", 80))
	fmt.Fprintf(&buf, "MEMORY USAGE (free -h)\n")
	fmt.Fprintf(&buf, "%s\n", strings.Repeat("=", 80))
	freeOutput, err := runWithTimeout(cmdTimeoutShort, "free", "-h")
	if err != nil {
		fmt.Fprintf(&buf, "Error running free: %v\n", err)
	} else {
		buf.Write(freeOutput)
	}

	return buf.Bytes()
}

type processInfo struct {
	pid     int32
	name    string
	state   string
	ppid    int32
	rssKB   uint64
	cpuPct  float64
	cmdline string
}

func gatherProcesses() ([]byte, error) {
	procs, err := process.Processes()
	if err != nil {
		return nil, fmt.Errorf("listing processes: %w", err)
	}

	var processes []processInfo

	for _, p := range procs {
		info := processInfo{pid: p.Pid}

		// Get process name
		if name, err := p.Name(); err == nil {
			info.name = name
		}

		// Get process status
		if status, err := p.Status(); err == nil && len(status) > 0 {
			info.state = status[0]
		}

		// Get parent PID
		if ppid, err := p.Ppid(); err == nil {
			info.ppid = ppid
		}

		// Get memory info
		if memInfo, err := p.MemoryInfo(); err == nil && memInfo != nil {
			info.rssKB = memInfo.RSS / 1024
		}

		// Get CPU percentage (over a short interval)
		if cpuPct, err := p.CPUPercent(); err == nil {
			info.cpuPct = cpuPct
		}

		// Get command line
		if cmdline, err := p.Cmdline(); err == nil {
			info.cmdline = cmdline
		}

		processes = append(processes, info)
	}

	// Sort by PID
	sort.Slice(processes, func(i, j int) bool {
		return processes[i].pid < processes[j].pid
	})

	// Format output
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%-8s %-20s %-6s %-8s %-8s %-12s %s\n", "PID", "NAME", "STATE", "PPID", "CPU%", "RSS(KB)", "CMDLINE")
	fmt.Fprintf(&buf, "%s\n", strings.Repeat("-", 120))

	for _, p := range processes {
		cmdline := p.cmdline
		if len(cmdline) > 50 {
			cmdline = cmdline[:47] + "..."
		}
		fmt.Fprintf(&buf, "%-8d %-20s %-6s %-8d %-8.1f %-12d %s\n",
			p.pid, truncate(p.name, 20), p.state, p.ppid, p.cpuPct, p.rssKB, cmdline)
	}

	return buf.Bytes(), nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// formatAge formats a duration as a human-readable age string like "2h ago" or "3d ago"
func formatAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	}
}

func gatherServerLogs(since, dockerContainer string) ([]byte, error) {
	var buf bytes.Buffer

	// Try journalctl first
	fmt.Fprintf(&buf, "%s\n", strings.Repeat("=", 80))
	fmt.Fprintf(&buf, "JOURNALCTL LOGS (miren service)\n")
	fmt.Fprintf(&buf, "%s\n", strings.Repeat("=", 80))

	output, err := runWithTimeout(cmdTimeoutLong, "journalctl", "-u", "miren", "--no-pager", "--since", since)
	if err != nil {
		fmt.Fprintf(&buf, "journalctl error: %v\n\nOutput:\n%s\n", err, string(output))
	} else {
		buf.Write(output)
	}

	// Check for miren running in Docker
	dockerLogs := gatherDockerLogs(since, dockerContainer)
	if len(dockerLogs) > 0 {
		buf.Write(dockerLogs)
	}

	return buf.Bytes(), nil
}

type dockerContainerInfo struct {
	id     string
	name   string
	status string
}

func findDockerContainers(containerName string) []dockerContainerInfo {
	// Check if docker is available
	if _, err := exec.LookPath("docker"); err != nil {
		return nil
	}

	// Find containers matching the specified name (running or recently stopped)
	output, err := runWithTimeout(cmdTimeoutShort, "docker", "ps", "-a", "--filter", "name="+containerName, "--format", "{{.ID}}\t{{.Names}}\t{{.Status}}")
	if err != nil {
		return nil
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		return nil
	}

	var containers []dockerContainerInfo
	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 2 {
			continue
		}

		info := dockerContainerInfo{
			id:   parts[0],
			name: parts[1],
		}
		if len(parts) >= 3 {
			info.status = parts[2]
		}
		containers = append(containers, info)
	}

	return containers
}

func gatherDockerLogs(since, containerName string) []byte {
	containers := findDockerContainers(containerName)
	if len(containers) == 0 {
		return nil
	}

	// Convert human-readable "since" to Docker format (Go duration or timestamp)
	dockerSince := convertToDockerSince(since)

	var buf bytes.Buffer
	for _, container := range containers {
		fmt.Fprintf(&buf, "\n%s\n", strings.Repeat("=", 80))
		fmt.Fprintf(&buf, "DOCKER LOGS: %s (%s)\n", container.name, container.status)
		fmt.Fprintf(&buf, "Container ID: %s\n", container.id)
		fmt.Fprintf(&buf, "%s\n", strings.Repeat("=", 80))

		// Get logs with --since flag
		logOutput, err := runWithTimeout(cmdTimeoutLong, "docker", "logs", "--since", dockerSince, container.id)
		if err != nil {
			fmt.Fprintf(&buf, "Error getting logs: %v\n%s\n", err, logOutput)
		} else if len(logOutput) == 0 {
			fmt.Fprintf(&buf, "(no logs in the specified time range)\n")
		} else {
			buf.Write(logOutput)
		}
	}

	return buf.Bytes()
}

// convertToDockerSince converts human-readable time strings to Docker's --since format.
// Docker accepts Go duration (e.g., "24h", "30m") or RFC3339 timestamps.
func convertToDockerSince(since string) string {
	since = strings.TrimSpace(strings.ToLower(since))

	// Common conversions from journalctl-style to Docker-style
	conversions := map[string]string{
		"1 day ago":      "24h",
		"1 hour ago":     "1h",
		"2 hours ago":    "2h",
		"6 hours ago":    "6h",
		"12 hours ago":   "12h",
		"1 week ago":     "168h",
		"30 minutes ago": "30m",
	}

	if dockerFormat, ok := conversions[since]; ok {
		return dockerFormat
	}

	// Try to parse patterns like "X days ago", "X hours ago", etc.
	var num int
	var unit string
	if _, err := fmt.Sscanf(since, "%d %s ago", &num, &unit); err == nil {
		switch {
		case strings.HasPrefix(unit, "day"):
			return fmt.Sprintf("%dh", num*24)
		case strings.HasPrefix(unit, "hour"):
			return fmt.Sprintf("%dh", num)
		case strings.HasPrefix(unit, "minute"):
			return fmt.Sprintf("%dm", num)
		case strings.HasPrefix(unit, "week"):
			return fmt.Sprintf("%dh", num*24*7)
		}
	}

	// If already in Go duration format, return as-is
	if _, err := time.ParseDuration(since); err == nil {
		return since
	}

	// Default fallback: 24 hours
	return "24h"
}

func gatherDockerProcesses(containerName string) []byte {
	containers := findDockerContainers(containerName)
	if len(containers) == 0 {
		return nil
	}

	var buf bytes.Buffer
	for _, container := range containers {
		// Only running containers can have exec run on them
		if !strings.HasPrefix(container.status, "Up") {
			continue
		}

		fmt.Fprintf(&buf, "\n%s\n", strings.Repeat("=", 80))
		fmt.Fprintf(&buf, "DOCKER CONTAINER: %s\n", container.name)
		fmt.Fprintf(&buf, "Container ID: %s\n", container.id)
		fmt.Fprintf(&buf, "%s\n", strings.Repeat("=", 80))

		// Run ps inside the container
		fmt.Fprintf(&buf, "\n--- PROCESSES (ps aux) ---\n")
		psOutput, err := runWithTimeout(cmdTimeoutShort, "docker", "exec", container.id, "ps", "aux")
		if err != nil {
			// Try alternative: ps without aux (some minimal containers don't have full ps)
			psOutput, err = runWithTimeout(cmdTimeoutShort, "docker", "exec", container.id, "ps", "-ef")
			if err != nil {
				fmt.Fprintf(&buf, "Error running ps: %v\n", err)
			} else {
				buf.Write(psOutput)
			}
		} else {
			buf.Write(psOutput)
		}

		// Run df inside the container
		fmt.Fprintf(&buf, "\n--- DISK USAGE (df -h) ---\n")
		dfOutput, err := runWithTimeout(cmdTimeoutShort, "docker", "exec", container.id, "df", "-h")
		if err != nil {
			fmt.Fprintf(&buf, "Error running df: %v\n", err)
		} else {
			buf.Write(dfOutput)
		}

		// Run free inside the container
		fmt.Fprintf(&buf, "\n--- MEMORY USAGE (free -h) ---\n")
		freeOutput, err := runWithTimeout(cmdTimeoutShort, "docker", "exec", container.id, "free", "-h")
		if err != nil {
			// Try without -h flag for systems that don't support it
			freeOutput, err = runWithTimeout(cmdTimeoutShort, "docker", "exec", container.id, "free")
			if err != nil {
				fmt.Fprintf(&buf, "Error running free: %v\n", err)
			} else {
				buf.Write(freeOutput)
			}
		} else {
			buf.Write(freeOutput)
		}

		// Run nerdctl to list miren containers
		fmt.Fprintf(&buf, "\n--- MIREN CONTAINERS (nerdctl) ---\n")
		nerdctlOutput, err := runWithTimeout(cmdTimeoutShort,
			"docker", "exec", container.id,
			"/var/lib/miren/release/nerdctl",
			"-a", "/var/lib/miren/containerd/containerd.sock",
			"--namespace", "miren",
			"ps", "-a", "--no-trunc")
		if err != nil {
			fmt.Fprintf(&buf, "Error running nerdctl: %v\n%s\n", err, nerdctlOutput)
		} else {
			buf.Write(nerdctlOutput)
		}
	}

	if buf.Len() == 0 {
		return nil
	}

	return buf.Bytes()
}

func gatherDockerInspect(containerName string) []byte {
	containers := findDockerContainers(containerName)
	if len(containers) == 0 {
		return nil
	}

	// Collect all container IDs to inspect
	var ids []string
	for _, container := range containers {
		ids = append(ids, container.id)
	}

	// Run docker inspect on all containers at once
	args := append([]string{"inspect"}, ids...)
	output, err := runWithTimeout(cmdTimeoutShort, "docker", args...)
	if err != nil {
		return nil
	}

	return output
}

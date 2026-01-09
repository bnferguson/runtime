//go:build linux

package commands

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"golang.org/x/sys/unix"
)

func gatherContainers(ctx *Context, socket, namespace string) ([]byte, error) {
	if socket == "" {
		socket = defaultContainerdSocket()
	}

	// Check if socket exists and is accessible before attempting connection
	if err := checkSocketAccess(socket); err != nil {
		return nil, err
	}

	// Use a timeout context for containerd operations
	timeoutCtx, cancel := context.WithTimeout(context.Background(), cmdTimeoutShort)
	defer cancel()

	client, err := containerd.New(socket)
	if err != nil {
		// Check if the error indicates a permission problem
		if isPermissionError(err) {
			return nil, fmt.Errorf("permission denied connecting to containerd at %s (try running with sudo)", socket)
		}
		return nil, fmt.Errorf("connecting to containerd at %s: %w", socket, err)
	}
	defer client.Close()

	cctx := namespaces.WithNamespace(timeoutCtx, namespace)

	containers, err := client.Containers(cctx)
	if err != nil {
		if timeoutCtx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("timed out listing containers after %s", cmdTimeoutShort)
		}
		return nil, fmt.Errorf("listing containers: %w", err)
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Namespace: %s\n", namespace)
	fmt.Fprintf(&buf, "Socket: %s\n\n", socket)
	fmt.Fprintf(&buf, "%-50s %-12s %-10s %-15s %s\n", "ID", "STATUS", "PID", "AGE", "ENTITY-ID")
	fmt.Fprintf(&buf, "%s\n", strings.Repeat("-", 115))

	for _, container := range containers {
		id := container.ID()
		if len(id) > 50 {
			id = id[:47] + "..."
		}

		status := "unknown"
		var taskPid uint32

		task, err := container.Task(cctx, nil)
		if err == nil {
			taskStatus, err := task.Status(cctx)
			if err == nil {
				status = string(taskStatus.Status)
			}
			taskPid = task.Pid()
		} else {
			status = "no-task"
		}

		labels, _ := container.Labels(cctx)
		entityID := labels["runtime.computer/entity-id"]

		pidStr := "-"
		if taskPid > 0 {
			pidStr = strconv.FormatUint(uint64(taskPid), 10)
		}

		// Get container creation time
		age := "-"
		if info, err := container.Info(cctx); err == nil {
			age = formatAge(time.Since(info.CreatedAt))
		}

		fmt.Fprintf(&buf, "%-50s %-12s %-10s %-15s %s\n", id, status, pidStr, age, entityID)
	}

	if len(containers) == 0 {
		fmt.Fprintf(&buf, "(no containers found)\n")
	}

	return buf.Bytes(), nil
}

// checkSocketAccess verifies the socket exists and is accessible.
// Returns a user-friendly error if permission is denied.
func checkSocketAccess(socket string) error {
	// Check if socket exists
	if _, err := os.Stat(socket); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("containerd socket not found at %s (is containerd running?)", socket)
		}
		if os.IsPermission(err) {
			return fmt.Errorf("permission denied accessing containerd socket at %s (try running with sudo)", socket)
		}
		return fmt.Errorf("checking containerd socket at %s: %w", socket, err)
	}

	// Check if we can actually connect to the socket
	if err := unix.Access(socket, unix.R_OK|unix.W_OK); err != nil {
		if errors.Is(err, unix.EACCES) || errors.Is(err, unix.EPERM) || os.IsPermission(err) {
			return fmt.Errorf("permission denied accessing containerd socket at %s (try running with sudo)", socket)
		}
		return fmt.Errorf("checking containerd socket permissions at %s: %w", socket, err)
	}

	return nil
}

// isPermissionError checks if an error indicates a permission problem.
func isPermissionError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "permission denied") ||
		strings.Contains(errStr, "Permission denied")
}

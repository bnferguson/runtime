//go:build linux

package commands

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/namespaces"
)

func gatherContainers(ctx *Context, socket, namespace string) ([]byte, error) {
	if socket == "" {
		socket = defaultContainerdSocket()
	}

	client, err := containerd.New(socket)
	if err != nil {
		return nil, fmt.Errorf("connecting to containerd at %s: %w", socket, err)
	}
	defer client.Close()

	cctx := namespaces.WithNamespace(context.Background(), namespace)

	containers, err := client.Containers(cctx)
	if err != nil {
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

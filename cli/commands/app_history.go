package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"miren.dev/runtime/api/deployment/deployment_v1alpha"
	"miren.dev/runtime/pkg/ui"
)

func AppHistory(ctx *Context, opts struct {
	AppCentric

	Limit      int    `short:"n" long:"limit" description:"Maximum number of deployments to show" default:"10"`
	Status     string `short:"s" long:"status" description:"Filter by status (active, failed, rolled_back)"`
	All        bool   `long:"all" description:"Show deployments from all clusters"`
	ShowFailed bool   `long:"show-failed" description:"Include failed deployments (shown by default)"`
	HideFailed bool   `long:"hide-failed" description:"Hide failed deployments"`
	Detailed   bool   `long:"detailed" description:"Show all columns including git information"`
}) error {
	// Connect to deployment service
	depCl, err := ctx.RPCClient("dev.miren.runtime/deployment")
	if err != nil {
		return fmt.Errorf("failed to connect to deployment service: %w", err)
	}
	depClient := deployment_v1alpha.NewDeploymentClient(depCl)

	// Get cluster filter
	clusterId := ""
	if !opts.All {
		clusterId = ctx.ClusterName
	}

	// List deployments
	result, err := depClient.ListDeployments(ctx, opts.App, clusterId, opts.Status, int32(opts.Limit))
	if err != nil {
		return fmt.Errorf("failed to list deployments: %w", err)
	}

	if !result.HasDeployments() || len(result.Deployments()) == 0 {
		ctx.Printf("No deployments found for app '%s'", opts.App)
		if !opts.All {
			ctx.Printf(" on cluster '%s'", ctx.ClusterName)
		}
		if opts.Status != "" {
			ctx.Printf(" with status '%s'", opts.Status)
		}
		ctx.Printf("\n")
		return nil
	}

	// Display deployments
	deployments := result.Deployments()

	// Filter out failed deployments if requested
	if opts.HideFailed {
		var filtered []*deployment_v1alpha.DeploymentInfo
		for _, dep := range deployments {
			if dep.Status() != "failed" {
				filtered = append(filtered, dep)
			}
		}
		deployments = filtered

		// Check if all deployments were filtered out
		if len(deployments) == 0 {
			ctx.Printf("No deployments found for app '%s'", opts.App)
			if !opts.All {
				ctx.Printf(" on cluster '%s'", ctx.ClusterName)
			}
			if opts.Status != "" {
				ctx.Printf(" with status '%s'", opts.Status)
			}
			ctx.Printf(" (failed deployments hidden)\n")
			return nil
		}
	}

	// Header
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	ctx.Printf("%s\n", headerStyle.Render(fmt.Sprintf("Deployment History for %s", opts.App)))
	if !opts.All {
		ctx.Printf("Cluster: %s\n", ctx.ClusterName)
	}
	ctx.Printf("\n")

	// Status styles
	activeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10"))     // Green
	succeededStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("12"))  // Blue
	failedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))      // Red
	rolledBackStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("11")) // Yellow
	inProgressStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("14")) // Cyan
	cancelledStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("11"))  // Yellow

	// Build headers and rows based on detailed mode
	var headers []string
	var rows []ui.Row

	if opts.Detailed {
		headers = []string{"STATUS", "CLUSTER", "VERSION", "DEPLOYED BY", "WHEN", "GIT SHA", "BRANCH", "COMMIT MESSAGE"}
	} else {
		headers = []string{"STATUS", "VERSION", "DEPLOYED BY", "WHEN", "ID", "ERROR"}
	}

	for _, dep := range deployments {
		// Format status with color and icons
		status := dep.Status()
		var styledStatus string
		switch status {
		case "active":
			styledStatus = activeStyle.Render("✓ " + status)
		case "succeeded":
			styledStatus = succeededStyle.Render("✓ " + status)
		case "failed":
			styledStatus = failedStyle.Render("✗ " + status)
		case "rolled_back":
			styledStatus = rolledBackStyle.Render("↩ " + status)
		case "in_progress":
			styledStatus = inProgressStyle.Render("⟳ " + status)
		case "cancelled":
			styledStatus = cancelledStyle.Render("⊘ " + status)
		default:
			styledStatus = status
		}

		// Format timestamp
		timeStr := "unknown"
		if dep.HasDeployedAt() && dep.DeployedAt() != nil {
			deployedAt := time.Unix(dep.DeployedAt().Seconds(), 0)
			timeStr = formatRelativeTime(deployedAt)
		}

		// Format cluster
		cluster := dep.ClusterId()
		if cluster == "" {
			cluster = "default"
		}

		// Format version (handle special patterns)
		version := dep.AppVersionId()
		if strings.HasPrefix(version, "pending-") {
			version = "pending (building...)"
		} else if strings.HasPrefix(version, "failed-") {
			version = "failed (no version)"
		} else if len(version) > 25 {
			version = version[:22] + "..."
		}

		// Format user (prefer name, fallback to email, then user ID)
		user := ""
		if dep.HasDeployedByUserName() && dep.DeployedByUserName() != "" {
			user = dep.DeployedByUserName()
		} else if dep.DeployedByUserEmail() != "" && dep.DeployedByUserEmail() != "unknown@example.com" && dep.DeployedByUserEmail() != "user@example.com" {
			user = dep.DeployedByUserEmail()
		} else if dep.DeployedByUserId() != "" {
			user = dep.DeployedByUserId()
		} else {
			user = "-"
		}

		// Format git info
		gitSha := "-"
		gitBranch := "-"
		gitMessage := "-"

		if dep.HasGitInfo() && dep.GitInfo() != nil {
			git := dep.GitInfo()
			if git.HasSha() && git.Sha() != "" {
				gitSha = git.Sha()
				if len(gitSha) > 10 {
					gitSha = gitSha[:10]
				}
				// Append -dirty if working tree was dirty
				if git.HasIsDirty() && git.IsDirty() {
					gitSha += "-dirty"
				}
			}
			if git.HasBranch() && git.Branch() != "" {
				gitBranch = git.Branch()
			}
			if git.HasCommitMessage() && git.CommitMessage() != "" {
				gitMessage = strings.TrimSpace(git.CommitMessage())
				if idx := strings.Index(gitMessage, "\n"); idx > 0 {
					gitMessage = gitMessage[:idx]
				}
			}
		}

		// Format error/phase info for ERROR column
		errorInfo := "-"
		if status == "in_progress" && dep.HasPhase() && dep.Phase() != "" {
			errorInfo = inProgressStyle.Render(dep.Phase())
		} else if (status == "failed" || status == "cancelled") && dep.HasErrorMessage() && dep.ErrorMessage() != "" {
			errorInfo = failedStyle.Render(dep.ErrorMessage())
		}

		var row ui.Row
		if opts.Detailed {
			row = ui.Row{styledStatus, cluster, version, user, timeStr, gitSha, gitBranch, gitMessage}
		} else {
			deploymentId := ui.CleanEntityID(dep.Id())
			row = ui.Row{styledStatus, version, user, timeStr, deploymentId, errorInfo}
		}
		rows = append(rows, row)
	}

	// Configure column hints
	var builder *ui.ColumnBuilder
	if opts.Detailed {
		// In detailed mode, protect STATUS from truncation, allow others to scale
		builder = ui.Columns().
			NoTruncate(0).  // STATUS
			MaxWidth(7, 40) // COMMIT MESSAGE
	} else {
		// In normal mode, protect STATUS and ID, allow ERROR to scale
		builder = ui.Columns().
			NoTruncate(0, 4) // STATUS and ID
	}

	columns := ui.AutoSizeColumns(headers, rows, builder)
	table := ui.NewTable(
		ui.WithColumns(columns),
		ui.WithRows(rows),
	)

	ctx.Printf("%s\n", table.Render())
	return nil
}

// formatRelativeTime formats a time as a relative string (e.g. "2 hours ago")
func formatRelativeTime(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		return fmt.Sprintf("%dm ago", mins)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		return fmt.Sprintf("%dh ago", hours)
	case diff < 7*24*time.Hour:
		days := int(diff.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	default:
		return t.Format("Jan 2")
	}
}

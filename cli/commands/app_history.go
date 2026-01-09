package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"miren.dev/runtime/api/deployment/deployment_v1alpha"
	"miren.dev/runtime/pkg/ui"
)

// Deployment status styles
var statusStyles = map[string]struct {
	icon  string
	style lipgloss.Style
}{
	"active":      {"✓", lipgloss.NewStyle().Foreground(lipgloss.Color("10"))}, // Green
	"succeeded":   {"✓", lipgloss.NewStyle().Foreground(lipgloss.Color("12"))}, // Blue
	"failed":      {"✗", lipgloss.NewStyle().Foreground(lipgloss.Color("9"))},  // Red
	"rolled_back": {"↩", lipgloss.NewStyle().Foreground(lipgloss.Color("11"))}, // Yellow
	"in_progress": {"⟳", lipgloss.NewStyle().Foreground(lipgloss.Color("14"))}, // Cyan
	"cancelled":   {"⊘", lipgloss.NewStyle().Foreground(lipgloss.Color("11"))}, // Yellow
}

type historyDisplayOpts struct {
	detailed bool
}

func AppHistory(ctx *Context, opts struct {
	AppCentric

	Limit      int32  `short:"n" long:"limit" description:"Maximum number of deployments to show" default:"20"`
	Status     string `short:"s" long:"status" description:"Filter by status (active, failed, rolled_back)"`
	All        bool   `long:"all" description:"Show deployments from all clusters"`
	HideFailed bool   `long:"hide-failed" description:"Hide failed deployments"`
	Detailed   bool   `long:"detailed" description:"Show all columns including git information"`
}) error {
	depCl, err := ctx.RPCClient("dev.miren.runtime/deployment")
	if err != nil {
		return fmt.Errorf("failed to connect to deployment service: %w", err)
	}
	depClient := deployment_v1alpha.NewDeploymentClient(depCl)

	clusterId := ""
	if !opts.All {
		clusterId = ctx.ClusterName
	}

	result, err := depClient.ListDeployments(ctx, opts.App, clusterId, opts.Status, opts.Limit)
	if err != nil {
		return fmt.Errorf("failed to list deployments: %w", err)
	}

	deployments := result.Deployments()
	if len(deployments) == 0 {
		printNoDeploymentsMessage(ctx, opts.App, opts.All, opts.Status, false)
		return nil
	}

	// Filter out failed deployments if requested
	if opts.HideFailed {
		deployments = filterDeployments(deployments, func(d *deployment_v1alpha.DeploymentInfo) bool {
			return d.Status() != "failed"
		})
		if len(deployments) == 0 {
			printNoDeploymentsMessage(ctx, opts.App, opts.All, opts.Status, true)
			return nil
		}
	}

	// Print header
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	ctx.Printf("%s\n", headerStyle.Render(fmt.Sprintf("Deployment History for %s", opts.App)))
	if !opts.All {
		ctx.Printf("Cluster: %s\n", ctx.ClusterName)
	}
	ctx.Printf("\n")

	displayOpts := historyDisplayOpts{
		detailed: opts.Detailed,
	}

	// Build and render table
	headers, rows, builder := buildDeploymentTable(deployments, displayOpts)
	columns := ui.AutoSizeColumns(headers, rows, builder)
	table := ui.NewTable(ui.WithColumns(columns), ui.WithRows(rows))
	ctx.Printf("%s\n", table.Render())
	return nil
}

func printNoDeploymentsMessage(ctx *Context, app string, all bool, status string, hiddenFailed bool) {
	ctx.Printf("No deployments found for app '%s'", app)
	if !all {
		ctx.Printf(" on cluster '%s'", ctx.ClusterName)
	}
	if status != "" {
		ctx.Printf(" with status '%s'", status)
	}
	if hiddenFailed {
		ctx.Printf(" (failed deployments hidden)")
	}
	ctx.Printf("\n")
}

func filterDeployments(deps []*deployment_v1alpha.DeploymentInfo, keep func(*deployment_v1alpha.DeploymentInfo) bool) []*deployment_v1alpha.DeploymentInfo {
	var filtered []*deployment_v1alpha.DeploymentInfo
	for _, d := range deps {
		if keep(d) {
			filtered = append(filtered, d)
		}
	}
	return filtered
}

func buildDeploymentTable(deployments []*deployment_v1alpha.DeploymentInfo, opts historyDisplayOpts) ([]string, []ui.Row, *ui.ColumnBuilder) {
	var headers []string
	var rows []ui.Row
	var builder *ui.ColumnBuilder

	if opts.detailed {
		headers = []string{"STATUS", "VERSION", "DEPLOYED BY", "WHEN", "GIT SHA", "BRANCH", "COMMIT MESSAGE"}
		builder = ui.Columns().
			NoTruncate(0).               // STATUS
			MaxWidth(len(headers)-1, 40) // COMMIT MESSAGE
	} else {
		headers = []string{"STATUS", "VERSION", "DEPLOYED BY", "WHEN", "ID", "ERROR"}
		builder = ui.Columns().
			NoTruncate(0, 4) // STATUS and ID
	}

	for _, dep := range deployments {
		row := buildDeploymentRow(dep, opts)
		rows = append(rows, row)
	}

	return headers, rows, builder
}

func buildDeploymentRow(dep *deployment_v1alpha.DeploymentInfo, opts historyDisplayOpts) ui.Row {
	status := dep.Status()

	row := ui.Row{
		formatDeploymentStatus(status),
	}

	if opts.detailed {
		row = append(row,
			formatVersion(dep.AppVersionId(), status),
			formatUser(dep),
			formatDeploymentTime(dep),
		)
		gitSha, gitBranch, gitMessage := formatGitInfo(dep)
		row = append(row, gitSha, gitBranch, gitMessage)
	} else {
		row = append(row,
			formatVersion(dep.AppVersionId(), status),
			formatUser(dep),
			formatDeploymentTime(dep),
			ui.CleanEntityID(dep.Id()),
			formatErrorInfo(dep, status),
		)
	}

	return row
}

func formatDeploymentStatus(status string) string {
	if s, ok := statusStyles[status]; ok {
		return s.style.Render(s.icon + " " + status)
	}
	return status
}

func formatVersion(version, status string) string {
	if strings.HasPrefix(version, "pending-") {
		if status == "in_progress" {
			return "pending (building...)"
		}
		return "-"
	}
	if strings.HasPrefix(version, "failed-") {
		return "-"
	}
	return version
}

func formatDeploymentTime(dep *deployment_v1alpha.DeploymentInfo) string {
	if dep.HasDeployedAt() && dep.DeployedAt() != nil {
		return formatRelativeTime(time.Unix(dep.DeployedAt().Seconds(), 0))
	}
	return "unknown"
}

func formatUser(dep *deployment_v1alpha.DeploymentInfo) string {
	if dep.HasDeployedByUserName() && dep.DeployedByUserName() != "" {
		return dep.DeployedByUserName()
	}
	return "-"
}

func formatGitInfo(dep *deployment_v1alpha.DeploymentInfo) (sha, branch, message string) {
	sha, branch, message = "-", "-", "-"

	if !dep.HasGitInfo() || dep.GitInfo() == nil {
		return
	}

	git := dep.GitInfo()
	if git.HasSha() && git.Sha() != "" {
		sha = git.Sha()
		if len(sha) > 10 {
			sha = sha[:10]
		}
		if git.HasIsDirty() && git.IsDirty() {
			sha += "-dirty"
		}
	}
	if git.HasBranch() && git.Branch() != "" {
		branch = git.Branch()
	}
	if git.HasCommitMessage() && git.CommitMessage() != "" {
		message = strings.TrimSpace(git.CommitMessage())
		if idx := strings.Index(message, "\n"); idx > 0 {
			message = message[:idx]
		}
	}
	return
}

func formatErrorInfo(dep *deployment_v1alpha.DeploymentInfo, status string) string {
	if status == "in_progress" && dep.HasPhase() && dep.Phase() != "" {
		return statusStyles["in_progress"].style.Render(firstLine(dep.Phase()))
	}
	if dep.HasErrorMessage() && dep.ErrorMessage() != "" {
		msg := firstLine(dep.ErrorMessage())
		switch status {
		case "failed":
			return statusStyles["failed"].style.Render(msg)
		case "cancelled":
			return statusStyles["cancelled"].style.Render(msg)
		}
	}
	return "-"
}

func firstLine(s string) string {
	if idx := strings.Index(s, "\n"); idx > 0 {
		return s[:idx]
	}
	return s
}

func formatRelativeTime(t time.Time) string {
	diff := time.Since(t)

	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		return fmt.Sprintf("%dm ago", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	case diff < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(diff.Hours()/24))
	default:
		return t.Format("Jan 2")
	}
}

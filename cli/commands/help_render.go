package commands

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"miren.dev/mflags"
)

// RenderTopLevelHelp renders grouped help output for the top-level command list.
// Commands are rendered in the order defined by HelpGroupOrder. Commands in
// GroupHidden are filtered out.
func RenderTopLevelHelp(d *mflags.Dispatcher) {
	children := d.GetDirectChildren("")

	grouped := make(map[string][]mflags.ChildEntry)
	for _, child := range children {
		if child.Group == GroupHidden {
			continue
		}
		grouped[child.Group] = append(grouped[child.Group], child)
	}

	// Compute max name length across rendered commands for column alignment.
	maxLen := 0
	for _, group := range HelpGroupOrder {
		for _, child := range grouped[group] {
			if len(child.Name) > maxLen {
				maxLen = len(child.Name)
			}
		}
	}

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("220"))

	fmt.Printf("Usage: %s <command> [arguments]\n", d.Name())

	for _, group := range HelpGroupOrder {
		entries := grouped[group]
		if len(entries) == 0 {
			continue
		}
		fmt.Println()
		fmt.Println(headerStyle.Render(fmt.Sprintf("%s:", group)))
		for _, child := range entries {
			fmt.Println(formatHelpLine(d, child, maxLen))
		}
	}

	fmt.Println()
	fmt.Printf("Use '%s help <command>' or '%s <command> --help' for more information.\n", d.Name(), d.Name())
}

// formatHelpLine formats a single command entry for the help listing.
func formatHelpLine(d *mflags.Dispatcher, child mflags.ChildEntry, maxLen int) string {
	grandchildren := d.GetDirectChildren(child.Path)

	faint := lipgloss.NewStyle().Faint(true)

	suffix := ""
	if len(grandchildren) > 0 {
		suffix = " " + faint.Render(subCommandsLabel(len(grandchildren)))
	}

	if child.Usage != "" {
		return fmt.Sprintf("  %-*s  %s%s", maxLen+2, child.Name, child.Usage, suffix)
	}
	if suffix != "" {
		return fmt.Sprintf("  %-*s %s", maxLen+2, child.Name, suffix)
	}
	return fmt.Sprintf("  %s", child.Name)
}

func subCommandsLabel(n int) string {
	if n == 1 {
		return "(1 sub-command)"
	}
	return fmt.Sprintf("(%d sub-commands)", n)
}

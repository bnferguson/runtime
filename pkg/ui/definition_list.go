package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"miren.dev/runtime/pkg/theme"
)

// Definition represents a single definition entry with a term, description, and optional details
type Definition struct {
	Term        string
	Description string
	Details     []DefinitionDetail
}

// DefinitionDetail represents a child item in a definition (displayed as a tree)
type DefinitionDetail struct {
	Name     string
	Type     string
	Required bool
}

// DefinitionList renders a list of definitions with tree-style details
type DefinitionList struct {
	title  string
	items  []Definition
	styles DefinitionListStyles
}

// DefinitionListStyles contains the styling configuration
type DefinitionListStyles struct {
	Title       lipgloss.Style
	Term        lipgloss.Style
	Description lipgloss.Style
	DetailName  lipgloss.Style
	DetailType  lipgloss.Style
	Required    lipgloss.Style
	TreeLine    lipgloss.Style
}

// DefaultDefinitionListStyles returns the default styling
func DefaultDefinitionListStyles() DefinitionListStyles {
	return DefinitionListStyles{
		Title:       lipgloss.NewStyle().Bold(true).Foreground(theme.Info),
		Term:        lipgloss.NewStyle().Bold(true).Foreground(theme.Success),
		Description: lipgloss.NewStyle().Italic(true),
		DetailName:  lipgloss.NewStyle().Foreground(theme.Info),
		DetailType:  lipgloss.NewStyle().Foreground(theme.Highlight),
		Required:    lipgloss.NewStyle().Foreground(theme.Error),
		TreeLine:    lipgloss.NewStyle().Foreground(theme.Muted),
	}
}

// DefinitionListOption is a function that configures a DefinitionList
type DefinitionListOption func(*DefinitionList)

// WithDefinitionListStyles sets custom styles
func WithDefinitionListStyles(styles DefinitionListStyles) DefinitionListOption {
	return func(d *DefinitionList) {
		d.styles = styles
	}
}

// WithDefinitionListTitle sets the title
func WithDefinitionListTitle(title string) DefinitionListOption {
	return func(d *DefinitionList) {
		d.title = title
	}
}

// NewDefinitionList creates a new definition list
func NewDefinitionList(items []Definition, opts ...DefinitionListOption) *DefinitionList {
	d := &DefinitionList{
		items:  items,
		styles: DefaultDefinitionListStyles(),
	}

	for _, opt := range opts {
		opt(d)
	}

	return d
}

// Render generates the string representation
func (d *DefinitionList) Render() string {
	if len(d.items) == 0 {
		return ""
	}

	var sb strings.Builder

	// Title
	if d.title != "" {
		sb.WriteString(d.styles.Title.Render(d.title))
		sb.WriteString("\n\n")
	}

	// Items
	for i, item := range d.items {
		// Term
		sb.WriteString("  ")
		sb.WriteString(d.styles.Term.Render(item.Term))
		sb.WriteString("\n")

		// Description
		if item.Description != "" {
			sb.WriteString("  ")
			sb.WriteString(d.styles.TreeLine.Render("│"))
			sb.WriteString(" ")
			sb.WriteString(d.styles.Description.Render(item.Description))
			sb.WriteString("\n")
		}

		// Details (tree-style)
		for j, detail := range item.Details {
			prefix := "├"
			if j == len(item.Details)-1 {
				prefix = "└"
			}

			sb.WriteString("  ")
			sb.WriteString(d.styles.TreeLine.Render(prefix))
			sb.WriteString(" ")
			sb.WriteString(d.styles.DetailName.Render(detail.Name))
			sb.WriteString(" ")
			sb.WriteString(d.styles.DetailType.Render(detail.Type))

			if detail.Required {
				sb.WriteString(" ")
				sb.WriteString(d.styles.Required.Render("(required)"))
			}

			sb.WriteString("\n")
		}

		// Spacing between items (except last)
		if i < len(d.items)-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

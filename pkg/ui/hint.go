package ui

import "github.com/charmbracelet/lipgloss"

// Hint renders a dimmed hint/tip message
type Hint struct {
	text  string
	style lipgloss.Style
}

// NewHint creates a new hint with the given text
func NewHint(text string) *Hint {
	return &Hint{
		text:  text,
		style: lipgloss.NewStyle().Faint(true),
	}
}

// WithStyle sets a custom style for the hint
func (h *Hint) WithStyle(style lipgloss.Style) *Hint {
	h.style = style
	return h
}

// Render generates the string representation
func (h *Hint) Render() string {
	return h.style.Render(h.text)
}

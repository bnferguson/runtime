package ui

import (
	"fmt"
	"os"
	"regexp"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

var mdLinkPattern = regexp.MustCompile(`^\[(.+)\]\((.+)\)$`)

// IsTTY reports whether stdout is connected to a terminal.
func IsTTY() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// TerminalWidth returns the current terminal width, or 0 if stdout is not a TTY.
func TerminalWidth() int {
	if !IsTTY() {
		return 0
	}
	if width, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && width > 0 {
		return width
	}
	return 80
}

// Hyperlink creates a clickable terminal hyperlink using the OSC 8 escape sequence
// with underline styling. Raw SGR sequences are used so the result can be safely
// combined with lipgloss-rendered text without interference.
//
// We use a plain underline (\x1b[4m) rather than the colon-style dotted underline
// (\x1b[4:4m): Terminal.app and other terminals don't understand colon SGR
// subparameters and misrender them as a stray background that bleeds across the
// line.
//
// When stdout is not a TTY, returns just the text with no escape sequences.
func Hyperlink(url, text string) string {
	if !IsTTY() {
		return text
	}
	return fmt.Sprintf("\x1b]8;;%s\x1b\\\x1b[4m%s\x1b[24m\x1b]8;;\x1b\\", url, text)
}

// HyperlinkStyled is like Hyperlink but also applies a foreground color, which
// adapts to the terminal's profile and background via the theme's color roles.
//
// When stdout is not a TTY, returns just the text with no escape sequences.
func HyperlinkStyled(url, text string, color lipgloss.TerminalColor) string {
	if !IsTTY() {
		return text
	}
	return fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b[4m%s\x1b[24;39m\x1b]8;;\x1b\\", url, foregroundSeq(color), text)
}

// foregroundSeq returns the SGR escape that sets the foreground to color for the
// active color profile (e.g. "\x1b[38;2;194;65;12m"), or "" when color is off.
// Raw OSC 8 hyperlinks can't be rendered through a lipgloss style without
// disturbing the hyperlink sequences, so we emit the color escape by hand.
func foregroundSeq(color lipgloss.TerminalColor) string {
	seq := lipgloss.ColorProfile().FromColor(color).Sequence(false)
	if seq == "" {
		return ""
	}
	return "\x1b[" + seq + "m"
}

// RenderMarkdownLink parses a markdown-style link "[text](url)" and renders it
// as a clickable terminal hyperlink in the given theme color.
//
// When stdout is not a TTY, renders as "text (url)" for readability in pipes/logs.
// If the input isn't a valid markdown link, it's returned as-is.
func RenderMarkdownLink(s string, color lipgloss.TerminalColor) string {
	m := mdLinkPattern.FindStringSubmatch(s)
	if m == nil {
		return s
	}
	if !IsTTY() {
		return fmt.Sprintf("%s (%s)", m[1], m[2])
	}
	return HyperlinkStyled(m[2], m[1], color)
}

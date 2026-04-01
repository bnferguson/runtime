package ui

import (
	"fmt"
	"os"
	"regexp"

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
// with dotted underline styling. Raw SGR sequences are used so the result can be
// safely combined with lipgloss-rendered text without interference.
//
// When stdout is not a TTY, returns just the text with no escape sequences.
func Hyperlink(url, text string) string {
	if !IsTTY() {
		return text
	}
	return fmt.Sprintf("\x1b]8;;%s\x1b\\\x1b[4:4m%s\x1b[24m\x1b]8;;\x1b\\", url, text)
}

// HyperlinkStyled is like Hyperlink but also applies a foreground color (256-color index).
//
// When stdout is not a TTY, returns just the text with no escape sequences.
func HyperlinkStyled(url, text string, color int) string {
	if !IsTTY() {
		return text
	}
	return fmt.Sprintf("\x1b]8;;%s\x1b\\\x1b[38;5;%dm\x1b[4:4m%s\x1b[24;39m\x1b]8;;\x1b\\", url, color, text)
}

// RenderMarkdownLink parses a markdown-style link "[text](url)" and renders it
// as a clickable terminal hyperlink with the given 256-color index.
//
// When stdout is not a TTY, renders as "text (url)" for readability in pipes/logs.
// If the input isn't a valid markdown link, it's returned as-is.
func RenderMarkdownLink(s string, color int) string {
	m := mdLinkPattern.FindStringSubmatch(s)
	if m == nil {
		return s
	}
	if !IsTTY() {
		return fmt.Sprintf("%s (%s)", m[1], m[2])
	}
	return HyperlinkStyled(m[2], m[1], color)
}

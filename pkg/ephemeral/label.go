package ephemeral

import (
	"fmt"
	"regexp"
	"strings"
)

const maxLabelLength = 63

var validLabelRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

// NormalizeLabel converts an input string (typically a Git branch name) into a
// DNS-compliant label suitable for use as a subdomain. The rules follow RFC 1123:
// lowercase alphanumeric and hyphens, starting and ending with alphanumeric,
// max 63 characters.
func NormalizeLabel(input string) (string, error) {
	s := strings.ToLower(input)

	// Replace common separators with hyphens
	s = strings.NewReplacer("_", "-", "/", "-", ".", "-").Replace(s)

	// Remove characters that aren't alphanumeric or hyphens
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	s = b.String()

	// Collapse consecutive hyphens
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}

	// Trim leading and trailing hyphens
	s = strings.Trim(s, "-")

	// Truncate to max length
	if len(s) > maxLabelLength {
		s = s[:maxLabelLength]
		// Trim trailing hyphen created by truncation
		s = strings.TrimRight(s, "-")
	}

	if s == "" {
		return "", fmt.Errorf("label is empty after normalization of %q", input)
	}

	return s, nil
}

// ValidateLabel checks that a label conforms to DNS label rules (RFC 1123).
func ValidateLabel(label string) error {
	if label == "" {
		return fmt.Errorf("label must not be empty")
	}
	if len(label) > maxLabelLength {
		return fmt.Errorf("label must be at most %d characters, got %d", maxLabelLength, len(label))
	}
	if !validLabelRe.MatchString(label) {
		return fmt.Errorf("label %q must contain only lowercase alphanumeric characters and hyphens, and must start and end with an alphanumeric character", label)
	}
	return nil
}

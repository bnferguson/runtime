package ephemeral

import (
	"strings"
	"testing"
)

func TestNormalizeLabel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"feature/login", "feature-login"},
		{"Feature_Login_Page", "feature-login-page"},
		{"fix/nav..bar", "fix-nav-bar"},
		{"feat-x", "feat-x"},
		{"MAIN", "main"},
		{"a", "a"},
		{"hello_world/test.branch", "hello-world-test-branch"},
		{"--leading-trailing--", "leading-trailing"},
		{"multiple___underscores", "multiple-underscores"},
		{"a/b/c/d", "a-b-c-d"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := NormalizeLabel(tt.input)
			if err != nil {
				t.Fatalf("NormalizeLabel(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("NormalizeLabel(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeLabelTruncation(t *testing.T) {
	long := strings.Repeat("a", 100)
	got, err := NormalizeLabel(long)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) > 63 {
		t.Errorf("expected length <= 63, got %d", len(got))
	}
}

func TestNormalizeLabelTruncationTrailingHyphen(t *testing.T) {
	// 63rd character would be a hyphen after truncation
	input := strings.Repeat("a", 62) + "-b"
	got, err := NormalizeLabel(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) > 63 {
		t.Errorf("expected length <= 63, got %d", len(got))
	}
	if strings.HasSuffix(got, "-") {
		t.Errorf("label should not end with hyphen: %q", got)
	}
}

func TestNormalizeLabelEmpty(t *testing.T) {
	inputs := []string{"", "---", "...", "///"}
	for _, input := range inputs {
		_, err := NormalizeLabel(input)
		if err == nil {
			t.Errorf("NormalizeLabel(%q) expected error, got nil", input)
		}
	}
}

func TestValidateLabel(t *testing.T) {
	valid := []string{"a", "feat-x", "my-branch-123", strings.Repeat("a", 63)}
	for _, label := range valid {
		if err := ValidateLabel(label); err != nil {
			t.Errorf("ValidateLabel(%q) unexpected error: %v", label, err)
		}
	}

	invalid := []string{"", "-start", "end-", "UPPER", "has_underscore", "has.dot", strings.Repeat("a", 64)}
	for _, label := range invalid {
		if err := ValidateLabel(label); err == nil {
			t.Errorf("ValidateLabel(%q) expected error, got nil", label)
		}
	}
}

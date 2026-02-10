package joincode

import (
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	code, err := Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if !Validate(code) {
		t.Errorf("Generated code %q does not validate", code)
	}

	parts := strings.Split(code, "-")
	if len(parts) != 3 {
		t.Fatalf("Expected 3 parts, got %d: %q", len(parts), code)
	}

	if len(parts[2]) != 4 {
		t.Errorf("Expected 4-char suffix, got %d: %q", len(parts[2]), parts[2])
	}
}

func TestGenerateUniqueness(t *testing.T) {
	codes := make(map[string]bool)
	for range 100 {
		code, err := Generate()
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		if codes[code] {
			t.Errorf("Duplicate code generated: %q", code)
		}
		codes[code] = true
	}
}

func TestHash(t *testing.T) {
	code := "calm-river-x7k2"

	hash1 := Hash(code)
	hash2 := Hash(code)

	if hash1 != hash2 {
		t.Errorf("Hash should be deterministic: got %q and %q", hash1, hash2)
	}

	if len(hash1) != 64 {
		t.Errorf("Expected 64-char hex SHA-256 hash, got %d chars", len(hash1))
	}

	hash3 := Hash("  CALM-RIVER-X7K2  ")
	if hash1 != hash3 {
		t.Errorf("Hash should normalize case and whitespace")
	}

	hash4 := Hash("different-code-abcd")
	if hash1 == hash4 {
		t.Errorf("Different codes should have different hashes")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		code  string
		valid bool
	}{
		{"calm-river-x7k2", true},
		{"CALM-RIVER-X7K2", true},
		{"bright-forest-a2b3", true},
		{"  golden-leaf-mnp4  ", true},
		{"invalid", false},
		{"too-many-parts-here", false},
		{"no-suffix", false},
		{"bad-suffix-toolong", false},
		{"bad-suffix-ab", false},
		{"has_underscores-river-ab12", false},
		{"123-number-start", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			if got := Validate(tt.code); got != tt.valid {
				t.Errorf("Validate(%q) = %v, want %v", tt.code, got, tt.valid)
			}
		})
	}
}

func TestWordlistsNotEmpty(t *testing.T) {
	if len(adjectives) < 50 {
		t.Errorf("Expected at least 50 adjectives, got %d", len(adjectives))
	}
	if len(nouns) < 50 {
		t.Errorf("Expected at least 50 nouns, got %d", len(nouns))
	}
}

func TestWordlistsLowercase(t *testing.T) {
	for _, adj := range adjectives {
		if adj != strings.ToLower(adj) {
			t.Errorf("Adjective %q is not lowercase", adj)
		}
	}
	for _, noun := range nouns {
		if noun != strings.ToLower(noun) {
			t.Errorf("Noun %q is not lowercase", noun)
		}
	}
}

func TestIsUUID(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"550e8400-e29b-41d4-a716-446655440000", true},
		{"550E8400-E29B-41D4-A716-446655440000", true},
		{"6ba7b810-9dad-11d1-80b4-00c04fd430c8", true},
		{"550e8400e29b41d4a716446655440000", true}, // UUID without hyphens is valid
		{"miren", false},
		{"", false},
		{"not-a-uuid", false},
		{"calm-river-x7k2", false},
		{"550e8400-e29b-41d4-a716", false}, // truncated UUID
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := IsUUID(tt.input); got != tt.expected {
				t.Errorf("IsUUID(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

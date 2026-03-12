package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseEnvVarSpecs(t *testing.T) {
	t.Run("parses KEY=VALUE format", func(t *testing.T) {
		specs, err := ParseEnvVarSpecs([]string{"FOO=bar", "BAZ=qux"}, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(specs) != 2 {
			t.Fatalf("expected 2 specs, got %d", len(specs))
		}
		if specs[0].Key != "FOO" || specs[0].Value != "bar" || specs[0].Sensitive {
			t.Errorf("unexpected first spec: %+v", specs[0])
		}
		if specs[1].Key != "BAZ" || specs[1].Value != "qux" || specs[1].Sensitive {
			t.Errorf("unexpected second spec: %+v", specs[1])
		}
	})

	t.Run("parses sensitive vars", func(t *testing.T) {
		specs, err := ParseEnvVarSpecs(nil, []string{"SECRET=hidden"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(specs) != 1 {
			t.Fatalf("expected 1 spec, got %d", len(specs))
		}
		if specs[0].Key != "SECRET" || specs[0].Value != "hidden" || !specs[0].Sensitive {
			t.Errorf("unexpected spec: %+v", specs[0])
		}
	})

	t.Run("combines regular and sensitive vars", func(t *testing.T) {
		specs, err := ParseEnvVarSpecs([]string{"FOO=bar"}, []string{"SECRET=hidden"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(specs) != 2 {
			t.Fatalf("expected 2 specs, got %d", len(specs))
		}
		// Regular vars come first
		if specs[0].Key != "FOO" || specs[0].Sensitive {
			t.Errorf("expected first spec to be regular var FOO: %+v", specs[0])
		}
		// Sensitive vars come after
		if specs[1].Key != "SECRET" || !specs[1].Sensitive {
			t.Errorf("expected second spec to be sensitive var SECRET: %+v", specs[1])
		}
	})

	t.Run("reads value from file with @", func(t *testing.T) {
		// Create a temp file with content
		tmpDir := t.TempDir()
		tmpFile := filepath.Join(tmpDir, "secret.txt")
		if err := os.WriteFile(tmpFile, []byte("file-content"), 0644); err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}

		specs, err := ParseEnvVarSpecs([]string{"VAR=@" + tmpFile}, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(specs) != 1 {
			t.Fatalf("expected 1 spec, got %d", len(specs))
		}
		if specs[0].Key != "VAR" || specs[0].Value != "file-content" {
			t.Errorf("unexpected spec: %+v", specs[0])
		}
		if !specs[0].FromFile {
			t.Error("expected FromFile to be true")
		}
	})

	t.Run("trims trailing newlines from file value", func(t *testing.T) {
		tmpDir := t.TempDir()
		tmpFile := filepath.Join(tmpDir, "secret.txt")
		if err := os.WriteFile(tmpFile, []byte("file-content\r\n\n"), 0644); err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}

		specs, err := ParseEnvVarSpecs([]string{"VAR=@" + tmpFile}, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(specs) != 1 {
			t.Fatalf("expected 1 spec, got %d", len(specs))
		}
		if specs[0].Value != "file-content" {
			t.Errorf("expected trimmed value %q, got %q", "file-content", specs[0].Value)
		}
	})

	t.Run("returns error for nonexistent file", func(t *testing.T) {
		_, err := ParseEnvVarSpecs([]string{"VAR=@/nonexistent/file.txt"}, nil)
		if err == nil {
			t.Fatal("expected error for nonexistent file")
		}
	})

	t.Run("returns error for empty key", func(t *testing.T) {
		_, err := ParseEnvVarSpecs([]string{"=value"}, nil)
		if err == nil {
			t.Fatal("expected error for empty key")
		}
	})

	t.Run("handles value with equals sign", func(t *testing.T) {
		specs, err := ParseEnvVarSpecs([]string{"URL=http://example.com?a=1&b=2"}, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(specs) != 1 {
			t.Fatalf("expected 1 spec, got %d", len(specs))
		}
		if specs[0].Key != "URL" || specs[0].Value != "http://example.com?a=1&b=2" {
			t.Errorf("unexpected spec: %+v", specs[0])
		}
	})

	t.Run("handles empty value", func(t *testing.T) {
		specs, err := ParseEnvVarSpecs([]string{"EMPTY="}, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(specs) != 1 {
			t.Fatalf("expected 1 spec, got %d", len(specs))
		}
		if specs[0].Key != "EMPTY" || specs[0].Value != "" {
			t.Errorf("unexpected spec: %+v", specs[0])
		}
	})
}

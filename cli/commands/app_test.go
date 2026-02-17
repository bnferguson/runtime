package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeAppToml(t *testing.T, dir, content string) {
	t.Helper()
	mirenDir := filepath.Join(dir, ".miren")
	if err := os.MkdirAll(mirenDir, 0755); err != nil {
		t.Fatalf("failed to create .miren dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(mirenDir, "app.toml"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write app.toml: %v", err)
	}
}

func TestAppCentricValidate(t *testing.T) {
	t.Run("invalid TOML syntax returns parse error", func(t *testing.T) {
		dir := t.TempDir()
		writeAppToml(t, dir, `[[[`)

		a := AppCentric{Dir: dir}
		err := a.Validate(&GlobalFlags{})
		if err == nil {
			t.Fatal("expected error for invalid TOML syntax")
		}
		if strings.Contains(err.Error(), "app is required") {
			t.Errorf("expected parse error, got generic 'app is required': %v", err)
		}
		if !strings.Contains(err.Error(), "error loading") {
			t.Errorf("expected error to mention 'error loading', got: %v", err)
		}
	})

	t.Run("type mismatch returns decode error", func(t *testing.T) {
		dir := t.TempDir()
		// command is a string field but we give it an array
		writeAppToml(t, dir, `
name = "myapp"

[services.web]
command = ["foo", "bar"]
`)

		a := AppCentric{Dir: dir}
		err := a.Validate(&GlobalFlags{})
		if err == nil {
			t.Fatal("expected error for type mismatch")
		}
		if strings.Contains(err.Error(), "app is required") {
			t.Errorf("expected decode error, got generic 'app is required': %v", err)
		}
		if !strings.Contains(err.Error(), "error loading") {
			t.Errorf("expected error to mention 'error loading', got: %v", err)
		}
	})

	t.Run("valid TOML with name populates App", func(t *testing.T) {
		dir := t.TempDir()
		writeAppToml(t, dir, `name = "myapp"`)

		a := AppCentric{Dir: dir}
		err := a.Validate(&GlobalFlags{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if a.App != "myapp" {
			t.Errorf("expected App to be 'myapp', got %q", a.App)
		}
	})

	t.Run("no app.toml returns app is required", func(t *testing.T) {
		dir := t.TempDir()

		a := AppCentric{Dir: dir}
		err := a.Validate(&GlobalFlags{})
		if err == nil {
			t.Fatal("expected error when no app.toml exists")
		}
		if err.Error() != "app is required" {
			t.Errorf("expected 'app is required', got: %v", err)
		}
	})

	t.Run("app flag with invalid TOML still returns parse error", func(t *testing.T) {
		dir := t.TempDir()
		writeAppToml(t, dir, `[[[`)

		a := AppCentric{Dir: dir, App: "myapp"}
		err := a.Validate(&GlobalFlags{})
		if err == nil {
			t.Fatal("expected error for invalid TOML even with -a flag")
		}
		if strings.Contains(err.Error(), "app is required") {
			t.Errorf("expected parse error, got: %v", err)
		}
	})

	t.Run("app flag with no app.toml succeeds", func(t *testing.T) {
		dir := t.TempDir()

		a := AppCentric{Dir: dir, App: "myapp"}
		err := a.Validate(&GlobalFlags{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if a.App != "myapp" {
			t.Errorf("expected App to be 'myapp', got %q", a.App)
		}
	})
}

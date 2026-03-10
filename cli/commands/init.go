package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
	"miren.dev/runtime/appconfig"
)

// inferAppName derives a sanitized app name from a directory path.
func inferAppName(dir string) string {
	name := filepath.Base(dir)
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")
	return name
}

// initApp creates a .miren/app.toml in dir with the given app name.
// It returns the path to the created file.
func initApp(dir, name string) (string, error) {
	appTomlPath := filepath.Join(dir, appconfig.AppConfigPath)
	runtimeDir := filepath.Dir(appTomlPath)

	if _, err := os.Stat(appTomlPath); err != nil {
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("failed to check for existing app.toml: %w", err)
		}
	} else {
		return "", fmt.Errorf("app.toml already exists in %s - app already initialized", runtimeDir)
	}

	if err := os.MkdirAll(runtimeDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create .miren directory: %w", err)
	}

	appConfig := &appconfig.AppConfig{
		Name: name,
	}

	content, err := toml.Marshal(appConfig)
	if err != nil {
		return "", fmt.Errorf("failed to marshal app config: %w", err)
	}

	if err := os.WriteFile(appTomlPath, content, 0644); err != nil {
		return "", fmt.Errorf("failed to write app.toml: %w", err)
	}

	return appTomlPath, nil
}

func Init(ctx *Context, opts struct {
	Name string `short:"n" long:"name" description:"Application name (defaults to directory name)"`
	Dir  string `short:"d" long:"dir" description:"Application directory (defaults to current directory)"`
	ConfigCentric
}) error {
	workDir := opts.Dir
	if workDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		workDir = wd
	} else {
		absDir, err := filepath.Abs(workDir)
		if err != nil {
			return fmt.Errorf("failed to resolve directory path: %w", err)
		}
		workDir = absDir
	}

	appName := opts.Name
	if appName == "" {
		appName = inferAppName(workDir)
	}

	appTomlPath, err := initApp(workDir, appName)
	if err != nil {
		return err
	}

	ctx.Printf("Initialized Miren app '%s' in %s\n", appName, appTomlPath)
	return nil
}

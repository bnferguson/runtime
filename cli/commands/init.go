package commands

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	toml "github.com/pelletier/go-toml/v2"
	"miren.dev/runtime/api/app/app_v1alpha"
	"miren.dev/runtime/appconfig"
	"miren.dev/runtime/pkg/stackbuild"
)

// generateSecretKey generates a cryptographically secure random hex string
// suitable for use as a secret key (e.g., Rails SECRET_KEY_BASE)
func generateSecretKey() (string, error) {
	bytes := make([]byte, 64) // 64 bytes = 128 hex characters
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func Init(ctx *Context, opts struct {
	Name   string `short:"n" long:"name" description:"Application name (defaults to directory name)"`
	Dir    string `short:"d" long:"dir" description:"Application directory (defaults to current directory)"`
	Update bool   `short:"u" long:"update" description:"Update existing app.toml with newly detected env vars"`
	ConfigCentric
}) error {
	// Determine working directory
	workDir := opts.Dir
	if workDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		workDir = wd
	} else {
		// Convert to absolute path
		absDir, err := filepath.Abs(workDir)
		if err != nil {
			return fmt.Errorf("failed to resolve directory path: %w", err)
		}
		workDir = absDir
	}

	appTomlPath := filepath.Join(workDir, appconfig.AppConfigPath)
	runtimeDir := filepath.Dir(appTomlPath)

	var appConfig *appconfig.AppConfig
	var existingEnvVars map[string]string // tracks existing env var values
	isUpdate := false

	// Check if already initialized
	if _, err := os.Stat(appTomlPath); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to check for existing app.toml: %w", err)
		}
		// File doesn't exist
		if opts.Update {
			return fmt.Errorf("app.toml does not exist - use 'miren init' without --update first")
		}
	} else {
		// File exists
		if !opts.Update {
			return fmt.Errorf("app.toml already exists in %s - use --update to add detected env vars", runtimeDir)
		}
		isUpdate = true
	}

	if isUpdate {
		// Load existing config
		content, err := os.ReadFile(appTomlPath)
		if err != nil {
			return fmt.Errorf("failed to read app.toml: %w", err)
		}
		appConfig, err = appconfig.ParseWithoutValidation(content)
		if err != nil {
			return fmt.Errorf("failed to parse app.toml: %w", err)
		}

		// Build map of existing env vars
		existingEnvVars = make(map[string]string)
		for _, ev := range appConfig.EnvVars {
			existingEnvVars[ev.Key] = ev.Value
		}

		ctx.Printf("Updating %s\n", appTomlPath)
	} else {
		// Determine app name for new config
		appName := opts.Name
		if appName == "" {
			appName = filepath.Base(workDir)
			appName = strings.ToLower(appName)
			appName = strings.ReplaceAll(appName, " ", "-")
			appName = strings.ReplaceAll(appName, "_", "-")
		}

		// Create .miren directory
		if err := os.MkdirAll(runtimeDir, 0755); err != nil {
			return fmt.Errorf("failed to create .miren directory: %w", err)
		}

		appConfig = &appconfig.AppConfig{
			Name: appName,
		}
		existingEnvVars = make(map[string]string)
	}

	// Analyze the codebase to detect stack and required env vars
	ctx.Printf("Analyzing codebase...\n")

	var detectedStack stackbuild.Stack
	stack, err := stackbuild.DetectStack(workDir, stackbuild.BuildOptions{})
	if err != nil {
		ctx.Printf("  No supported stack detected\n")
	} else {
		detectedStack = stack
		ctx.Printf("  Detected stack: %s\n", lipgloss.NewStyle().Bold(true).Render(stack.Name()))

		// Show detection events
		for _, event := range stack.Events() {
			if event.Kind == "framework" || event.Kind == "package" {
				ctx.Printf("  Found %s: %s\n", event.Kind, event.Name)
			}
		}
	}

	// secretToStore represents a secret that should be sent to the server
	type secretToStore struct {
		Key       string
		Value     string
		Sensitive bool
		Source    string // "generated" or "read from <file>"
	}

	// defaultEnvVar represents a non-secret env var with a default value (goes in app.toml)
	type defaultEnvVar struct {
		Key    string
		Value  string
		Source string // e.g., "default"
	}

	var secretsToStore []secretToStore
	var defaultsForAppToml []defaultEnvVar
	var needsManualConfig []stackbuild.EnvVarRequirement
	var alreadyConfigured []stackbuild.EnvVarRequirement

	// Get required environment variables from stack detection
	if detectedStack != nil {
		envVars := detectedStack.RequiredEnvVars()
		var requiredVars []stackbuild.EnvVarRequirement
		var recommendedVars []stackbuild.EnvVarRequirement

		// Separate required from recommended vars
		for _, ev := range envVars {
			switch ev.Confidence {
			case "required":
				requiredVars = append(requiredVars, ev)
			case "recommended":
				recommendedVars = append(recommendedVars, ev)
			}
		}

		if len(requiredVars) > 0 {
			ctx.Printf("\n")
			ctx.Printf("%s\n", lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("1")).Render("Required Environment Variables"))

			for _, ev := range requiredVars {
				// Check if already configured locally (in app.toml)
				if existingVal, exists := existingEnvVars[ev.Name]; exists && existingVal != "" {
					alreadyConfigured = append(alreadyConfigured, ev)
					continue
				}

				if ev.DefaultValue != "" {
					// Non-secret with a default value - write to app.toml
					defaultsForAppToml = append(defaultsForAppToml, defaultEnvVar{
						Key:    ev.Name,
						Value:  ev.DefaultValue,
						Source: "default",
					})
				} else if ev.CanGenerate {
					// Generate a value automatically - will be stored server-side
					value, err := generateSecretKey()
					if err != nil {
						needsManualConfig = append(needsManualConfig, ev)
						continue
					}
					secretsToStore = append(secretsToStore, secretToStore{
						Key:       ev.Name,
						Value:     value,
						Sensitive: true,
						Source:    "generated",
					})
				} else if ev.ReadFromFile != "" {
					// Try to read value from file - will be stored server-side
					filePath := filepath.Join(workDir, ev.ReadFromFile)
					content, err := os.ReadFile(filePath)
					if err != nil {
						needsManualConfig = append(needsManualConfig, ev)
						continue
					}
					value := strings.TrimSpace(string(content))
					if value == "" {
						needsManualConfig = append(needsManualConfig, ev)
						continue
					}
					secretsToStore = append(secretsToStore, secretToStore{
						Key:       ev.Name,
						Value:     value,
						Sensitive: true,
						Source:    "read from " + ev.ReadFromFile,
					})
				} else {
					needsManualConfig = append(needsManualConfig, ev)
				}
			}
		}

		if len(recommendedVars) > 0 {
			ctx.Printf("\n")
			ctx.Printf("%s\n", lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3")).Render("Recommended Environment Variables"))
			ctx.Printf("You may also want to configure:\n\n")

			for _, ev := range recommendedVars {
				ctx.Printf("  %s\n", lipgloss.NewStyle().Bold(true).Render(ev.Name))
				ctx.Printf("    %s\n", lipgloss.NewStyle().Faint(true).Render(ev.Reason))
			}
		}
	}

	// Add default env vars to app.toml
	for _, defVar := range defaultsForAppToml {
		appConfig.EnvVars = append(appConfig.EnvVars, appconfig.AppEnvVar{
			Key:   defVar.Key,
			Value: defVar.Value,
		})
	}

	// Send secrets to server if we have any
	var serverConfigured []secretToStore
	if len(secretsToStore) > 0 {
		cl, err := ctx.RPCClient("dev.miren.runtime/app")
		if err != nil {
			ctx.Printf("\n%s Could not connect to server: %v\n",
				lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render("Warning:"), err)
			ctx.Printf("Secrets not stored. Run 'miren config set' to configure manually after logging in.\n")
		} else {
			ac := app_v1alpha.NewCrudClient(cl)

			// Create app if it doesn't exist
			_, err = ac.New(ctx, appConfig.Name)
			if err != nil {
				ctx.Printf("\n%s Could not create app on server: %v\n",
					lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render("Warning:"), err)
				ctx.Printf("Secrets not stored. Run 'miren config set' to configure manually.\n")
			} else {
				// Set each secret on the server
				for _, secret := range secretsToStore {
					_, err := ac.SetEnvVar(ctx, appConfig.Name, secret.Key, secret.Value, secret.Sensitive, "")
					if err != nil {
						ctx.Printf("  %s Failed to set %s: %v\n",
							lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render("Warning:"), secret.Key, err)
					} else {
						serverConfigured = append(serverConfigured, secret)
					}
				}
			}
		}
	}

	// Display what was configured
	if len(alreadyConfigured) > 0 || len(serverConfigured) > 0 || len(defaultsForAppToml) > 0 {
		ctx.Printf("\nAutomatically configured:\n")
		for _, ev := range alreadyConfigured {
			ctx.Printf("  %s %s\n",
				lipgloss.NewStyle().Bold(true).Render(ev.Name),
				lipgloss.NewStyle().Faint(true).Foreground(lipgloss.Color("2")).Render("✓ already configured"))
		}
		for _, defVar := range defaultsForAppToml {
			ctx.Printf("  %s=%s %s\n",
				lipgloss.NewStyle().Bold(true).Render(defVar.Key),
				defVar.Value,
				lipgloss.NewStyle().Faint(true).Foreground(lipgloss.Color("2")).Render("✓ default (in app.toml)"))
		}
		for _, secret := range serverConfigured {
			ctx.Printf("  %s %s\n",
				lipgloss.NewStyle().Bold(true).Render(secret.Key),
				lipgloss.NewStyle().Faint(true).Foreground(lipgloss.Color("2")).Render("✓ "+secret.Source+" (stored on server)"))
		}
	}

	if len(needsManualConfig) > 0 {
		ctx.Printf("\nMust be configured manually:\n")
		for _, ev := range needsManualConfig {
			ctx.Printf("  %s\n", lipgloss.NewStyle().Bold(true).Render(ev.Name))
			ctx.Printf("    %s\n", lipgloss.NewStyle().Faint(true).Render(ev.Reason))
		}
		ctx.Printf("\nSet their values with:\n")
		ctx.Printf("  miren config set <KEY>=<value>\n")
	}

	// Marshal to TOML
	content, err := toml.Marshal(appConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal app config: %w", err)
	}

	// Write app.toml
	if err := os.WriteFile(appTomlPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write app.toml: %w", err)
	}

	if isUpdate {
		ctx.Printf("\nUpdated %s\n", appTomlPath)
	} else {
		ctx.Printf("\nInitialized Miren app '%s' in %s\n", appConfig.Name, appTomlPath)
	}
	return nil
}

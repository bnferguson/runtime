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

// inferAppName derives a sanitized app name from a directory path.
func inferAppName(dir string) string {
	name := filepath.Base(dir)
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")
	return name
}

// initApp creates a fresh .miren/app.toml in dir with the given app name.
// Returns an error if app.toml already exists. Used by callers that need a
// minimal init without env-var detection or server-side secret setup.
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

	appConfig := &appconfig.AppConfig{Name: name}

	content, err := toml.Marshal(appConfig)
	if err != nil {
		return "", fmt.Errorf("failed to marshal app config: %w", err)
	}

	if err := os.WriteFile(appTomlPath, content, 0644); err != nil {
		return "", fmt.Errorf("failed to write app.toml: %w", err)
	}

	return appTomlPath, nil
}

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

	appTomlPath := filepath.Join(workDir, appconfig.AppConfigPath)
	runtimeDir := filepath.Dir(appTomlPath)

	var appConfig *appconfig.AppConfig
	// Presence map: a key declared in app.toml counts as configured even
	// when its value is empty, since server-side secrets legitimately leave
	// the value blank. Tracking by value would re-process already-declared
	// secret keys and re-append their defaults on each --update.
	existingEnvVars := make(map[string]struct{})
	isUpdate := false

	if _, err := os.Stat(appTomlPath); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to check for existing app.toml: %w", err)
		}
		if opts.Update {
			return fmt.Errorf("app.toml does not exist - use 'miren init' without --update first")
		}
	} else {
		if !opts.Update {
			return fmt.Errorf("app.toml already exists in %s - use --update to add detected env vars", runtimeDir)
		}
		isUpdate = true
	}

	if isUpdate {
		content, err := os.ReadFile(appTomlPath)
		if err != nil {
			return fmt.Errorf("failed to read app.toml: %w", err)
		}
		appConfig, err = appconfig.ParseWithoutValidation(content)
		if err != nil {
			return fmt.Errorf("failed to parse app.toml: %w", err)
		}

		for _, ev := range appConfig.EnvVars {
			existingEnvVars[ev.Key] = struct{}{}
		}

		ctx.Printf("Updating %s\n", appTomlPath)
	} else {
		appName := opts.Name
		if appName == "" {
			appName = inferAppName(workDir)
		}

		if err := os.MkdirAll(runtimeDir, 0755); err != nil {
			return fmt.Errorf("failed to create .miren directory: %w", err)
		}

		appConfig = &appconfig.AppConfig{
			Name: appName,
		}
	}

	ctx.Printf("Analyzing codebase...\n")

	var detectedStack stackbuild.Stack
	stack, err := stackbuild.DetectStack(workDir, stackbuild.BuildOptions{})
	if err != nil {
		ctx.Printf("  No supported stack detected\n")
	} else {
		detectedStack = stack
		ctx.Printf("  Detected stack: %s\n", lipgloss.NewStyle().Bold(true).Render(stack.Name()))

		for _, event := range stack.Events() {
			if event.Kind == "framework" || event.Kind == "package" {
				ctx.Printf("  Found %s: %s\n", event.Kind, event.Name)
			}
		}
	}

	type secretToStore struct {
		Key       string
		Value     string
		Sensitive bool
		Source    string
	}

	type defaultEnvVar struct {
		Key    string
		Value  string
		Source string
	}

	var secretsToStore []secretToStore
	var defaultsForAppToml []defaultEnvVar
	var needsManualConfig []stackbuild.EnvVarRequirement
	var alreadyConfigured []stackbuild.EnvVarRequirement

	if detectedStack != nil {
		envVars := detectedStack.RequiredEnvVars()
		var requiredVars []stackbuild.EnvVarRequirement
		var recommendedVars []stackbuild.EnvVarRequirement

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
				if _, exists := existingEnvVars[ev.Name]; exists {
					alreadyConfigured = append(alreadyConfigured, ev)
					continue
				}

				if ev.DefaultValue != "" {
					defaultsForAppToml = append(defaultsForAppToml, defaultEnvVar{
						Key:    ev.Name,
						Value:  ev.DefaultValue,
						Source: "default",
					})
				} else if ev.CanGenerate {
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

	for _, defVar := range defaultsForAppToml {
		appConfig.EnvVars = append(appConfig.EnvVars, appconfig.AppEnvVar{
			Key:   defVar.Key,
			Value: defVar.Value,
		})
	}

	var serverConfigured []secretToStore
	if len(secretsToStore) > 0 {
		cl, err := ctx.RPCClient("dev.miren.runtime/app")
		if err != nil {
			ctx.Printf("\n%s Could not connect to server: %v\n",
				lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render("Warning:"), err)
			ctx.Printf("Secrets not stored. Run 'miren config set' to configure manually after logging in.\n")
		} else {
			ac := app_v1alpha.NewCrudClient(cl)

			_, err = ac.New(ctx, appConfig.Name)
			if err != nil {
				ctx.Printf("\n%s Could not create app on server: %v\n",
					lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render("Warning:"), err)
				ctx.Printf("Secrets not stored. Run 'miren config set' to configure manually.\n")
			} else {
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

	content, err := toml.Marshal(appConfig)
	if err != nil {
		return err
	}

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

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
	"miren.dev/runtime/pkg/theme"
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

	appTomlPath := filepath.Join(workDir, appconfig.AppConfigPath)
	runtimeDir := filepath.Dir(appTomlPath)

	if _, err := os.Stat(appTomlPath); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to check for existing app.toml: %w", err)
		}
	} else {
		return fmt.Errorf("app.toml already exists in %s - app already initialized", runtimeDir)
	}

	appName := opts.Name
	if appName == "" {
		appName = inferAppName(workDir)
	}

	if err := os.MkdirAll(runtimeDir, 0755); err != nil {
		return fmt.Errorf("failed to create .miren directory: %w", err)
	}

	appConfig := &appconfig.AppConfig{
		Name: appName,
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
			ctx.Printf("%s\n", lipgloss.NewStyle().Bold(true).Foreground(theme.Error).Render("Required Environment Variables"))

			for _, ev := range requiredVars {
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
			ctx.Printf("%s\n", lipgloss.NewStyle().Bold(true).Foreground(theme.Warning).Render("Recommended Environment Variables"))
			ctx.Printf("You may also want to configure:\n\n")

			for _, ev := range recommendedVars {
				ctx.Printf("  %s\n", lipgloss.NewStyle().Bold(true).Render(ev.Name))
				ctx.Printf("    %s\n", lipgloss.NewStyle().Foreground(theme.Muted).Render(ev.Reason))
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
				lipgloss.NewStyle().Foreground(theme.Warning).Render("Warning:"), err)
			ctx.Printf("Secrets not stored. Run 'miren config set' to configure manually after logging in.\n")
		} else {
			ac := app_v1alpha.NewCrudClient(cl)

			_, err = ac.New(ctx, appConfig.Name)
			if err != nil {
				ctx.Printf("\n%s Could not create app on server: %v\n",
					lipgloss.NewStyle().Foreground(theme.Warning).Render("Warning:"), err)
				ctx.Printf("Secrets not stored. Run 'miren config set' to configure manually.\n")
			} else {
				// Stage the secrets on the app's initial ConfigVersion via a
				// single batched RPC. This deliberately avoids creating an
				// AppVersion before the first build — the initial config is
				// picked up by the first deploy.
				rpcVars := make([]*app_v1alpha.NamedValue, 0, len(secretsToStore))
				for _, secret := range secretsToStore {
					nv := &app_v1alpha.NamedValue{}
					nv.SetKey(secret.Key)
					nv.SetValue(secret.Value)
					nv.SetSensitive(secret.Sensitive)
					rpcVars = append(rpcVars, nv)
				}

				_, err := ac.SetInitialEnvVars(ctx, appConfig.Name, rpcVars, "")
				if err != nil {
					ctx.Printf("\n%s Failed to stage secrets on server: %v\n",
						lipgloss.NewStyle().Foreground(theme.Warning).Render("Warning:"), err)
					ctx.Printf("Secrets not stored. Run 'miren config set' to configure manually.\n")
				} else {
					serverConfigured = append(serverConfigured, secretsToStore...)
				}
			}
		}
	}

	if len(serverConfigured) > 0 || len(defaultsForAppToml) > 0 {
		ctx.Printf("\nAutomatically configured:\n")
		for _, defVar := range defaultsForAppToml {
			ctx.Printf("  %s=%s %s\n",
				lipgloss.NewStyle().Bold(true).Render(defVar.Key),
				defVar.Value,
				lipgloss.NewStyle().Foreground(theme.Success).Render("✓ default (in app.toml)"))
		}
		for _, secret := range serverConfigured {
			ctx.Printf("  %s %s\n",
				lipgloss.NewStyle().Bold(true).Render(secret.Key),
				lipgloss.NewStyle().Foreground(theme.Success).Render("✓ "+secret.Source+" (stored on server)"))
		}
	}

	if len(needsManualConfig) > 0 {
		ctx.Printf("\nMust be configured manually:\n")
		for _, ev := range needsManualConfig {
			ctx.Printf("  %s\n", lipgloss.NewStyle().Bold(true).Render(ev.Name))
			ctx.Printf("    %s\n", lipgloss.NewStyle().Foreground(theme.Muted).Render(ev.Reason))
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

	ctx.Printf("\nInitialized Miren app '%s' in %s\n", appConfig.Name, appTomlPath)
	return nil
}

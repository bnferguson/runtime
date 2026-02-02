package stackbuild

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// EnvVarRequirement represents a detected environment variable requirement
type EnvVarRequirement struct {
	Name         string // e.g., "DATABASE_URL"
	Source       string // "gem", "code", "config", "rails_core"
	Confidence   string // "required", "recommended", "optional"
	Reason       string // e.g., "pg gem detected in Gemfile"
	CanGenerate  bool   // true if Miren can auto-generate a value (e.g., SECRET_KEY_BASE)
	ReadFromFile string // if set, read value from this file path (relative to app dir)
	DefaultValue string // if set, this is the default value to use (non-secret)
}

// ignoredEnvVars are environment variables that are commonly used but don't need explicit configuration
// or are detected via dedicated logic (SECRET_KEY_BASE, RAILS_MASTER_KEY are detected via rails_core)
var ignoredEnvVars = map[string]bool{
	"PATH":                  true,
	"HOME":                  true,
	"USER":                  true,
	"SHELL":                 true,
	"LANG":                  true,
	"TZ":                    true,
	"PWD":                   true,
	"TERM":                  true,
	"RACK_ENV":              true, // Handled via RAILS_ENV in Rails apps
	"NODE_ENV":              true,
	"BUNDLE_PATH":           true,
	"BUNDLE_WITHOUT":        true,
	"GEM_HOME":              true,
	"GEM_PATH":              true,
	"SECRET_KEY_BASE_DUMMY": true,
	"SECRET_KEY_BASE":       true, // Detected via rails_core
	"RAILS_MASTER_KEY":      true, // Detected via rails_core
	"RAILS_ENV":             true, // Detected via rails_core with default
	"PORT":                  true,
}

// parseEnvSampleFile parses a .env.sample or .env.example file and returns variable names
func parseEnvSampleFile(dir string, filename string) []string {
	path := filepath.Join(dir, filename)
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	var found []string
	seen := make(map[string]bool)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=value or KEY= format
		idx := strings.Index(line, "=")
		if idx > 0 {
			key := strings.TrimSpace(line[:idx])
			// Validate it looks like an env var name
			if isValidEnvVarName(key) && !seen[key] && !ignoredEnvVars[key] {
				seen[key] = true
				found = append(found, key)
			}
		}
	}

	return found
}

// isValidEnvVarName checks if a string is a valid environment variable name
// Valid names start with uppercase letter or underscore, followed by uppercase letters, digits, or underscores
func isValidEnvVarName(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i, c := range s {
		isUppercase := c >= 'A' && c <= 'Z'
		isDigit := c >= '0' && c <= '9'
		isUnderscore := c == '_'

		if i == 0 {
			if !isUppercase && !isUnderscore {
				return false
			}
		} else {
			if !isUppercase && !isDigit && !isUnderscore {
				return false
			}
		}
	}
	return true
}

// hasEnvVar checks if a slice of EnvVarRequirement contains a variable with the given name
func hasEnvVar(vars []EnvVarRequirement, name string) bool {
	for _, v := range vars {
		if v.Name == name {
			return true
		}
	}
	return false
}

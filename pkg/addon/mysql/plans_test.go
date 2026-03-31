package mysql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefinitionHasAllVariants(t *testing.T) {
	def := Definition()

	assert.Equal(t, AddonName, def.Name)
	assert.Equal(t, "Miren MySQL", def.DisplayName)
	assert.Equal(t, "small", def.DefaultVariant)
	assert.Len(t, def.Variants, 2)

	variantNames := make(map[string]bool)
	for _, v := range def.Variants {
		variantNames[v.Name] = true
	}

	assert.True(t, variantNames["small"])
	assert.True(t, variantNames["shared"])
}

func TestIsSharedVariant(t *testing.T) {
	assert.True(t, IsSharedVariant("shared"))
	assert.False(t, IsSharedVariant("small"))
}

func TestSanitizeIdentifier(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"my-app", "my_app"},
		{"MyApp", "myapp"},
		{"123app", "a123app"},
		{"app_name", "app_name"},
		{"app.name", "appname"},
		{"APP-NAME", "app_name"},
		{"", "app"},
		{"a", "a"},
		{"my-really-long-application-name-for-production", "my_really_long_application_name_"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, sanitizeIdentifier(tt.input))
		})
	}
}

func TestBuildEnvVars(t *testing.T) {
	vars := buildEnvVars("myhost", 3306, "myuser", "mypass", "mydb")

	assert.Len(t, vars, 6)

	varMap := make(map[string]string)
	sensitiveMap := make(map[string]bool)
	for _, v := range vars {
		varMap[v.Key] = v.Value
		sensitiveMap[v.Key] = v.Sensitive
	}

	assert.Equal(t, "mysql://myuser:mypass@myhost:3306/mydb", varMap["DATABASE_URL"])
	assert.True(t, sensitiveMap["DATABASE_URL"])

	assert.Equal(t, "myhost", varMap["MYSQL_HOST"])
	assert.False(t, sensitiveMap["MYSQL_HOST"])

	assert.Equal(t, "3306", varMap["MYSQL_PORT"])
	assert.False(t, sensitiveMap["MYSQL_PORT"])

	assert.Equal(t, "myuser", varMap["MYSQL_USER"])
	assert.False(t, sensitiveMap["MYSQL_USER"])

	assert.Equal(t, "mypass", varMap["MYSQL_PASSWORD"])
	assert.True(t, sensitiveMap["MYSQL_PASSWORD"])

	assert.Equal(t, "mydb", varMap["MYSQL_DATABASE"])
	assert.False(t, sensitiveMap["MYSQL_DATABASE"])
}

func TestBuildDatabaseURL(t *testing.T) {
	url := buildDatabaseURL("host.example.com", 3306, "user", "pass", "dbname")
	assert.Equal(t, "mysql://user:pass@host.example.com:3306/dbname", url)
}

func TestVariantConfigContainsExpectedKeys(t *testing.T) {
	def := Definition()

	for _, variant := range def.Variants {
		t.Run(variant.Name, func(t *testing.T) {
			if variant.Name == "shared" {
				assert.Equal(t, "true", variant.Config[ConfigShared])
			} else {
				assert.NotEmpty(t, variant.Config[ConfigStorage])
				assert.Equal(t, "false", variant.Config[ConfigShared])
			}
		})
	}
}

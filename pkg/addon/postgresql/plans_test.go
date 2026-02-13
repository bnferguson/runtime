package postgresql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefinitionHasAllVariants(t *testing.T) {
	def := Definition()

	assert.Equal(t, AddonName, def.Name)
	assert.Equal(t, "Miren PostgreSQL", def.DisplayName)
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
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, sanitizeIdentifier(tt.input))
		})
	}
}

func TestBuildEnvVars(t *testing.T) {
	vars := buildEnvVars("myhost", 5432, "myuser", "mypass", "mydb")

	assert.Len(t, vars, 6)

	varMap := make(map[string]string)
	sensitiveMap := make(map[string]bool)
	for _, v := range vars {
		varMap[v.Key] = v.Value
		sensitiveMap[v.Key] = v.Sensitive
	}

	assert.Equal(t, "postgres://myuser:mypass@myhost:5432/mydb", varMap["DATABASE_URL"])
	assert.True(t, sensitiveMap["DATABASE_URL"])

	assert.Equal(t, "myhost", varMap["PGHOST"])
	assert.False(t, sensitiveMap["PGHOST"])

	assert.Equal(t, "5432", varMap["PGPORT"])
	assert.False(t, sensitiveMap["PGPORT"])

	assert.Equal(t, "myuser", varMap["PGUSER"])
	assert.False(t, sensitiveMap["PGUSER"])

	assert.Equal(t, "mypass", varMap["PGPASSWORD"])
	assert.True(t, sensitiveMap["PGPASSWORD"])

	assert.Equal(t, "mydb", varMap["PGDATABASE"])
	assert.False(t, sensitiveMap["PGDATABASE"])
}

func TestBuildDatabaseURL(t *testing.T) {
	url := buildDatabaseURL("host.example.com", 5432, "user", "pass", "dbname")
	assert.Equal(t, "postgres://user:pass@host.example.com:5432/dbname", url)
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

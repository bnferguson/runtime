package memcache

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefinitionHasAllVariants(t *testing.T) {
	def := Definition()

	assert.Equal(t, AddonName, def.Name)
	assert.Equal(t, "Miren Memcache", def.DisplayName)
	assert.Equal(t, "small", def.DefaultVariant)
	assert.Len(t, def.Variants, 1)
	assert.Equal(t, "small", def.Variants[0].Name)
}

func TestBuildEnvVars(t *testing.T) {
	vars := buildEnvVars("myhost", 11211)

	assert.Len(t, vars, 3)

	varMap := make(map[string]string)
	sensitiveMap := make(map[string]bool)
	for _, v := range vars {
		varMap[v.Key] = v.Value
		sensitiveMap[v.Key] = v.Sensitive
	}

	assert.Equal(t, "memcache://myhost:11211", varMap["MEMCACHE_URL"])
	assert.False(t, sensitiveMap["MEMCACHE_URL"])
	assert.Equal(t, "myhost", varMap["MEMCACHE_HOST"])
	assert.False(t, sensitiveMap["MEMCACHE_HOST"])
	assert.Equal(t, "11211", varMap["MEMCACHE_PORT"])
	assert.False(t, sensitiveMap["MEMCACHE_PORT"])
}

func TestBuildMemcacheURL(t *testing.T) {
	url := buildMemcacheURL("host.example.com", 11211)
	assert.Equal(t, "memcache://host.example.com:11211", url)
}

func TestVariantConfigContainsExpectedKeys(t *testing.T) {
	def := Definition()

	for _, variant := range def.Variants {
		t.Run(variant.Name, func(t *testing.T) {
			assert.NotEmpty(t, variant.Config[ConfigMemory])
		})
	}
}

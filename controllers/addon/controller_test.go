package addon

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"miren.dev/runtime/api/core/core_v1alpha"
	"miren.dev/runtime/pkg/addon"
	"miren.dev/runtime/pkg/entity"
)

func TestAddonNameFromRef(t *testing.T) {
	tests := []struct {
		ref  string
		want string
	}{
		{"addon/miren-postgresql", "miren-postgresql"},
		{"miren-postgresql", "miren-postgresql"},
		{"addon/ns/miren-postgresql", "miren-postgresql"},
	}

	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			got := addonNameFromRef(entity.Id("dev.miren.addon/" + tt.ref))
			// addonNameFromRef takes the last segment after "/"
			assert.Equal(t, tt.want, got)
		})
	}

	// Direct test without prefix
	assert.Equal(t, "miren-postgresql", addonNameFromRef(entity.Id("addon/miren-postgresql")))
}

func TestFindCollisions(t *testing.T) {
	existing := []core_v1alpha.Variable{
		{Key: "DATABASE_URL", Value: "old", Source: "manual"},
		{Key: "PGHOST", Value: "old", Source: "config"},
		{Key: "OTHER_VAR", Value: "val", Source: "manual"},
	}

	addonVars := []addon.Variable{
		{Key: "DATABASE_URL", Value: "new"},
		{Key: "PGHOST", Value: "new"},
		{Key: "PGPORT", Value: "5432"},
	}

	collisions := findCollisions(existing, addonVars)
	assert.ElementsMatch(t, []string{"DATABASE_URL", "PGHOST"}, collisions)
}

func TestFindCollisionsNoOverlap(t *testing.T) {
	existing := []core_v1alpha.Variable{
		{Key: "OTHER_VAR", Value: "val"},
	}

	addonVars := []addon.Variable{
		{Key: "DATABASE_URL", Value: "new"},
	}

	collisions := findCollisions(existing, addonVars)
	assert.Empty(t, collisions)
}

func TestMergeAddonVars(t *testing.T) {
	existing := []core_v1alpha.Variable{
		{Key: "MANUAL_VAR", Value: "manual_val", Source: "manual"},
		{Key: "CONFIG_VAR", Value: "config_val", Source: "config"},
	}

	addonVars := []addon.Variable{
		{Key: "DATABASE_URL", Value: "postgres://...", Sensitive: true},
		{Key: "PGHOST", Value: "host.addon.app.miren"},
		{Key: "MANUAL_VAR", Value: "should_not_override"},
		{Key: "CONFIG_VAR", Value: "should_override"},
	}

	result := mergeAddonVars(existing, addonVars)

	varMap := make(map[string]core_v1alpha.Variable, len(result))
	for _, v := range result {
		varMap[v.Key] = v
	}

	// Manual var should NOT be overridden
	assert.Equal(t, "manual_val", varMap["MANUAL_VAR"].Value)
	assert.Equal(t, "manual", varMap["MANUAL_VAR"].Source)

	// Config var should be overridden by addon
	assert.Equal(t, "should_override", varMap["CONFIG_VAR"].Value)
	assert.Equal(t, "addon", varMap["CONFIG_VAR"].Source)

	// New addon vars should be added
	assert.Equal(t, "postgres://...", varMap["DATABASE_URL"].Value)
	assert.True(t, varMap["DATABASE_URL"].Sensitive)
	assert.Equal(t, "addon", varMap["DATABASE_URL"].Source)

	assert.Equal(t, "host.addon.app.miren", varMap["PGHOST"].Value)
	assert.Equal(t, "addon", varMap["PGHOST"].Source)
}

func TestMergeAddonVarsEmptySource(t *testing.T) {
	// Vars with empty source should be treated as manual (backward compat)
	existing := []core_v1alpha.Variable{
		{Key: "DATABASE_URL", Value: "old", Source: ""},
	}

	addonVars := []addon.Variable{
		{Key: "DATABASE_URL", Value: "new"},
	}

	result := mergeAddonVars(existing, addonVars)

	varMap := make(map[string]core_v1alpha.Variable, len(result))
	for _, v := range result {
		varMap[v.Key] = v
	}

	// Empty source treated as manual — should NOT be overridden
	assert.Equal(t, "old", varMap["DATABASE_URL"].Value)
}

func TestMergeAddonVarsEmpty(t *testing.T) {
	result := mergeAddonVars(nil, nil)
	assert.Empty(t, result)
}

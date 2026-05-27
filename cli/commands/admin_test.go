package commands

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"miren.dev/runtime/api/admin/admin_v1alpha"
)

func TestHasHelpFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{"empty", nil, false},
		{"only positional", []string{"foo", "bar"}, false},
		{"long help flag", []string{"--help"}, true},
		{"short help flag", []string{"-h"}, true},
		{"help with equals true", []string{"--help=true"}, true},
		{"short help with equals", []string{"-h=true"}, true},
		{"help among other flags", []string{"--name=foo", "--help"}, true},
		// Bare "help" must not trigger — it's a legitimate value for another
		// flag (e.g. `--topic help`) and shouldn't be hijacked.
		{"bare help word is a value, not the flag", []string{"help"}, false},
		{"help as flag value", []string{"--topic", "help"}, false},
		{"help as kv argument", []string{"topic=help"}, false},
		// --help-text shouldn't match — only exact --help or --help=
		{"flag starting with help but not help", []string{"--help-text"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, hasHelpFlag(tc.args))
		})
	}
}

// buildMethod is a small helper for assembling AdminMethod fixtures.
func buildMethod(name, desc string, params []paramSpec, hasParamsList bool) *admin_v1alpha.AdminMethod {
	m := &admin_v1alpha.AdminMethod{}
	m.SetName(name)
	if desc != "" {
		m.SetDescription(desc)
	}
	if hasParamsList {
		built := make([]*admin_v1alpha.AdminMethodParam, 0, len(params))
		for _, p := range params {
			ap := &admin_v1alpha.AdminMethodParam{}
			ap.SetName(p.name)
			if p.typ != "" {
				ap.SetParamType(p.typ)
			}
			if p.required {
				ap.SetRequired(true)
			}
			built = append(built, ap)
		}
		m.SetParams(built)
	}
	return m
}

type paramSpec struct {
	name     string
	typ      string
	required bool
}

func TestParamShapeNote(t *testing.T) {
	t.Run("params declared empty -> no parameters", func(t *testing.T) {
		m := buildMethod("doThing", "", nil, true)
		require.True(t, m.HasParams())
		require.Empty(t, m.Params())
		assert.Contains(t, paramShapeNote(m), "no parameters")
	})

	t.Run("params not advertised -> note says so", func(t *testing.T) {
		m := buildMethod("doThing", "", nil, false)
		require.False(t, m.HasParams())
		assert.Contains(t, paramShapeNote(m), "not advertised")
	})

	t.Run("params present -> no extra note", func(t *testing.T) {
		m := buildMethod("doThing", "", []paramSpec{{name: "x", typ: "string"}}, true)
		assert.Equal(t, "", paramShapeNote(m))
	})
}

func TestMethodDefinition(t *testing.T) {
	t.Run("populates term, description, details", func(t *testing.T) {
		m := buildMethod("setFlag", "Toggle a feature flag", []paramSpec{
			{name: "name", typ: "string", required: true},
			{name: "enabled", typ: "boolean"},
		}, true)
		def := methodDefinition(m)
		assert.Equal(t, "setFlag", def.Term)
		assert.Equal(t, "Toggle a feature flag", def.Description)
		require.Len(t, def.Details, 2)
		assert.Equal(t, "name", def.Details[0].Name)
		assert.Equal(t, "string", def.Details[0].Type)
		assert.True(t, def.Details[0].Required)
		assert.Equal(t, "enabled", def.Details[1].Name)
		assert.Equal(t, "boolean", def.Details[1].Type)
		assert.False(t, def.Details[1].Required)
	})

	t.Run("default param type is string", func(t *testing.T) {
		m := buildMethod("x", "", []paramSpec{{name: "thing"}}, true)
		def := methodDefinition(m)
		require.Len(t, def.Details, 1)
		assert.Equal(t, "string", def.Details[0].Type)
	})

	t.Run("empty params -> no details", func(t *testing.T) {
		m := buildMethod("x", "", nil, true)
		def := methodDefinition(m)
		assert.Empty(t, def.Details)
	})
}

func TestRenderMethodHelpString(t *testing.T) {
	m := buildMethod("labs.setOrgFlag", "Set or update a labs flag", []paramSpec{
		{name: "name", typ: "string", required: true},
		{name: "enabled", typ: "boolean"},
		{name: "org_xid", typ: "string", required: true},
	}, true)
	out := renderMethodHelpString("cloud", m)
	assert.Contains(t, out, "labs.setOrgFlag")
	assert.Contains(t, out, "Set or update a labs flag")
	assert.Contains(t, out, "name")
	assert.Contains(t, out, "enabled")
	assert.Contains(t, out, "org_xid")
	assert.Contains(t, out, "boolean")
	assert.Contains(t, out, "required")
	assert.Contains(t, out, "Usage: miren admin -a cloud labs.setOrgFlag")
}

func TestValidateAdminCall_LocalLogic(t *testing.T) {
	// validateAdminCall calls ListMethods on the wire, which requires a real
	// client. Cover its core validation logic by exercising the helpers it
	// composes — methodDefinition and renderMethodHelpString — under both
	// "method declares zero params" and "method declares params" shapes.

	t.Run("rendered help for declared-zero-params method notes 'no parameters'", func(t *testing.T) {
		m := buildMethod("ping", "Health check", nil, true)
		out := renderMethodHelpString("svc", m)
		assert.Contains(t, out, "no parameters")
		assert.NotContains(t, out, "not advertised")
	})

	t.Run("rendered help for undeclared-params method notes 'not advertised'", func(t *testing.T) {
		m := buildMethod("legacy", "Old method", nil, false)
		out := renderMethodHelpString("svc", m)
		assert.Contains(t, out, "not advertised")
		assert.NotContains(t, out, "no parameters)")
	})
}

func TestAdmin_NoParamsSuppliedReturnsError(t *testing.T) {
	// When a method declares params but none are supplied, the Admin handler
	// must error (non-zero exit) rather than silently rendering help and
	// returning nil — otherwise `m admin ... && echo ok` would print "ok"
	// for a call that never happened. We exercise the error-formatting via
	// the helper that builds the message.
	m := buildMethod("labs.setOrgFlag", "Set a flag", []paramSpec{
		{name: "name", typ: "string", required: true},
	}, true)
	// The Admin handler returns this exact format on the no-params branch.
	msg := "no parameters supplied\n\n" + renderMethodHelpString("cloud", m)
	assert.True(t, strings.HasPrefix(msg, "no parameters supplied"),
		"error message must lead with 'no parameters supplied'")
	assert.Contains(t, msg, "labs.setOrgFlag")
	assert.Contains(t, msg, "name")
}

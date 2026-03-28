package addon

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeIdentifier(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"my-app", 63, "my_app"},
		{"MyApp", 63, "myapp"},
		{"123app", 63, "a123app"},
		{"app_name", 63, "app_name"},
		{"app.name", 63, "appname"},
		{"APP-NAME", 63, "app_name"},
		{"", 63, "app"},
		{"a", 63, "a"},
		{"my-really-long-application-name-for-production", 32, "my_really_long_application_name_"},
		{"a-very-long-name-that-exceeds-63", 63, "a_very_long_name_that_exceeds_63"},
		{"no-limit", 0, "no_limit"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, SanitizeIdentifier(tt.input, tt.maxLen))
		})
	}
}

func TestParseStorageGb(t *testing.T) {
	assert.Equal(t, int64(1), ParseStorageGb("1Gi"))
	assert.Equal(t, int64(50), ParseStorageGb("50Gi"))
	assert.Equal(t, int64(1), ParseStorageGb("bad"))
	assert.Equal(t, int64(1), ParseStorageGb(""))
	assert.Equal(t, int64(1), ParseStorageGb("0Gi"))
}

func TestIsSharedVariant(t *testing.T) {
	assert.True(t, IsSharedVariant("shared"))
	assert.False(t, IsSharedVariant("small"))
	assert.False(t, IsSharedVariant(""))
}

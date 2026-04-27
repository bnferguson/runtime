package commands

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLooksLikeAppEnvVar(t *testing.T) {
	testCases := []struct {
		key      string
		expected bool
	}{
		// Prefix matches
		{"DATABASE_URL", true},
		{"DB_HOST", true},
		{"REDIS_URL", true},
		{"AWS_ACCESS_KEY_ID", true},
		{"STRIPE_API_KEY", true},
		{"SENTRY_DSN", true},

		// Suffix matches
		{"MY_API_KEY", true},
		{"CUSTOM_SECRET", true},
		{"SERVICE_TOKEN", true},
		{"BACKEND_URL", true},
		{"APP_PASSWORD", true},

		// Standalone names
		{"SECRET_KEY_BASE", true},
		{"MASTER_KEY", true},

		// Should NOT match
		{"PATH", false},
		{"HOME", false},
		{"USER", false},
		{"SHELL", false},
		{"TERM", false},
		{"MY_RANDOM_VAR", false},
		{"SOME_THING", false},
	}

	for _, tc := range testCases {
		t.Run(tc.key, func(t *testing.T) {
			result := looksLikeAppEnvVar(tc.key)
			assert.Equal(t, tc.expected, result, "looksLikeAppEnvVar(%q)", tc.key)
		})
	}
}

func TestLooksLikeSensitive(t *testing.T) {
	testCases := []struct {
		key      string
		expected bool
	}{
		// Substring patterns
		{"API_KEY", true},
		{"SECRET_KEY_BASE", true},
		{"DATABASE_PASSWORD", true},
		{"AUTH_TOKEN", true},
		{"PRIVATE_KEY", true},
		{"AWS_SECRET_ACCESS_KEY", true},
		{"STRIPE_API_KEY", true},

		// Connection-string URLs typically embed credentials.
		{"DATABASE_URL", true},
		{"DATABASE_URI", true},
		{"REDIS_URL", true},
		{"POSTGRES_URL", true},
		{"POSTGRESQL_URL", true},
		{"MYSQL_URL", true},
		{"MARIADB_URL", true},
		{"MONGO_URL", true},
		{"MONGODB_URI", true},
		{"ELASTICSEARCH_URL", true},
		{"RABBITMQ_URL", true},
		{"AMQP_URL", true},
		{"CLOUDAMQP_URL", true},
		{"CLOUDINARY_URL", true},
		{"DB_REPLICA_URL", true},

		// DSNs always embed credentials.
		{"SENTRY_DSN", true},
		{"BUGSNAG_DSN", true},

		// Plain hosts/ports without creds stay unmasked.
		{"APP_HOST", false},
		{"WEBHOOK_URL", false},
		{"CALLBACK_URL", false},
		{"PORT", false},
	}

	for _, tc := range testCases {
		t.Run(tc.key, func(t *testing.T) {
			result := looksLikeSensitive(tc.key)
			assert.Equal(t, tc.expected, result, "looksLikeSensitive(%q)", tc.key)
		})
	}
}

func TestMaskValue(t *testing.T) {
	testCases := []struct {
		name      string
		value     string
		sensitive bool
		expected  string
	}{
		{"long_sensitive_keeps_2_char_prefix", "mysecret123", true, "my••••••••"},
		{"short_sensitive_fully_masked", "short", true, "••••••••"},
		{"empty_sensitive", "", true, ""},
		{"plain_visible", "visible", false, "visible"},
		{"empty_visible", "", false, ""},
		{"longer_sensitive", "verylongsecretvalue", true, "ve••••••••"},

		// Control / ANSI escape characters must not pass through verbatim
		// or they could spoof or corrupt CLI output and CI logs.
		{"newline_escaped", "line1\nline2", false, `line1\nline2`},
		{"crlf_escaped", "a\r\nb", false, `a\r\nb`},
		{"tab_escaped", "a\tb", false, `a\tb`},
		{"ansi_escape_escaped", "\x1b[31mred\x1b[0m", false, `\x1b[31mred\x1b[0m`},
		{"sensitive_prefix_escaped", "\x1b[31mxxxxxxxxxx", true, `\x1b[••••••••`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := MaskValue(tc.value, tc.sensitive)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestDetectLocalEnvVars(t *testing.T) {
	// Save original environment
	origEnv := os.Environ()
	defer func() {
		os.Clearenv()
		for _, e := range origEnv {
			parts := splitEnv(e)
			if len(parts) == 2 {
				os.Setenv(parts[0], parts[1])
			}
		}
	}()

	// Set up a controlled environment
	os.Clearenv()
	os.Setenv("DATABASE_URL", "postgres://localhost/test")
	os.Setenv("REDIS_URL", "redis://localhost:6379")
	os.Setenv("SECRET_KEY_BASE", "abc123secret")
	os.Setenv("MY_CUSTOM_API_KEY", "custom-key-value")
	os.Setenv("PATH", "/usr/bin")     // Should be ignored
	os.Setenv("RANDOM_VAR", "random") // Should not be detected as app-related

	// Test with detected vars
	detected := []string{"DATABASE_URL", "STRIPE_API_KEY", "SENTRY_DSN"}
	result := DetectLocalEnvVars(detected)

	// DATABASE_URL should be in Available (detected + found locally)
	assert.Len(t, result.Available, 1)
	assert.Equal(t, "DATABASE_URL", result.Available[0].Key)
	assert.Equal(t, "postgres://localhost/test", result.Available[0].Value)
	assert.True(t, result.Available[0].HasValue)

	// STRIPE_API_KEY and SENTRY_DSN should be in Missing (detected but not found)
	assert.Len(t, result.Missing, 2)
	missingKeys := []string{result.Missing[0].Key, result.Missing[1].Key}
	assert.Contains(t, missingKeys, "STRIPE_API_KEY")
	assert.Contains(t, missingKeys, "SENTRY_DSN")

	// REDIS_URL, SECRET_KEY_BASE, and MY_CUSTOM_API_KEY should be in Additional
	// (found locally, look app-related, not in detected list)
	additionalKeys := make([]string, len(result.Additional))
	for i, ev := range result.Additional {
		additionalKeys[i] = ev.Key
	}
	assert.Contains(t, additionalKeys, "REDIS_URL")
	assert.Contains(t, additionalKeys, "SECRET_KEY_BASE")
	assert.Contains(t, additionalKeys, "MY_CUSTOM_API_KEY")

	// PATH and RANDOM_VAR should NOT be in any list
	allKeys := make(map[string]bool)
	for _, ev := range result.Available {
		allKeys[ev.Key] = true
	}
	for _, ev := range result.Missing {
		allKeys[ev.Key] = true
	}
	for _, ev := range result.Additional {
		allKeys[ev.Key] = true
	}
	assert.NotContains(t, allKeys, "PATH")
	assert.NotContains(t, allKeys, "RANDOM_VAR")
}

func TestDetectLocalEnvVars_NoDetected(t *testing.T) {
	// Save original environment
	origEnv := os.Environ()
	defer func() {
		os.Clearenv()
		for _, e := range origEnv {
			parts := splitEnv(e)
			if len(parts) == 2 {
				os.Setenv(parts[0], parts[1])
			}
		}
	}()

	// Set up a controlled environment
	os.Clearenv()
	os.Setenv("DATABASE_URL", "postgres://localhost/test")
	os.Setenv("API_KEY", "my-api-key")

	// Test with no detected vars
	result := DetectLocalEnvVars(nil)

	// Available and Missing should be empty
	assert.Empty(t, result.Available)
	assert.Empty(t, result.Missing)

	// DATABASE_URL and API_KEY should be in Additional
	additionalKeys := make([]string, len(result.Additional))
	for i, ev := range result.Additional {
		additionalKeys[i] = ev.Key
	}
	assert.Contains(t, additionalKeys, "DATABASE_URL")
	assert.Contains(t, additionalKeys, "API_KEY")
}

func TestDetectLocalEnvVars_SensitiveFlag(t *testing.T) {
	// Save original environment
	origEnv := os.Environ()
	defer func() {
		os.Clearenv()
		for _, e := range origEnv {
			parts := splitEnv(e)
			if len(parts) == 2 {
				os.Setenv(parts[0], parts[1])
			}
		}
	}()

	// Set up environment with sensitive and non-sensitive vars
	os.Clearenv()
	os.Setenv("DATABASE_URL", "postgres://localhost/test")
	os.Setenv("SECRET_KEY_BASE", "abc123")
	os.Setenv("API_TOKEN", "token123")

	detected := []string{"DATABASE_URL", "SECRET_KEY_BASE", "API_TOKEN"}
	result := DetectLocalEnvVars(detected)

	// Check sensitive flags
	for _, ev := range result.Available {
		switch ev.Key {
		case "DATABASE_URL":
			assert.True(t, ev.Sensitive, "DATABASE_URL should be sensitive (embeds creds)")
		case "SECRET_KEY_BASE":
			assert.True(t, ev.Sensitive, "SECRET_KEY_BASE should be sensitive")
		case "API_TOKEN":
			assert.True(t, ev.Sensitive, "API_TOKEN should be sensitive")
		}
	}
}

// splitEnv splits an environment variable string into key and value
func splitEnv(e string) []string {
	for i := 0; i < len(e); i++ {
		if e[i] == '=' {
			return []string{e[:i], e[i+1:]}
		}
	}
	return []string{e}
}

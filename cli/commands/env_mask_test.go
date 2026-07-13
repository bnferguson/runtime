package commands

import (
	"bytes"
	"strings"
	"testing"

	"miren.dev/runtime/api/app/app_v1alpha"
)

func TestMaskURLUserinfo(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  string
	}{
		{
			name:  "postgres url with user and password",
			value: "postgres://admin:s3cr3t@db.internal:5432/app",
			want:  "postgres://" + envRedacted + "@db.internal:5432/app",
		},
		{
			name:  "password only userinfo",
			value: "redis://:hunter2@cache:6379/0",
			want:  "redis://" + envRedacted + "@cache:6379/0",
		},
		{
			name:  "bare username userinfo",
			value: "amqp://guest@broker:5672",
			want:  "amqp://" + envRedacted + "@broker:5672",
		},
		{
			name:  "plain value passes through",
			value: "production",
			want:  "production",
		},
		{
			name:  "url without userinfo passes through",
			value: "https://api.example.com/v1/webhook",
			want:  "https://api.example.com/v1/webhook",
		},
		{
			name:  "email-shaped value without scheme passes through",
			value: "admin@example.com",
			want:  "admin@example.com",
		},
		{
			name:  "value with equals and query passes through",
			value: "http://example.com?a=1&b=2",
			want:  "http://example.com?a=1&b=2",
		},
		{
			name:  "multiple urls both redacted",
			value: "primary=postgres://u:p@h1/db secondary=postgres://u2:p2@h2/db",
			want:  "primary=postgres://" + envRedacted + "@h1/db secondary=postgres://" + envRedacted + "@h2/db",
		},
		{
			name:  "empty value",
			value: "",
			want:  "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := maskURLUserinfo(tc.value)
			if got != tc.want {
				t.Errorf("maskURLUserinfo(%q) = %q, want %q", tc.value, got, tc.want)
			}
		})
	}
}

// TestMaskURLUserinfoNoLeak guards the security-critical property: once redacted,
// no fragment of the original credential should survive in the output.
func TestMaskURLUserinfoNoLeak(t *testing.T) {
	value := "postgres://admin:supersecretpassword@db.internal/app"
	got := maskURLUserinfo(value)
	for _, leaked := range []string{"admin", "supersecretpassword", "supersecret", "admin:super"} {
		if strings.Contains(got, leaked) {
			t.Errorf("redacted value %q leaked credential fragment %q", got, leaked)
		}
	}
}

// TestPrintEnvTableDoesNotLeak drives the real table renderer end-to-end and
// asserts the DATABASE_URL password never reaches the output unless unmasked.
// This is the exact surface (name-match-only) that leaked in the reported bug.
func TestPrintEnvTableDoesNotLeak(t *testing.T) {
	nv := &app_v1alpha.NamedValue{}
	nv.SetKey("DATABASE_URL")
	nv.SetValue("postgres://appuser:s3cr3tpassword@db.internal:5432/app")
	nv.SetSensitive(false) // deliberately NOT marked sensitive — key name gives no hint
	entries := []envVarEntry{{nv: nv, service: ""}}

	// The table has no bulk-reveal path: it always masks. Single-secret reveal
	// is only via `miren env get <key> --unmask`.
	var buf bytes.Buffer
	ctx := &Context{Stdout: &buf}
	printEnvTable(ctx, entries)
	out := buf.String()
	if strings.Contains(out, "s3cr3tpassword") {
		t.Errorf("env table leaked DATABASE_URL password:\n%s", out)
	}
	if !strings.Contains(out, "DATABASE_URL") {
		t.Errorf("env table dropped the key:\n%s", out)
	}
}

func TestMaskEnvValue(t *testing.T) {
	cases := []struct {
		name      string
		value     string
		sensitive bool
		unmask    bool
		want      string
	}{
		{
			name:      "sensitive var is fully redacted",
			value:     "supersecret",
			sensitive: true,
			want:      envRedacted,
		},
		{
			name:  "non-sensitive url masks embedded credentials",
			value: "postgres://admin:s3cr3t@db/app",
			want:  "postgres://" + envRedacted + "@db/app",
		},
		{
			name:  "non-sensitive plain value shows through",
			value: "production",
			want:  "production",
		},
		{
			name:      "unmask reveals sensitive value",
			value:     "supersecret",
			sensitive: true,
			unmask:    true,
			want:      "supersecret",
		},
		{
			name:   "unmask reveals url credentials",
			value:  "postgres://admin:s3cr3t@db/app",
			unmask: true,
			want:   "postgres://admin:s3cr3t@db/app",
		},
		{
			// Display path escapes control/ANSI sequences so a hostile value
			// can't corrupt terminal or CI output.
			name:  "non-sensitive value escapes ansi and control chars",
			value: "\x1b[31mred\x1b[0m\nsecond",
			want:  `\x1b[31mred\x1b[0m\nsecond`,
		},
		{
			// unmask is a literal-bytes escape hatch: no masking AND no
			// escaping, so scripts capture the exact value.
			name:   "unmask returns literal bytes without escaping",
			value:  "line1\nline2",
			unmask: true,
			want:   "line1\nline2",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := maskEnvValue(tc.value, tc.sensitive, tc.unmask)
			if got != tc.want {
				t.Errorf("maskEnvValue(%q, sensitive=%v, unmask=%v) = %q, want %q",
					tc.value, tc.sensitive, tc.unmask, got, tc.want)
			}
		})
	}
}

// TestMaskEnvValueRedactionHidesLengthAndFirstChar guards against the old
// value[:1]+Repeat("*") and value[:2]+dots behaviors that leaked the first
// character(s) and/or exact length of a sensitive value.
func TestMaskEnvValueRedactionHidesLengthAndFirstChar(t *testing.T) {
	short := maskEnvValue("ab", true, false)
	long := maskEnvValue("a-very-long-secret-value-here", true, false)
	if short != long {
		t.Errorf("redaction width leaks length: short=%q long=%q", short, long)
	}
	if strings.HasPrefix(short, "a") {
		t.Errorf("redaction leaks first character: %q", short)
	}
}

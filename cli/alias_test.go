package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"miren.dev/mflags"
)

func TestTokenizeCommand(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{
			name:  "simple words",
			input: "app logs -f",
			want:  []string{"app", "logs", "-f"},
		},
		{
			name:  "double quoted string",
			input: `app exec -i --reuse "bin/rails console"`,
			want:  []string{"app", "exec", "-i", "--reuse", "bin/rails console"},
		},
		{
			name:  "single quoted string",
			input: `app exec -i 'bin/rails console'`,
			want:  []string{"app", "exec", "-i", "bin/rails console"},
		},
		{
			name:  "escaped quote in double quotes",
			input: `echo "hello \"world\""`,
			want:  []string{"echo", `hello "world"`},
		},
		{
			name:  "multiple spaces between words",
			input: "app   logs   -f",
			want:  []string{"app", "logs", "-f"},
		},
		{
			name:    "unterminated double quote",
			input:   `app exec "unclosed`,
			wantErr: true,
		},
		{
			name:    "unterminated single quote",
			input:   `app exec 'unclosed`,
			wantErr: true,
		},
		{
			name:  "empty string",
			input: "",
			want:  nil,
		},
		{
			name:  "only spaces",
			input: "   ",
			want:  nil,
		},
		{
			name:  "single word",
			input: "version",
			want:  []string{"version"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tokenizeCommand(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestExpandAlias(t *testing.T) {
	newDispatcher := func() *mflags.Dispatcher {
		d := mflags.NewDispatcher("miren")
		fs := mflags.NewFlagSet("version")
		d.Dispatch("version", mflags.NewCommand(fs, func(fs *mflags.FlagSet, args []string) error {
			return nil
		}, mflags.WithUsage("Print version")))
		fs2 := mflags.NewFlagSet("app list")
		d.Dispatch("app list", mflags.NewCommand(fs2, func(fs *mflags.FlagSet, args []string) error {
			return nil
		}, mflags.WithUsage("List apps")))
		return d
	}

	t.Run("no args returns unchanged", func(t *testing.T) {
		d := newDispatcher()
		got, err := expandAlias(d, nil)
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("no config returns unchanged", func(t *testing.T) {
		d := newDispatcher()
		args := []string{"foo", "bar"}
		got, err := expandAlias(d, args)
		require.NoError(t, err)
		assert.Equal(t, args, got)
	})

	// Note: testing actual alias expansion with config files requires
	// integration tests since expandAlias calls LoadAppConfig which reads
	// from the filesystem. The "no config" case above covers the early-return
	// path. The shadow check and expansion logic are exercised in blackbox tests.
}

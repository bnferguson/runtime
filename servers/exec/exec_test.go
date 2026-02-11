package exec

import (
	"testing"

	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"miren.dev/runtime/api/core/core_v1alpha"
	"miren.dev/runtime/api/exec/exec_v1alpha"
)

func TestCommand(t *testing.T) {
	server := &Server{}

	tests := []struct {
		name     string
		cfgSpec  *core_v1alpha.ConfigSpec
		service  string
		expected string
	}{
		{
			name: "console command with entrypoint",
			cfgSpec: &core_v1alpha.ConfigSpec{
				Entrypoint: "mise exec --",
				Services: []core_v1alpha.ConfigSpecServices{
					{Name: "console", Command: "bin/rails console"},
				},
			},
			service:  "console",
			expected: "mise exec -- bin/rails console",
		},
		{
			name: "console command without entrypoint",
			cfgSpec: &core_v1alpha.ConfigSpec{
				Services: []core_v1alpha.ConfigSpecServices{
					{Name: "console", Command: "bin/rails console"},
				},
			},
			service:  "console",
			expected: "bin/rails console",
		},
		{
			name: "no matching service",
			cfgSpec: &core_v1alpha.ConfigSpec{
				Entrypoint: "mise exec --",
				Services: []core_v1alpha.ConfigSpecServices{
					{Name: "web", Command: "bin/rails server"},
				},
			},
			service:  "console",
			expected: "",
		},
		{
			name: "empty command for service",
			cfgSpec: &core_v1alpha.ConfigSpec{
				Services: []core_v1alpha.ConfigSpecServices{
					{Name: "console", Command: ""},
				},
			},
			service:  "console",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := server.command(tt.cfgSpec, tt.service)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSpec(t *testing.T) {
	server := &Server{}

	baseOCISpec := &oci.Spec{
		Process: &specs.Process{
			Cwd: "/app",
			Env: []string{"PATH=/usr/bin", "HOME=/home/app"},
			User: specs.User{
				UID: 1000,
				GID: 1000,
			},
		},
	}

	tests := []struct {
		name         string
		opts         *exec_v1alpha.ShellOptions
		cfgSpec      *core_v1alpha.ConfigSpec
		expectedArgs []string
		description  string
	}{
		{
			name:         "nil config, no command - defaults to /bin/sh",
			opts:         &exec_v1alpha.ShellOptions{},
			cfgSpec:      nil,
			expectedArgs: []string{"/bin/sh"},
			description:  "Non-app containers (like postgres) should get plain shell",
		},
		{
			name: "nil config, with command - runs command directly",
			opts: func() *exec_v1alpha.ShellOptions {
				o := &exec_v1alpha.ShellOptions{}
				o.SetCommand([]string{"psql", "-U", "postgres"})
				return o
			}(),
			cfgSpec:      nil,
			expectedArgs: []string{"psql", "-U", "postgres"},
			description:  "Non-app containers should run commands without entrypoint",
		},
		{
			name: "config with entrypoint, no command - wraps shell with entrypoint",
			opts: &exec_v1alpha.ShellOptions{},
			cfgSpec: &core_v1alpha.ConfigSpec{
				Entrypoint: "mise exec --",
			},
			expectedArgs: []string{"/bin/sh", "-c", "exec mise exec -- /bin/sh"},
			description:  "App containers should have entrypoint applied to shell",
		},
		{
			name: "config with entrypoint, with command - wraps command with entrypoint",
			opts: func() *exec_v1alpha.ShellOptions {
				o := &exec_v1alpha.ShellOptions{}
				o.SetCommand([]string{"bin/rails", "runner", "puts 'hello'"})
				return o
			}(),
			cfgSpec: &core_v1alpha.ConfigSpec{
				Entrypoint: "mise exec --",
			},
			expectedArgs: []string{"/bin/sh", "-c", "exec mise exec -- bin/rails runner puts 'hello'"},
			description:  "App containers should have entrypoint applied to commands",
		},
		{
			name:         "config without entrypoint, no command - plain shell",
			opts:         &exec_v1alpha.ShellOptions{},
			cfgSpec:      &core_v1alpha.ConfigSpec{},
			expectedArgs: []string{"/bin/sh"},
			description:  "App containers without entrypoint get plain shell",
		},
		{
			name: "config without entrypoint, with command - runs command directly",
			opts: func() *exec_v1alpha.ShellOptions {
				o := &exec_v1alpha.ShellOptions{}
				o.SetCommand([]string{"ls", "-la"})
				return o
			}(),
			cfgSpec:      &core_v1alpha.ConfigSpec{},
			expectedArgs: []string{"ls", "-la"},
			description:  "App containers without entrypoint run commands directly",
		},
		{
			name: "config with console command, no user command - uses console command",
			opts: &exec_v1alpha.ShellOptions{},
			cfgSpec: &core_v1alpha.ConfigSpec{
				Entrypoint: "mise exec --",
				Services: []core_v1alpha.ConfigSpecServices{
					{Name: "console", Command: "bin/rails console"},
				},
			},
			expectedArgs: []string{"/bin/sh", "-c", "exec mise exec -- bin/rails console"},
			description:  "Interactive exec should use configured console command",
		},
		{
			name: "config with console command but no entrypoint",
			opts: &exec_v1alpha.ShellOptions{},
			cfgSpec: &core_v1alpha.ConfigSpec{
				Services: []core_v1alpha.ConfigSpecServices{
					{Name: "console", Command: "bin/rails console"},
				},
			},
			expectedArgs: []string{"/bin/sh", "-c", "exec bin/rails console"},
			description:  "Console command without entrypoint",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proc, err := server.spec(tt.opts, baseOCISpec, tt.cfgSpec)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedArgs, proc.Args, tt.description)

			// Verify inherited properties from OCI spec
			assert.Equal(t, baseOCISpec.Process.Cwd, proc.Cwd)
			assert.Equal(t, baseOCISpec.Process.Env, proc.Env)
			assert.Equal(t, baseOCISpec.Process.User, proc.User)
		})
	}
}

func TestSpec_TerminalHandling(t *testing.T) {
	server := &Server{}

	baseOCISpec := &oci.Spec{
		Process: &specs.Process{
			Cwd: "/app",
		},
	}

	t.Run("with window size sets terminal mode", func(t *testing.T) {
		opts := &exec_v1alpha.ShellOptions{}
		opts.SetWinSize(&exec_v1alpha.WindowSize{})
		// Set width/height on the WindowSize
		ws := opts.WinSize()
		ws.SetWidth(120)
		ws.SetHeight(40)

		proc, err := server.spec(opts, baseOCISpec, nil)
		require.NoError(t, err)
		assert.True(t, proc.Terminal)
		assert.NotNil(t, proc.ConsoleSize)
		assert.Equal(t, uint(120), proc.ConsoleSize.Width)
		assert.Equal(t, uint(40), proc.ConsoleSize.Height)
	})

	t.Run("without window size no terminal mode", func(t *testing.T) {
		opts := &exec_v1alpha.ShellOptions{}

		proc, err := server.spec(opts, baseOCISpec, nil)
		require.NoError(t, err)
		assert.False(t, proc.Terminal)
		assert.Nil(t, proc.ConsoleSize)
	})
}

// TestEntrypointNotAppliedToCustomImageContainers documents the expected behavior
// for the bug fix: containers with custom images (like postgres) should NOT have
// the app's entrypoint applied.
func TestEntrypointNotAppliedToCustomImageContainers(t *testing.T) {
	server := &Server{}

	baseOCISpec := &oci.Spec{
		Process: &specs.Process{
			Cwd: "/var/lib/postgresql/data",
		},
	}

	t.Run("postgres container without version gets plain shell", func(t *testing.T) {
		// When exec'ing into a postgres container, ver should be nil
		// because the container's image doesn't match the app's image
		opts := &exec_v1alpha.ShellOptions{}

		proc, err := server.spec(opts, baseOCISpec, nil)
		require.NoError(t, err)
		assert.Equal(t, []string{"/bin/sh"}, proc.Args,
			"Postgres container should get plain shell, not app entrypoint")
	})

	t.Run("postgres container runs psql directly", func(t *testing.T) {
		opts := &exec_v1alpha.ShellOptions{}
		opts.SetCommand([]string{"psql", "-U", "postgres", "-d", "mydb"})

		proc, err := server.spec(opts, baseOCISpec, nil)
		require.NoError(t, err)
		assert.Equal(t, []string{"psql", "-U", "postgres", "-d", "mydb"}, proc.Args,
			"Postgres commands should run directly without app entrypoint")
	})

	t.Run("app container with same image gets entrypoint", func(t *testing.T) {
		// When exec'ing into an app container, cfgSpec is populated
		// because the container's image matches the app's image
		opts := &exec_v1alpha.ShellOptions{}
		cfgSpec := &core_v1alpha.ConfigSpec{
			Entrypoint: "mise exec --",
		}

		proc, err := server.spec(opts, baseOCISpec, cfgSpec)
		require.NoError(t, err)
		assert.Equal(t, []string{"/bin/sh", "-c", "exec mise exec -- /bin/sh"}, proc.Args,
			"App container should have entrypoint applied")
	})
}

// TestImageMatchesAppVersion tests the image matching logic that determines
// whether to apply the app's entrypoint when exec'ing into a container.
func TestImageMatchesAppVersion(t *testing.T) {
	testCases := []struct {
		name            string
		containerImage  string
		appVersionImage string
		shouldMatch     bool
		description     string
	}{
		{
			name:            "exact match",
			containerImage:  "myapp:v1.2.3",
			appVersionImage: "myapp:v1.2.3",
			shouldMatch:     true,
			description:     "Identical image references should match",
		},
		{
			name:            "different tags",
			containerImage:  "myapp:v1.2.3",
			appVersionImage: "myapp:v1.2.4",
			shouldMatch:     false,
			description:     "Different tags should not match",
		},
		{
			name:            "postgres vs app",
			containerImage:  "postgres:16",
			appVersionImage: "myapp:v1.2.3",
			shouldMatch:     false,
			description:     "Postgres container should not match app image",
		},
		{
			name:            "redis vs app",
			containerImage:  "redis:7-alpine",
			appVersionImage: "myapp:v1.2.3",
			shouldMatch:     false,
			description:     "Redis container should not match app image",
		},
		{
			name:            "with docker.io registry prefix",
			containerImage:  "docker.io/library/myapp:v1",
			appVersionImage: "myapp:v1",
			shouldMatch:     true,
			description:     "docker.io registry prefix should be handled via suffix match",
		},
		{
			name:            "with custom registry prefix",
			containerImage:  "ghcr.io/myorg/myapp:v1",
			appVersionImage: "myapp:v1",
			shouldMatch:     true,
			description:     "Custom registry prefix should be handled via suffix match",
		},
		{
			name:            "both have same registry",
			containerImage:  "gcr.io/project/myapp:v1",
			appVersionImage: "gcr.io/project/myapp:v1",
			shouldMatch:     true,
			description:     "Both with same registry should match exactly",
		},
		{
			name:            "different registries same image name",
			containerImage:  "gcr.io/project/myapp:v1",
			appVersionImage: "docker.io/library/myapp:v1",
			shouldMatch:     false,
			description:     "Different registries should not match",
		},
		{
			name:            "empty app version image",
			containerImage:  "postgres:16",
			appVersionImage: "",
			shouldMatch:     false,
			description:     "Empty app version image should not match",
		},
		{
			name:            "similar but not matching",
			containerImage:  "myapp-sidecar:v1",
			appVersionImage: "myapp:v1",
			shouldMatch:     false,
			description:     "Similar names should not match unless exact or suffix",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := imageMatchesAppVersion(tc.containerImage, tc.appVersionImage)
			assert.Equal(t, tc.shouldMatch, result, tc.description)
		})
	}
}

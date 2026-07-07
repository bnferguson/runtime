package commands

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsSupportedRuntime(t *testing.T) {
	r := require.New(t)

	r.True(isSupportedRuntime("docker"))
	r.True(isSupportedRuntime("podman"))
	r.False(isSupportedRuntime("nerdctl"))
	r.False(isSupportedRuntime(""))
	r.False(isSupportedRuntime("Docker")) // case-sensitive; callers lowercase first
}

func TestResolveContainerRuntimeRejectsUnsupportedOverride(t *testing.T) {
	r := require.New(t)

	// An unsupported override is rejected up front, before we ever try to shell
	// out to check availability, so this is safe to assert without a real engine.
	_, err := resolveContainerRuntime("nerdctl")
	r.Error(err)
	r.Contains(err.Error(), "unsupported container runtime")
	r.Contains(err.Error(), "docker, podman")
}

func TestResolveContainerRuntimeNormalizesOverride(t *testing.T) {
	// Mixed case / surrounding whitespace should normalize to a supported
	// runtime rather than being rejected as unsupported. We can't assert success
	// (docker/podman may not be installed in CI), but we can assert it does NOT
	// fail with the "unsupported" error — i.e. it got past normalization.
	_, err := resolveContainerRuntime("  Podman ")
	if err != nil {
		require.NotContains(t, err.Error(), "unsupported container runtime")
	}
}

func TestContainerRuntimeCommandUsesBin(t *testing.T) {
	r := require.New(t)

	rt := containerRuntime{bin: "podman"}
	cmd := rt.command("ps", "-a")
	// exec.Command resolves Args[0] to the bin name; Path may be absolute if
	// found on PATH, so assert on Args which preserve what we asked for.
	r.Equal([]string{"podman", "ps", "-a"}, cmd.Args)
	r.Equal("podman", rt.String())
}

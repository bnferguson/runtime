package commands

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
)

// containerRuntime is the container CLI Miren drives to run the server inside a
// container — either Docker or Podman. Podman implements the subset of the
// Docker CLI we use (run/ps/logs/exec/cp/volume/inspect/info/start/stop/rm)
// with matching flags, so the only thing that varies is which binary we invoke.
type containerRuntime struct {
	// bin is the executable name, e.g. "docker" or "podman".
	bin string
}

// String returns the runtime's binary name, handy for help text and the
// copy-pasteable management commands we print after an install.
func (r containerRuntime) String() string { return r.bin }

// command builds an *exec.Cmd that invokes this runtime with the given args.
func (r containerRuntime) command(args ...string) *exec.Cmd {
	return exec.Command(r.bin, args...)
}

// containerRuntimeEnvVar overrides runtime auto-detection when set, mirroring
// the --runtime flag. Handy for scripted installs on hosts that have both.
const containerRuntimeEnvVar = "MIREN_CONTAINER_RUNTIME"

// supportedContainerRuntimes lists the binaries we know how to drive, in
// auto-detect preference order. Docker is first: it's the more common install
// and its always-on daemon gives restart-on-reboot for free, so when a host has
// both we default to it and let Podman users opt in explicitly.
var supportedContainerRuntimes = []string{"docker", "podman"}

// errNoContainerRuntime is returned when neither runtime is installed and
// running. The message names both options so the user knows they have a choice.
var errNoContainerRuntime = errors.New(
	"no container runtime found: install Docker (https://www.docker.com/products/docker-desktop) " +
		"or Podman (https://podman.io), then make sure it's running")

// resolveContainerRuntime picks the container runtime to drive. Precedence:
// an explicit override (the --runtime flag, else the MIREN_CONTAINER_RUNTIME
// env var), then auto-detection preferring Docker, then Podman. It only returns
// an error when the requested override is unusable, or when nothing usable is
// found at all.
func resolveContainerRuntime(override string) (containerRuntime, error) {
	if override == "" {
		override = os.Getenv(containerRuntimeEnvVar)
	}

	if override != "" {
		bin := strings.ToLower(strings.TrimSpace(override))
		if !isSupportedRuntime(bin) {
			return containerRuntime{}, fmt.Errorf(
				"unsupported container runtime %q (supported: %s)",
				override, strings.Join(supportedContainerRuntimes, ", "))
		}
		rt := containerRuntime{bin: bin}
		if err := rt.checkAvailable(); err != nil {
			return containerRuntime{}, err
		}
		return rt, nil
	}

	for _, bin := range supportedContainerRuntimes {
		rt := containerRuntime{bin: bin}
		if err := rt.checkAvailable(); err == nil {
			return rt, nil
		}
	}

	return containerRuntime{}, errNoContainerRuntime
}

func isSupportedRuntime(bin string) bool {
	return slices.Contains(supportedContainerRuntimes, bin)
}

// checkAvailable verifies the runtime binary is on PATH and its engine is
// responding. `info` talks to the daemon (Docker) or queries the local engine
// (Podman), so a clean exit means we can actually drive it.
func (r containerRuntime) checkAvailable() error {
	if _, err := exec.LookPath(r.bin); err != nil {
		return fmt.Errorf("%s command not found", r.bin)
	}
	if err := r.command("info").Run(); err != nil {
		return fmt.Errorf("%s is installed but not responding (is it running?)", r.bin)
	}
	return nil
}

// availableContainerRuntimes returns every supported runtime that's currently
// installed and responding. Order follows supportedContainerRuntimes. Used by
// diagnostics (debug bundle) that want to collect from whichever engine is
// actually hosting the miren container rather than a single preferred one.
func availableContainerRuntimes() []containerRuntime {
	var found []containerRuntime
	for _, bin := range supportedContainerRuntimes {
		rt := containerRuntime{bin: bin}
		if rt.checkAvailable() == nil {
			found = append(found, rt)
		}
	}
	return found
}

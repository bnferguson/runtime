package containerenv

import (
	"os"
	"path/filepath"
	"testing"
)

// isolateProbes points all probe paths at a fresh temp dir so a test never sees
// the real host/container markers, and restores them on cleanup.
func isolateProbes(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	origDocker, origPodman, origCgroups := dockerEnvPath, podmanEnvPath, cgroupPaths
	t.Cleanup(func() {
		dockerEnvPath, podmanEnvPath, cgroupPaths = origDocker, origPodman, origCgroups
	})

	dockerEnvPath = filepath.Join(dir, "dockerenv")
	podmanEnvPath = filepath.Join(dir, "containerenv")
	cgroupPaths = []string{filepath.Join(dir, "self-cgroup"), filepath.Join(dir, "pid1-cgroup")}
	return dir
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestDetect_NoMarkers(t *testing.T) {
	isolateProbes(t)
	if detect() {
		t.Fatal("detect() = true with no markers present, want false")
	}
}

func TestDetect_DockerEnv(t *testing.T) {
	isolateProbes(t)
	writeFile(t, dockerEnvPath, "")
	if !detect() {
		t.Fatal("detect() = false with /.dockerenv present, want true")
	}
}

func TestDetect_PodmanEnv(t *testing.T) {
	isolateProbes(t)
	writeFile(t, podmanEnvPath, "")
	if !detect() {
		t.Fatal("detect() = false with /run/.containerenv present, want true")
	}
}

func TestDetect_CgroupMarker(t *testing.T) {
	for _, marker := range cgroupMarkers {
		t.Run(marker, func(t *testing.T) {
			isolateProbes(t)
			writeFile(t, cgroupPaths[0], "12:pids:/"+marker+"/abc123\n")
			if !detect() {
				t.Fatalf("detect() = false with %q cgroup marker, want true", marker)
			}
		})
	}
}

func TestDetect_CgroupNoMarker(t *testing.T) {
	isolateProbes(t)
	writeFile(t, cgroupPaths[0], "0::/user.slice/session-1.scope\n")
	writeFile(t, cgroupPaths[1], "0::/init.scope\n")
	if detect() {
		t.Fatal("detect() = true for a non-container cgroup, want false")
	}
}

func TestInContainer_Caches(t *testing.T) {
	// InContainer memoizes via sync.Once; the first caller wins. Just assert it
	// runs and returns a stable value across calls.
	first := InContainer()
	if InContainer() != first {
		t.Fatal("InContainer() returned different values across calls")
	}
}

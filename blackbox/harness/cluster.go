package harness

import (
	"os"
	"path/filepath"
	"testing"
)

// Mode determines how the harness reaches the miren cluster.
type Mode string

const (
	// ModeDev routes commands through hack/dev-exec inside the iso container.
	ModeDev Mode = "dev"

	// ModeLocal calls the miren binary directly on the host.
	ModeLocal Mode = "local"

	// ModePeers routes commands through iso peers exec into the coordinator
	// container. Used for testing distributed runner topologies.
	ModePeers Mode = "peers"
)

// Cluster holds targeting information for a test run.
type Cluster struct {
	Mode        Mode
	RepoRoot    string
	TestdataDir string
	MirenBin    string // only used in local mode
}

// NewCluster reads environment variables and returns a configured Cluster.
// It fails the test if required configuration is missing.
func NewCluster(t *testing.T) *Cluster {
	t.Helper()

	mode := ModeDev
	if m := os.Getenv("BLACKBOX_MODE"); m != "" {
		switch Mode(m) {
		case ModeDev, ModeLocal, ModePeers:
			mode = Mode(m)
		default:
			t.Fatalf("invalid BLACKBOX_MODE %q (expected %q, %q, or %q)", m, ModeDev, ModeLocal, ModePeers)
		}
	}

	repoRoot := os.Getenv("BLACKBOX_REPO_ROOT")
	if repoRoot == "" {
		repoRoot = detectRepoRoot(t)
	}

	c := &Cluster{
		Mode:        mode,
		RepoRoot:    repoRoot,
		TestdataDir: filepath.Join(repoRoot, "testdata"),
	}

	if mode == ModeLocal {
		c.MirenBin = os.Getenv("BLACKBOX_MIREN_BIN")
		if c.MirenBin == "" {
			c.MirenBin = filepath.Join(repoRoot, "bin", "miren")
		}
	}

	return c
}

// IsPeers returns true when the cluster is running in distributed peers mode.
func (c *Cluster) IsPeers() bool {
	return c.Mode == ModePeers
}

// detectRepoRoot walks up from the working directory looking for the .iso
// directory that marks the repo root.
func detectRepoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, ".iso")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not detect repo root (no .iso directory found); set BLACKBOX_REPO_ROOT")
		}
		dir = parent
	}
}

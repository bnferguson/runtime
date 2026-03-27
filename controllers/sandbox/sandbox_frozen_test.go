package sandbox

import (
	"crypto/sha256"
	"fmt"
	"os"
	"testing"
)

// TestSandboxControllerFrozen guards against accidental modifications to the
// original sandbox controller files while the saga-based replacement is being
// developed.
//
// If this test fails, it means one of the frozen files was modified. Before
// updating the hash, please:
//
//  1. Audit the saga controller (saga_controller.go, create_saga.go) to ensure
//     it reflects the same behavioral change.
//  2. Consider whether the change can wait until we fully cut over to sagas,
//     which would avoid maintaining two code paths.
//
// To update hashes after an intentional change:
//
//	sha256sum controllers/sandbox/sandbox.go controllers/sandbox/volume.go controllers/sandbox/firewall.go
func TestSandboxControllerFrozen(t *testing.T) {
	frozen := map[string]string{
		"sandbox.go":  "15532b2466be230e3b450ed4602265bdc44db7c7985b4f7cfedb5aca76de8563",
		"volume.go":   "28e668016a394c28cb9b6b8264d6e4a3e8e6d4b14176e57c4d98b76d9255f6f5",
		"firewall.go": "802cb47113ab3c3710451ded4c203922d750d3ab42124d92d31f7c62acc2e73c",
	}

	for file, expectedHash := range frozen {
		t.Run(file, func(t *testing.T) {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("reading %s: %v", file, err)
			}

			actual := fmt.Sprintf("%x", sha256.Sum256(data))
			if actual != expectedHash {
				t.Fatalf(`%s has been modified (hash mismatch).

  expected: %s
  actual:   %s

This file is frozen while the saga-based sandbox controller is being developed.
Before updating the hash, please:

  1. Audit saga_controller.go and create_saga.go to ensure they reflect
     the same behavioral change you're making here.
  2. Consider holding off on this change until we fully switch over to sagas,
     so we don't have to maintain two code paths.

To update hashes:
  sha256sum controllers/sandbox/sandbox.go controllers/sandbox/volume.go controllers/sandbox/firewall.go`,
					file, expectedHash, actual)
			}
		})
	}
}

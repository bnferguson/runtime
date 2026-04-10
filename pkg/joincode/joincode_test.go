package joincode

import (
	"testing"
)

func TestHash(t *testing.T) {
	code := "abcdef1234567890"

	hash1 := Hash(code)
	hash2 := Hash(code)

	if hash1 != hash2 {
		t.Errorf("Hash should be deterministic: got %q and %q", hash1, hash2)
	}

	if len(hash1) != 64 {
		t.Errorf("Expected 64-char hex SHA-256 hash, got %d chars", len(hash1))
	}

	// Should normalize case and whitespace
	hash3 := Hash("  ABCDEF1234567890  ")
	if hash1 != hash3 {
		t.Errorf("Hash should normalize case and whitespace")
	}

	hash4 := Hash("different-secret")
	if hash1 == hash4 {
		t.Errorf("Different inputs should have different hashes")
	}
}

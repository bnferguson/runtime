package joincode

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// Hash returns the SHA-256 hex digest of the given secret, normalized
// to lowercase with whitespace trimmed. Used for storing and looking up
// invite secrets without persisting the secret itself.
func Hash(secret string) string {
	normalized := strings.ToLower(strings.TrimSpace(secret))
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

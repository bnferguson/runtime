package enrolltoken

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"miren.dev/runtime/pkg/joincode"
)

const (
	// Prefix is the string prefix for all enrollment tokens.
	Prefix = "mren_"

	// SecretBytes is the number of random bytes in an enrollment secret.
	SecretBytes = 32
)

var hexPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type tokenPayload struct {
	Addr   string `json:"a"`
	Secret string `json:"s"`
}

// GenerateSecret produces a cryptographically random hex-encoded secret
// suitable for use in an enrollment token.
func GenerateSecret() (string, error) {
	b := make([]byte, SecretBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating random secret: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// Encode produces an enrollment token string from a coordinator address
// and secret. The token has the form mren_<base64url(json)>.
func Encode(addr, secret string) string {
	payload := tokenPayload{Addr: addr, Secret: secret}
	data, _ := json.Marshal(payload)
	return Prefix + base64.RawURLEncoding.EncodeToString(data)
}

// Decode parses an enrollment token and returns the coordinator address
// and secret it contains.
func Decode(token string) (addr, secret string, err error) {
	if !IsToken(token) {
		return "", "", fmt.Errorf("not an enrollment token (missing %q prefix)", Prefix)
	}

	encoded := strings.TrimPrefix(token, Prefix)
	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", "", fmt.Errorf("decoding token: %w", err)
	}

	var payload tokenPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", "", fmt.Errorf("parsing token payload: %w", err)
	}

	if payload.Addr == "" {
		return "", "", fmt.Errorf("token missing coordinator address")
	}
	if payload.Secret == "" {
		return "", "", fmt.Errorf("token missing secret")
	}

	return payload.Addr, payload.Secret, nil
}

// IsToken reports whether s looks like an enrollment token.
func IsToken(s string) bool {
	return strings.HasPrefix(s, Prefix)
}

// IsHexSecret reports whether s is a valid hex-encoded enrollment secret
// (64 lowercase hex characters = 32 bytes).
func IsHexSecret(s string) bool {
	return hexPattern.MatchString(s)
}

// Hash returns the SHA-256 hash of a secret, using the same scheme as
// invite code hashing for consistency.
func Hash(secret string) string {
	return joincode.Hash(secret)
}

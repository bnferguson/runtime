package joincode

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"regexp"
	"strings"
)

const alphanumChars = "abcdefghjkmnpqrstuvwxyz23456789"

var codePattern = regexp.MustCompile(`^[a-z]+-[a-z]+-[a-z0-9]{4}$`)

func Generate() (string, error) {
	adjIdx, err := randomIndex(len(adjectives))
	if err != nil {
		return "", fmt.Errorf("selecting adjective: %w", err)
	}

	nounIdx, err := randomIndex(len(nouns))
	if err != nil {
		return "", fmt.Errorf("selecting noun: %w", err)
	}

	suffix, err := randomAlphanumeric(4)
	if err != nil {
		return "", fmt.Errorf("generating suffix: %w", err)
	}

	return fmt.Sprintf("%s-%s-%s", adjectives[adjIdx], nouns[nounIdx], suffix), nil
}

func Hash(code string) string {
	normalized := strings.ToLower(strings.TrimSpace(code))
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

func Validate(code string) bool {
	return codePattern.MatchString(strings.ToLower(strings.TrimSpace(code)))
}

func randomIndex(max int) (int, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}
	return int(n.Int64()), nil
}

func randomAlphanumeric(length int) (string, error) {
	result := make([]byte, length)
	max := big.NewInt(int64(len(alphanumChars)))
	for i := range result {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		result[i] = alphanumChars[n.Int64()]
	}
	return string(result), nil
}

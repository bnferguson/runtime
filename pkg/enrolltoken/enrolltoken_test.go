package enrolltoken

import (
	"strings"
	"testing"
)

func TestGenerateSecret(t *testing.T) {
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatal(err)
	}
	if len(secret) != 64 {
		t.Fatalf("expected 64-char hex string, got %d chars", len(secret))
	}
	if !IsHexSecret(secret) {
		t.Fatalf("generated secret is not valid hex: %q", secret)
	}

	// Two secrets should differ
	secret2, err := GenerateSecret()
	if err != nil {
		t.Fatal(err)
	}
	if secret == secret2 {
		t.Fatal("two generated secrets should not be equal")
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	addr := "coordinator.example.com:8443"
	secret := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"

	token := Encode(addr, secret)

	if !IsToken(token) {
		t.Fatalf("encoded token should start with prefix, got %q", token)
	}
	if !strings.HasPrefix(token, Prefix) {
		t.Fatalf("expected prefix %q, got %q", Prefix, token[:len(Prefix)])
	}

	gotAddr, gotSecret, err := Decode(token)
	if err != nil {
		t.Fatal(err)
	}
	if gotAddr != addr {
		t.Fatalf("addr: got %q, want %q", gotAddr, addr)
	}
	if gotSecret != secret {
		t.Fatalf("secret: got %q, want %q", gotSecret, secret)
	}
}

func TestDecodeErrors(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  string
	}{
		{"no prefix", "not_a_token", "missing"},
		{"bad base64", "mren_!!!invalid!!!", "decoding"},
		{"bad json", "mren_" + "bm90anNvbg", "parsing"},
		{"missing addr", "mren_" + "eyJzIjoiYWJjZCJ9", "missing coordinator"},
		{"missing secret", "mren_" + "eyJhIjoiaG9zdDo4NDQzIn0", "missing secret"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := Decode(tt.token)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error %q should contain %q", err, tt.want)
			}
		})
	}
}

func TestIsToken(t *testing.T) {
	if IsToken("not-a-token") {
		t.Fatal("should not match non-token")
	}
	if IsToken("") {
		t.Fatal("should not match empty string")
	}
	if !IsToken("mren_anything") {
		t.Fatal("should match mren_ prefix")
	}
}

func TestIsHexSecret(t *testing.T) {
	valid := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	if !IsHexSecret(valid) {
		t.Fatalf("should accept valid 64-char hex")
	}

	if IsHexSecret("short") {
		t.Fatal("should reject short strings")
	}
	if IsHexSecret("ABCDEF1234567890ABCDEF1234567890ABCDEF1234567890ABCDEF1234567890") {
		t.Fatal("should reject uppercase hex")
	}
	if IsHexSecret("zzzzzz1234567890abcdef1234567890abcdef1234567890abcdef1234567890") {
		t.Fatal("should reject non-hex chars")
	}
	if IsHexSecret("") {
		t.Fatal("should reject empty string")
	}
}

func TestHash(t *testing.T) {
	h1 := Hash("secret1")
	h2 := Hash("secret1")
	h3 := Hash("secret2")

	if h1 != h2 {
		t.Fatal("same input should produce same hash")
	}
	if h1 == h3 {
		t.Fatal("different inputs should produce different hashes")
	}
	if len(h1) != 64 {
		t.Fatalf("hash should be 64 hex chars, got %d", len(h1))
	}
}

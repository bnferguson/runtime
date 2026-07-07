//go:build linux

package commands

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"
)

// makeTestCertPEM mints a self-signed leaf certificate with the given CommonName
// and validity window, returning its PEM encoding. It lets the expiry/CN checks be
// tested against precise NotBefore/NotAfter values (caauth issues relative to now).
func makeTestCertPEM(t *testing.T, commonName string, notBefore, notAfter time.Time) string {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func TestCertPastRenewalThreshold(t *testing.T) {
	now := time.Now()
	year := 365 * 24 * time.Hour

	tests := []struct {
		name      string
		notBefore time.Time
		notAfter  time.Time
		want      bool
	}{
		{
			// Freshly issued: ~0% of lifetime elapsed.
			name:      "fresh cert not due",
			notBefore: now,
			notAfter:  now.Add(year),
			want:      false,
		},
		{
			// Just under two-thirds (200 of 365 days elapsed).
			name:      "past half but under threshold",
			notBefore: now.Add(-200 * 24 * time.Hour),
			notAfter:  now.Add(165 * 24 * time.Hour),
			want:      false,
		},
		{
			// Past two-thirds (300 of 365 days elapsed).
			name:      "past renewal threshold",
			notBefore: now.Add(-300 * 24 * time.Hour),
			notAfter:  now.Add(65 * 24 * time.Hour),
			want:      true,
		},
		{
			// Already expired is well past the threshold.
			name:      "expired",
			notBefore: now.Add(-2 * year),
			notAfter:  now.Add(-year),
			want:      true,
		},
		{
			// Degenerate zero-length validity is treated as not-due.
			name:      "zero lifetime",
			notBefore: now,
			notAfter:  now,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			certPEM := makeTestCertPEM(t, "runner-abc", tt.notBefore, tt.notAfter)
			got, err := certPastRenewalThreshold(certPEM)
			if err != nil {
				t.Fatalf("certPastRenewalThreshold returned error: %v", err)
			}
			if got != tt.want {
				t.Errorf("certPastRenewalThreshold = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCertPastRenewalThresholdInvalidPEM(t *testing.T) {
	if _, err := certPastRenewalThreshold("not a pem"); err == nil {
		t.Fatal("expected error for invalid PEM input")
	}
}

func TestCertExpired(t *testing.T) {
	now := time.Now()
	hour := time.Hour

	valid := makeTestCertPEM(t, "runner-abc", now.Add(-hour), now.Add(hour))
	if got, err := certExpired(valid); err != nil || got {
		t.Errorf("certExpired(valid) = %v (err %v), want false", got, err)
	}

	expired := makeTestCertPEM(t, "runner-abc", now.Add(-2*hour), now.Add(-hour))
	if got, err := certExpired(expired); err != nil || !got {
		t.Errorf("certExpired(expired) = %v (err %v), want true", got, err)
	}

	if _, err := certExpired("not a pem"); err == nil {
		t.Fatal("expected error for invalid PEM input")
	}
}

func TestCertCommonName(t *testing.T) {
	now := time.Now()
	certPEM := makeTestCertPEM(t, "runner-abcd1234-5678-90ab-cdef-1234567890ab", now, now.Add(time.Hour))

	cn, err := certCommonName(certPEM)
	if err != nil {
		t.Fatalf("certCommonName returned error: %v", err)
	}
	if want := "runner-abcd1234-5678-90ab-cdef-1234567890ab"; cn != want {
		t.Errorf("certCommonName = %q, want %q", cn, want)
	}
}

func TestCertCommonNameInvalidPEM(t *testing.T) {
	if _, err := certCommonName("not a pem"); err == nil {
		t.Fatal("expected error for invalid PEM input")
	}
}

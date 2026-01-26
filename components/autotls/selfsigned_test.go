package autotls

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadOrGenerateFallbackCert(t *testing.T) {
	t.Run("generates new cert when none exists", func(t *testing.T) {
		tmpDir := t.TempDir()

		cert, err := loadOrGenerateFallbackCert(tmpDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify cert is valid
		if len(cert.Certificate) == 0 {
			t.Fatal("expected cert to have certificate data")
		}

		// Verify files were created
		certPath := filepath.Join(tmpDir, fallbackCertFile)
		keyPath := filepath.Join(tmpDir, fallbackKeyFile)

		if _, err := os.Stat(certPath); os.IsNotExist(err) {
			t.Error("expected cert file to be created")
		}
		if _, err := os.Stat(keyPath); os.IsNotExist(err) {
			t.Error("expected key file to be created")
		}
	})

	t.Run("loads existing valid cert", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Generate initial cert
		cert1, err := loadOrGenerateFallbackCert(tmpDir)
		if err != nil {
			t.Fatalf("unexpected error generating cert: %v", err)
		}

		// Get the serial number of first cert
		x509Cert1, err := x509.ParseCertificate(cert1.Certificate[0])
		if err != nil {
			t.Fatalf("failed to parse first cert: %v", err)
		}

		// Load again - should return same cert
		cert2, err := loadOrGenerateFallbackCert(tmpDir)
		if err != nil {
			t.Fatalf("unexpected error loading cert: %v", err)
		}

		x509Cert2, err := x509.ParseCertificate(cert2.Certificate[0])
		if err != nil {
			t.Fatalf("failed to parse second cert: %v", err)
		}

		// Serial numbers should match (same cert loaded from disk)
		if x509Cert1.SerialNumber.Cmp(x509Cert2.SerialNumber) != 0 {
			t.Error("expected same cert to be loaded, but serial numbers differ")
		}
	})

	t.Run("creates certs directory if missing", func(t *testing.T) {
		tmpDir := t.TempDir()
		nestedDir := filepath.Join(tmpDir, "nested", "certs", "dir")

		_, err := loadOrGenerateFallbackCert(nestedDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, err := os.Stat(nestedDir); os.IsNotExist(err) {
			t.Error("expected nested directory to be created")
		}
	})
}

func TestCertExpiringSoon(t *testing.T) {
	t.Run("returns true for empty cert", func(t *testing.T) {
		var emptyCert tls.Certificate
		if !certExpiringSoon(emptyCert) {
			t.Error("expected empty cert to be considered expiring")
		}
	})

	t.Run("returns false for fresh cert", func(t *testing.T) {
		// Generate a fresh cert (valid for 1 year)
		cert, err := generateSelfSignedCert()
		if err != nil {
			t.Fatalf("failed to generate cert: %v", err)
		}

		if certExpiringSoon(cert) {
			t.Error("expected fresh cert (1 year validity) to not be expiring soon")
		}
	})

	t.Run("correctly checks expiry window", func(t *testing.T) {
		cert, err := generateSelfSignedCert()
		if err != nil {
			t.Fatalf("failed to generate cert: %v", err)
		}

		x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			t.Fatalf("failed to parse cert: %v", err)
		}

		// Cert should expire in ~365 days, which is > 30 day renewal window
		timeUntilExpiry := time.Until(x509Cert.NotAfter)
		if timeUntilExpiry < fallbackCertRenewalWindow {
			t.Errorf("expected cert to expire in > %v, but expires in %v",
				fallbackCertRenewalWindow, timeUntilExpiry)
		}
	})
}

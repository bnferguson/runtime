//go:build linux

package commands

import (
	"net"
	"testing"
	"time"

	"miren.dev/runtime/pkg/caauth"
)

func TestCertCoversListenAddr(t *testing.T) {
	ca, err := caauth.New(caauth.Options{
		CommonName:   "test-ca",
		Organization: "test",
		ValidFor:     24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("failed to create CA: %v", err)
	}

	cc, err := ca.IssueCertificate(caauth.Options{
		CommonName:   "runner-abc12345",
		Organization: "miren",
		ValidFor:     time.Hour,
		IPs:          []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("10.0.0.45")},
		DNSNames:     []string{"localhost", "runner.example.com"},
	})
	if err != nil {
		t.Fatalf("failed to issue cert: %v", err)
	}
	certPEM := string(cc.CertPEM)

	tests := []struct {
		name       string
		listenAddr string
		want       bool
	}{
		{"covered IP", "10.0.0.45:8444", true},
		{"loopback IP", "127.0.0.1:8444", true},
		{"changed IP not covered", "10.0.0.47:8444", false},
		{"covered DNS host", "runner.example.com:8444", true},
		{"uncovered DNS host", "other.example.com:8444", false},
		{"bare IP without port", "10.0.0.45", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := certCoversListenAddr(certPEM, tt.listenAddr)
			if err != nil {
				t.Fatalf("certCoversListenAddr returned error: %v", err)
			}
			if got != tt.want {
				t.Errorf("certCoversListenAddr(%q) = %v, want %v", tt.listenAddr, got, tt.want)
			}
		})
	}
}

func TestCertCoversListenAddrInvalidPEM(t *testing.T) {
	if _, err := certCoversListenAddr("not a pem", "10.0.0.1:8444"); err == nil {
		t.Fatal("expected error for invalid PEM input")
	}
}

package serverconfig

import (
	"strings"
	"testing"
)

func TestValidateIngressCoherence(t *testing.T) {
	type setup func(*Config)

	cases := []struct {
		name         string
		setup        setup
		wantContains string // empty means no error
	}{
		{
			name: "default config is valid",
			setup: func(c *Config) {
				c.Ingress.SetMode(IngressModeAutoprovision)
			},
		},
		{
			name: "tls-autoprovision rejects address override",
			setup: func(c *Config) {
				c.Ingress.SetMode(IngressModeAutoprovision)
				c.Ingress.SetAddress("0.0.0.0:443")
			},
			wantContains: "ingress.address must be empty",
		},
		{
			name: "behind-proxy-https accepts default (empty) address",
			setup: func(c *Config) {
				c.Ingress.SetMode(IngressModeBehindProxyHTTPS)
				c.TLS.SetSelfSigned(true)
			},
		},
		{
			name: "behind-proxy-https accepts custom address",
			setup: func(c *Config) {
				c.Ingress.SetMode(IngressModeBehindProxyHTTPS)
				c.Ingress.SetAddress("127.0.0.1:8443")
				c.TLS.SetSelfSigned(true)
			},
		},
		{
			name: "behind-proxy-http accepts plain address",
			setup: func(c *Config) {
				c.Ingress.SetMode(IngressModeBehindProxyHTTP)
				c.Ingress.SetAddress("0.0.0.0:80")
			},
		},
		{
			name: "behind-proxy-http rejects populated tls.self_signed",
			setup: func(c *Config) {
				c.Ingress.SetMode(IngressModeBehindProxyHTTP)
				c.TLS.SetSelfSigned(true)
			},
			wantContains: "tls.self_signed",
		},
		{
			name: "behind-proxy-http rejects populated tls.acme_email",
			setup: func(c *Config) {
				c.Ingress.SetMode(IngressModeBehindProxyHTTP)
				c.TLS.SetAcmeEmail("ops@example.com")
			},
			wantContains: "tls.acme_email",
		},
		{
			name: "behind-proxy-http reports all populated tls fields",
			setup: func(c *Config) {
				c.Ingress.SetMode(IngressModeBehindProxyHTTP)
				c.TLS.SetAcmeEmail("ops@example.com")
				c.TLS.SetAcmeDNSProvider("cloudflare")
			},
			wantContains: "tls.acme_email, tls.acme_dns_provider",
		},
		{
			// tls.additional_names / tls.additional_ips also feed the API
			// server cert (and the etcd cert), which exist regardless of
			// ingress.mode. So they must be allowed in behind-proxy-http.
			name: "behind-proxy-http accepts tls.additional_names",
			setup: func(c *Config) {
				c.Ingress.SetMode(IngressModeBehindProxyHTTP)
				c.TLS.AdditionalNames = []string{"miren.example.com"}
			},
		},
		{
			name: "behind-proxy-http accepts tls.additional_ips",
			setup: func(c *Config) {
				c.Ingress.SetMode(IngressModeBehindProxyHTTP)
				c.TLS.AdditionalIPs = []string{"203.0.113.5"}
			},
		},
		{
			name: "behind-proxy-http rejects ingress-only fields even when additional_* are also set",
			setup: func(c *Config) {
				c.Ingress.SetMode(IngressModeBehindProxyHTTP)
				c.TLS.SetAcmeEmail("ops@example.com")
				c.TLS.AdditionalNames = []string{"miren.example.com"}
			},
			wantContains: "tls.acme_email",
		},
		{
			name: "rejects unix: address with clear message",
			setup: func(c *Config) {
				c.Ingress.SetMode(IngressModeBehindProxyHTTP)
				c.Ingress.SetAddress("unix:/var/run/miren.sock")
			},
			wantContains: "unix socket binding",
		},
		{
			name: "rejects malformed address",
			setup: func(c *Config) {
				c.Ingress.SetMode(IngressModeBehindProxyHTTP)
				c.Ingress.SetAddress("not-a-real-address")
			},
			wantContains: "must be a host:port form",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tc.setup(cfg)

			err := cfg.ValidateIngressCoherence()
			switch {
			case tc.wantContains == "" && err != nil:
				t.Fatalf("unexpected error: %v", err)
			case tc.wantContains != "" && err == nil:
				t.Fatalf("expected error containing %q, got nil", tc.wantContains)
			case tc.wantContains != "" && !strings.Contains(err.Error(), tc.wantContains):
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantContains)
			}
		})
	}
}

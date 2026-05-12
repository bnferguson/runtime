package serverconfig

import (
	"testing"

	toml "github.com/pelletier/go-toml/v2"
)

// Verifies that unknown TOML fields don't blow up the parser. This matters for
// backwards compatibility with operators who still have `standard_tls = true`
// in their pre-RFD-84 server.toml files. If go-toml/v2's default is strict,
// we need a different shim than cli_only.
func TestUnknownTOMLFieldIsIgnored(t *testing.T) {
	data := []byte(`
[tls]
acme_email = "ops@example.com"
standard_tls = true
some_nonsense_field = "whatever"
`)
	var c Config
	if err := toml.Unmarshal(data, &c); err != nil {
		t.Fatalf("toml.Unmarshal failed on unknown field, default is strict: %v", err)
	}
	if got := c.TLS.GetAcmeEmail(); got != "ops@example.com" {
		t.Fatalf("AcmeEmail = %q, want ops@example.com", got)
	}
}

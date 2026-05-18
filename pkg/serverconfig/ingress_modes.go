package serverconfig

// Ingress mode values. Keep these in sync with the enum on IngressConfig.mode
// in schema.yml (the schema is the source of truth for valid values; these
// constants exist so callers can refer to modes by name in switch statements
// and validation code instead of bare strings).
const (
	IngressModeAutoprovision    = "tls-autoprovision"
	IngressModeBehindProxyHTTP  = "behind-proxy-http"
	IngressModeBehindProxyHTTPS = "behind-proxy-https"
)

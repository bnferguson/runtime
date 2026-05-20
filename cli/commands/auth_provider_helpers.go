package commands

import (
	"encoding/json"
	"fmt"
	"strings"

	"miren.dev/runtime/api/ingress/ingress_v1alpha"
	"miren.dev/runtime/pkg/connectors"
)

// isConnector reports whether an OIDC provider entity is a connector-backed
// provider (e.g. github) rather than a built-in OIDC discovery client.
// Empty connector_type is treated as the legacy OIDC default.
func isConnector(p *ingress_v1alpha.OidcProvider) bool {
	return p.ConnectorType != "" && p.ConnectorType != "oidc"
}

// summarizeConnectorConfig formats a connector config_json blob for table
// display. Renders as a short human string like "orgs: mirendev[eng,platform]".
func summarizeConnectorConfig(configJSON string) string {
	if configJSON == "" {
		return ""
	}
	var cfg struct {
		Orgs []connectors.GitHubOrg `json:"orgs,omitempty"`
	}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return "(invalid config)"
	}
	if len(cfg.Orgs) == 0 {
		return ""
	}
	parts := make([]string, 0, len(cfg.Orgs))
	for _, o := range cfg.Orgs {
		if len(o.Teams) == 0 {
			parts = append(parts, o.Name)
		} else {
			parts = append(parts, fmt.Sprintf("%s[%s]", o.Name, strings.Join(o.Teams, ",")))
		}
	}
	return "orgs: " + strings.Join(parts, ", ")
}

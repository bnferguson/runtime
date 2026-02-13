package postgresql

import (
	"strconv"
	"strings"

	"miren.dev/runtime/pkg/addon"
)

const (
	AddonName    = "miren-postgresql"
	DefaultImage = "docker.io/library/postgres:17"
)

// Variant configuration keys
const (
	ConfigStorage = "storage"
	ConfigShared  = "shared"
)

// Definition returns the addon definition for PostgreSQL.
func Definition() addon.AddonDefinition {
	return addon.AddonDefinition{
		Name:           AddonName,
		DisplayName:    "Miren PostgreSQL",
		Description:    "Managed PostgreSQL database",
		DefaultVariant: "small",
		Variants: []addon.VariantDefinition{
			{
				Name:        "small",
				Description: "Dedicated PostgreSQL server",
				Details: map[string]string{
					"Storage": "1 GB",
				},
				Config: map[string]string{
					ConfigStorage: "1Gi",
					ConfigShared:  "false",
				},
			},
			{
				Name:        "shared",
				Description: "Multi-app shared server",
				Details: map[string]string{
					"Type": "Shared server",
					"Note": "Multiple apps share one PostgreSQL instance",
				},
				Config: map[string]string{
					ConfigShared: "true",
				},
			},
		},
	}
}

const sharedDefaultStorageGb int64 = 10

// parseStorageGb converts a Kubernetes-style size string (e.g. "1Gi", "50Gi")
// to an int64 value in gigabytes. Returns 1 if the string cannot be parsed.
func parseStorageGb(s string) int64 {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "Gi") {
		n, err := strconv.ParseInt(strings.TrimSuffix(s, "Gi"), 10, 64)
		if err == nil && n > 0 {
			return n
		}
	}
	return 1
}

// IsSharedVariant returns true if the variant is a shared-server variant.
func IsSharedVariant(variantName string) bool {
	return variantName == "shared"
}

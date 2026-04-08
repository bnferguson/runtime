package postgresql

import (
	"miren.dev/runtime/pkg/addon"
)

const (
	AddonName      = "miren-postgresql"
	BaseImage      = "oci.miren.cloud/postgres"
	DefaultVersion = "17"
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
		BaseImage:      BaseImage,
		DefaultVersion: DefaultVersion,
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

// IsSharedVariant returns true if the variant is a shared-server variant.
func IsSharedVariant(variantName string) bool {
	return addon.IsSharedVariant(variantName)
}

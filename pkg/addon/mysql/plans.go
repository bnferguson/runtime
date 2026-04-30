package mysql

import (
	"miren.dev/runtime/pkg/addon"
)

const (
	AddonName      = "miren-mysql"
	BaseImage      = "oci.miren.cloud/mysql"
	DefaultVersion = "9"
)

const (
	ConfigStorage = "storage"
	ConfigShared  = "shared"
)

func Definition() addon.AddonDefinition {
	return addon.AddonDefinition{
		Name:           AddonName,
		DisplayName:    "Miren MySQL",
		Description:    "Managed MySQL database",
		DefaultVariant: "small",
		BaseImage:      BaseImage,
		DefaultVersion: DefaultVersion,
		Variants: []addon.VariantDefinition{
			{
				Name:        "small",
				Description: "Dedicated MySQL server",
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
					"Note": "Multiple apps share one MySQL instance",
				},
				Config: map[string]string{
					ConfigShared: "true",
				},
			},
		},
	}
}

const sharedDefaultStorageGb int64 = 10

func IsSharedVariant(variantName string) bool {
	return addon.IsSharedVariant(variantName)
}

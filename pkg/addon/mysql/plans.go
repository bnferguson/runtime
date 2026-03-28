package mysql

import (
	"strconv"
	"strings"

	"miren.dev/runtime/pkg/addon"
)

const (
	AddonName    = "miren-mysql"
	DefaultImage = "docker.io/library/mysql:8"
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

func IsSharedVariant(variantName string) bool {
	return variantName == "shared"
}

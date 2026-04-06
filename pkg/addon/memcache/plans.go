package memcache

import (
	"miren.dev/runtime/pkg/addon"
)

const (
	AddonName    = "miren-memcache"
	DefaultImage = "docker.io/library/memcached:1.6"
)

const (
	ConfigMemory = "memory"
)

func Definition() addon.AddonDefinition {
	return addon.AddonDefinition{
		Name:           AddonName,
		DisplayName:    "Miren Memcache",
		Description:    "Managed Memcached in-memory cache",
		DefaultVariant: "small",
		Variants: []addon.VariantDefinition{
			{
				Name:        "small",
				Description: "Dedicated Memcached server",
				Details: map[string]string{
					"Memory": "64 MB",
				},
				Config: map[string]string{
					ConfigMemory: "64",
				},
			},
		},
	}
}

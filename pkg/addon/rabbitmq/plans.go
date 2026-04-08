package rabbitmq

import (
	"miren.dev/runtime/pkg/addon"
)

const (
	AddonName      = "miren-rabbitmq"
	BaseImage      = "oci.miren.cloud/rabbitmq"
	DefaultVersion = "4"
)

const (
	ConfigStorage = "storage"
)

func Definition() addon.AddonDefinition {
	return addon.AddonDefinition{
		Name:           AddonName,
		DisplayName:    "Miren RabbitMQ",
		Description:    "Managed RabbitMQ message broker",
		DefaultVariant: "small",
		BaseImage:      BaseImage,
		DefaultVersion: DefaultVersion,
		Variants: []addon.VariantDefinition{
			{
				Name:        "small",
				Description: "Dedicated RabbitMQ server",
				Details: map[string]string{
					"Storage": "1 GB",
				},
				Config: map[string]string{
					ConfigStorage: "1Gi",
				},
			},
		},
	}
}

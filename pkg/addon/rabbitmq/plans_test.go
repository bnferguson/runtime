package rabbitmq

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefinitionHasAllVariants(t *testing.T) {
	def := Definition()

	assert.Equal(t, AddonName, def.Name)
	assert.Equal(t, "Miren RabbitMQ", def.DisplayName)
	assert.Equal(t, "small", def.DefaultVariant)
	assert.Len(t, def.Variants, 1)
	assert.Equal(t, "small", def.Variants[0].Name)
	assert.Equal(t, "1Gi", def.Variants[0].Config[ConfigStorage])
}

func TestBuildEnvVars(t *testing.T) {
	vars := buildEnvVars("miren", "secret", "myhost", 5672, "miren")

	assert.Len(t, vars, 6)

	varMap := make(map[string]string)
	sensitiveMap := make(map[string]bool)
	for _, v := range vars {
		varMap[v.Key] = v.Value
		sensitiveMap[v.Key] = v.Sensitive
	}

	assert.Equal(t, "amqp://miren:secret@myhost:5672/miren", varMap["RABBITMQ_URL"])
	assert.True(t, sensitiveMap["RABBITMQ_URL"])
	assert.Equal(t, "myhost", varMap["RABBITMQ_HOST"])
	assert.False(t, sensitiveMap["RABBITMQ_HOST"])
	assert.Equal(t, "5672", varMap["RABBITMQ_PORT"])
	assert.False(t, sensitiveMap["RABBITMQ_PORT"])
	assert.Equal(t, "miren", varMap["RABBITMQ_USER"])
	assert.False(t, sensitiveMap["RABBITMQ_USER"])
	assert.Equal(t, "secret", varMap["RABBITMQ_PASSWORD"])
	assert.True(t, sensitiveMap["RABBITMQ_PASSWORD"])
	assert.Equal(t, "miren", varMap["RABBITMQ_VHOST"])
	assert.False(t, sensitiveMap["RABBITMQ_VHOST"])
}

func TestBuildRabbitmqURL(t *testing.T) {
	t.Run("simple vhost", func(t *testing.T) {
		u := buildRabbitmqURL("user", "pass", "host.example.com", 5672, "myapp")
		assert.Equal(t, "amqp://user:pass@host.example.com:5672/myapp", u)
	})

	t.Run("root vhost is percent-encoded", func(t *testing.T) {
		u := buildRabbitmqURL("user", "pass", "host.example.com", 5672, "/")
		assert.Equal(t, "amqp://user:pass@host.example.com:5672/%2F", u)
	})

	t.Run("vhost with slash", func(t *testing.T) {
		u := buildRabbitmqURL("user", "pass", "host.example.com", 5672, "my/app")
		assert.Equal(t, "amqp://user:pass@host.example.com:5672/my%2Fapp", u)
	})
}

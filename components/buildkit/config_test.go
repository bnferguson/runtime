package buildkit

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateConfig(t *testing.T) {
	c := &Component{}
	config := c.generateConfig(10*1024*1024*1024, 86400, "registry.example.com:5000")

	t.Run("has single catch-all gcpolicy", func(t *testing.T) {
		r := require.New(t)

		count := strings.Count(config, "[[worker.oci.gcpolicy]]")
		r.Equal(1, count, "should have exactly one gcpolicy block")
	})

	t.Run("policy has no filters (catch-all)", func(t *testing.T) {
		r := require.New(t)

		idx := strings.Index(config, "[[worker.oci.gcpolicy]]")
		r.GreaterOrEqual(idx, 0, "gcpolicy block should be present in config")

		block := config[idx:]
		// Extract just the policy block (up to next section)
		nextSection := strings.Index(block, "\n[registry")
		if nextSection > 0 {
			block = block[:nextSection]
		}

		r.NotContains(block, "filters")
		r.Contains(block, "keepBytes")
		r.Contains(block, "keepDuration")
	})

	t.Run("uses provided registry host", func(t *testing.T) {
		r := require.New(t)
		r.Contains(config, `[registry."registry.example.com:5000"]`)
	})

	t.Run("uses default registry host when empty", func(t *testing.T) {
		r := require.New(t)
		defaultConfig := c.generateConfig(10*1024*1024*1024, 86400, "")
		r.Contains(defaultConfig, `[registry."cluster.local:5000"]`)
	})
}

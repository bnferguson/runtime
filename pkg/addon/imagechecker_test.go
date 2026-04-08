package addon

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistryImageChecker(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	checker := NewRegistryImageChecker()
	ctx := context.Background()

	t.Run("docker.io image exists", func(t *testing.T) {
		err := checker.CheckImage(ctx, "docker.io/library/postgres:17")
		require.NoError(t, err)
	})

	t.Run("docker.io image does not exist", func(t *testing.T) {
		err := checker.CheckImage(ctx, "docker.io/library/postgres:99999-nonexistent")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not accessible")
	})

	t.Run("oci.miren.cloud image exists", func(t *testing.T) {
		err := checker.CheckImage(ctx, "oci.miren.cloud/postgres:17")
		require.NoError(t, err)
	})

	t.Run("oci.miren.cloud image does not exist", func(t *testing.T) {
		err := checker.CheckImage(ctx, "oci.miren.cloud/postgres:99999-nonexistent")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not accessible")
	})
}

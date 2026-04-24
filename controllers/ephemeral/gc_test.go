package ephemeral

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	compute_v1alpha "miren.dev/runtime/api/compute/compute_v1alpha"
	"miren.dev/runtime/api/core/core_v1alpha"
	"miren.dev/runtime/pkg/entity"
	testutils "miren.dev/runtime/pkg/entity/testutils"
	ephemeralx "miren.dev/runtime/pkg/ephemeral"
)

func TestGCController_RunGC(t *testing.T) {
	ctx := context.Background()
	inmem, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()
	log := testutils.TestLogger(t)

	app := &core_v1alpha.App{}
	appID, err := inmem.Client.Create(ctx, "gcapp", app)
	require.NoError(t, err)

	t.Run("deletes expired ephemeral versions", func(t *testing.T) {
		// Create an expired ephemeral version
		expired := &core_v1alpha.AppVersion{
			App:                appID,
			Version:            "gcapp-expired",
			EphemeralLabel:     "old-pr",
			EphemeralTtl:       "1h",
			EphemeralExpiresAt: time.Now().Add(-30 * time.Minute),
		}
		_, err := inmem.Client.Create(ctx, "gcapp-expired", expired)
		require.NoError(t, err)

		// Create a valid (non-expired) ephemeral version
		valid := &core_v1alpha.AppVersion{
			App:                appID,
			Version:            "gcapp-valid",
			EphemeralLabel:     "active-pr",
			EphemeralTtl:       "48h",
			EphemeralExpiresAt: time.Now().Add(47 * time.Hour),
		}
		_, err = inmem.Client.Create(ctx, "gcapp-valid", valid)
		require.NoError(t, err)

		// Create a non-ephemeral version (should not be touched)
		normal := &core_v1alpha.AppVersion{
			App:     appID,
			Version: "gcapp-normal",
		}
		_, err = inmem.Client.Create(ctx, "gcapp-normal", normal)
		require.NoError(t, err)

		gc := &GCController{
			Log:    log,
			EAC:    inmem.EAC,
			Config: DefaultGCConfig(),
		}

		result, err := gc.RunGC(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, result.DeletedVersions)
		require.Equal(t, 0, result.FailedVersions)

		// Expired version should be gone
		found, err := ephemeralx.LookupByLabel(ctx, inmem.EAC, appID, "old-pr")
		require.NoError(t, err)
		require.Nil(t, found, "expired version should have been deleted")

		// Valid ephemeral version should remain
		found, err = ephemeralx.LookupByLabel(ctx, inmem.EAC, appID, "active-pr")
		require.NoError(t, err)
		require.NotNil(t, found, "non-expired version should remain")

		// Non-ephemeral version should remain
		var normalCheck core_v1alpha.AppVersion
		err = inmem.Client.Get(ctx, "gcapp-normal", &normalCheck)
		require.NoError(t, err)
		require.Equal(t, "gcapp-normal", normalCheck.Version)
	})

	t.Run("no-op when no expired versions", func(t *testing.T) {
		gc := &GCController{
			Log:    log,
			EAC:    inmem.EAC,
			Config: DefaultGCConfig(),
		}

		result, err := gc.RunGC(ctx)
		require.NoError(t, err)
		require.Equal(t, 0, result.DeletedVersions)
	})
}

func TestGCController_RunGCByApps(t *testing.T) {
	ctx := context.Background()
	inmem, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()
	log := testutils.TestLogger(t)

	// Create two apps, each with an expired ephemeral version
	app1 := &core_v1alpha.App{}
	app1ID, err := inmem.Client.Create(ctx, "app1", app1)
	require.NoError(t, err)

	app2 := &core_v1alpha.App{}
	app2ID, err := inmem.Client.Create(ctx, "app2", app2)
	require.NoError(t, err)

	expired1 := &core_v1alpha.AppVersion{
		App:                app1ID,
		Version:            "app1-expired",
		EphemeralLabel:     "old-1",
		EphemeralTtl:       "1h",
		EphemeralExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	_, err = inmem.Client.Create(ctx, "app1-expired", expired1)
	require.NoError(t, err)

	expired2 := &core_v1alpha.AppVersion{
		App:                app2ID,
		Version:            "app2-expired",
		EphemeralLabel:     "old-2",
		EphemeralTtl:       "2h",
		EphemeralExpiresAt: time.Now().Add(-30 * time.Minute),
	}
	_, err = inmem.Client.Create(ctx, "app2-expired", expired2)
	require.NoError(t, err)

	gc := &GCController{
		Log:    log,
		EAC:    inmem.EAC,
		Config: DefaultGCConfig(),
	}

	// runGCByApps is the fallback path
	result, err := gc.runGCByApps(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, result.DeletedVersions)

	// Both expired versions should be gone
	found, err := ephemeralx.LookupByLabel(ctx, inmem.EAC, app1ID, "old-1")
	require.NoError(t, err)
	require.Nil(t, found)

	found, err = ephemeralx.LookupByLabel(ctx, inmem.EAC, app2ID, "old-2")
	require.NoError(t, err)
	require.Nil(t, found)
}

func TestGCController_StartStop(t *testing.T) {
	inmem, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()
	log := testutils.TestLogger(t)

	gc := &GCController{
		Log: log,
		EAC: inmem.EAC,
		Config: GCConfig{
			CheckInterval: 1 * time.Hour, // long interval so it doesn't fire during test
		},
	}

	ctx := context.Background()
	gc.Start(ctx)
	require.NotNil(t, gc.cancel)

	gc.Stop()
	// Verify stop is idempotent
	gc.Stop()
}

func TestGCController_RunGCWithLogging(t *testing.T) {
	ctx := context.Background()
	inmem, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()
	log := testutils.TestDebugLogger(t)

	gc := &GCController{
		Log:    log,
		EAC:    inmem.EAC,
		Config: DefaultGCConfig(),
	}

	// Should not panic with no data
	gc.runGCWithLogging(ctx)
}

func TestGCController_DeletesAssociatedPools(t *testing.T) {
	ctx := context.Background()
	inmem, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()
	log := testutils.TestLogger(t)

	app := &core_v1alpha.App{}
	appID, err := inmem.Client.Create(ctx, "poolapp", app)
	require.NoError(t, err)

	// Create expired ephemeral version
	expired := &core_v1alpha.AppVersion{
		App:                appID,
		Version:            "poolapp-expired",
		EphemeralLabel:     "old-branch",
		EphemeralTtl:       "1h",
		EphemeralExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	versionID, err := inmem.Client.Create(ctx, "poolapp-expired", expired)
	require.NoError(t, err)

	// Create a sandbox pool referencing this version
	pool := &compute_v1alpha.SandboxPool{
		App:                  appID,
		Service:              "web",
		ReferencedByVersions: []entity.Id{versionID},
	}
	poolID, err := inmem.Client.Create(ctx, "poolapp-web-pool", pool)
	require.NoError(t, err)

	gc := &GCController{
		Log:    log,
		EAC:    inmem.EAC,
		Config: DefaultGCConfig(),
	}

	result, err := gc.RunGC(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, result.DeletedVersions)

	// Version should be gone
	found, err := ephemeralx.LookupByLabel(ctx, inmem.EAC, appID, "old-branch")
	require.NoError(t, err)
	require.Nil(t, found)

	// Pool should also be gone
	_, getErr := inmem.EAC.Get(ctx, string(poolID))
	require.Error(t, getErr, "sandbox pool should have been deleted")
}

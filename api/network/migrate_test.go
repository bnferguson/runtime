package network

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"miren.dev/runtime/api/network/network_v1alpha"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/entity/testutils"
)

func TestMigrateServiceToTarget(t *testing.T) {
	server, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	ctx := context.Background()
	log := testutils.TestLogger(t)
	eac := server.EAC

	// Create a service entity with old-style attribute IDs directly in the store
	svcID := entity.Id("target/test-svc-1")
	svcEnt := entity.New(
		entity.DBId, svcID,
		entity.Ref(entity.EntityKind, oldKindService),
		entity.String(oldServiceIp, "10.10.0.5"),
		entity.String(oldServiceMatch, "app=web"),
	)
	server.AddEntity(svcEnt)

	// Create an endpoints entity with old endpoints.service ref
	epID := entity.Id("endpoints/test-ep-1")
	epEnt := entity.New(
		entity.DBId, epID,
		entity.Ref(entity.EntityKind, network_v1alpha.KindEndpoints),
		entity.Ref(oldEndpointsService, svcID),
	)
	server.AddEntity(epEnt)

	// Run migration
	err := MigrateServiceToTarget(ctx, log, eac)
	require.NoError(t, err)

	// Verify service entity was migrated to target
	svcRes, err := eac.Get(ctx, string(svcID))
	require.NoError(t, err)

	migratedSvc := svcRes.Entity().Entity()

	// EntityKind should now be KindTarget
	kindAttr, ok := migratedSvc.Get(entity.EntityKind)
	require.True(t, ok, "migrated entity should have EntityKind")
	assert.Equal(t, network_v1alpha.KindTarget, kindAttr.Value.Id())

	// IP should use target.ip ID
	ipAttr, ok := migratedSvc.Get(network_v1alpha.TargetIpId)
	require.True(t, ok, "migrated entity should have target.ip attr")
	assert.Equal(t, "10.10.0.5", ipAttr.Value.String())

	// Match should use target.match ID
	matchAttr, ok := migratedSvc.Get(network_v1alpha.TargetMatchId)
	require.True(t, ok, "migrated entity should have target.match attr")
	assert.Equal(t, "app=web", matchAttr.Value.String())

	// Old IDs should no longer be present
	_, ok = migratedSvc.Get(oldServiceIp)
	assert.False(t, ok, "old service.ip should not exist")
	_, ok = migratedSvc.Get(oldServiceMatch)
	assert.False(t, ok, "old service.match should not exist")

	// Verify endpoints entity was migrated
	epRes, err := eac.Get(ctx, string(epID))
	require.NoError(t, err)

	migratedEp := epRes.Entity().Entity()

	// Should now have endpoints.target
	targetRef, ok := migratedEp.Get(network_v1alpha.EndpointsTargetId)
	require.True(t, ok, "migrated endpoints should have endpoints.target attr")
	assert.Equal(t, svcID, targetRef.Value.Id())

	// Old endpoints.service should not exist
	_, ok = migratedEp.Get(oldEndpointsService)
	assert.False(t, ok, "old endpoints.service should not exist")
}

func TestMigrateServiceToTarget_WithPort(t *testing.T) {
	server, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	ctx := context.Background()
	log := testutils.TestLogger(t)
	eac := server.EAC

	// Create a service entity with a port component using old attribute IDs.
	// Ports are stored as component attrs whose children use port.* IDs.
	svcID := entity.Id("target/test-svc-port")
	portAttrs := []entity.Attr{
		entity.String(network_v1alpha.PortNameId, "http"),
		entity.Int64(network_v1alpha.PortPortId, 80),
	}
	svcEnt := entity.New(
		entity.DBId, svcID,
		entity.Ref(entity.EntityKind, oldKindService),
		entity.String(oldServiceIp, "10.10.0.10"),
		entity.Component(oldServicePort, portAttrs),
	)
	server.AddEntity(svcEnt)

	err := MigrateServiceToTarget(ctx, log, eac)
	require.NoError(t, err)

	svcRes, err := eac.Get(ctx, string(svcID))
	require.NoError(t, err)

	migrated := svcRes.Entity().Entity()

	// Decode as a Target and verify the port is fully populated
	var target network_v1alpha.Target
	target.Decode(migrated)

	require.Len(t, target.Port, 1, "migrated target should have one port")
	assert.Equal(t, "http", target.Port[0].Name)
	assert.Equal(t, int64(80), target.Port[0].Port)

	// Old service.port ID should not exist
	_, ok := migrated.Get(oldServicePort)
	assert.False(t, ok, "old service.port should not exist")
}

func TestMigrateServiceToTarget_AlreadyMigrated(t *testing.T) {
	server, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	ctx := context.Background()
	log := testutils.TestLogger(t)
	eac := server.EAC

	// Create an entity that already uses the new target IDs
	targetID := entity.Id("target/already-migrated")
	targetEnt := entity.New(
		entity.DBId, targetID,
		entity.Ref(entity.EntityKind, network_v1alpha.KindTarget),
		entity.String(network_v1alpha.TargetIpId, "10.10.0.20"),
	)
	server.AddEntity(targetEnt)

	// Create an endpoints entity already using endpoints.target
	epID := entity.Id("endpoints/already-migrated")
	epEnt := entity.New(
		entity.DBId, epID,
		entity.Ref(entity.EntityKind, network_v1alpha.KindEndpoints),
		entity.Ref(network_v1alpha.EndpointsTargetId, targetID),
	)
	server.AddEntity(epEnt)

	// Running migration should succeed without changing anything
	err := MigrateServiceToTarget(ctx, log, eac)
	require.NoError(t, err)

	// Verify target entity is unchanged
	res, err := eac.Get(ctx, string(targetID))
	require.NoError(t, err)

	ent := res.Entity().Entity()
	ipAttr, ok := ent.Get(network_v1alpha.TargetIpId)
	require.True(t, ok)
	assert.Equal(t, "10.10.0.20", ipAttr.Value.String())

	// Verify endpoints entity is unchanged
	epRes, err := eac.Get(ctx, string(epID))
	require.NoError(t, err)

	epEnt2 := epRes.Entity().Entity()
	ref, ok := epEnt2.Get(network_v1alpha.EndpointsTargetId)
	require.True(t, ok)
	assert.Equal(t, targetID, ref.Value.Id())

	// Old attr should not appear
	_, ok = epEnt2.Get(oldEndpointsService)
	assert.False(t, ok)
}

func TestMigrateServiceToTarget_NoEntities(t *testing.T) {
	server, cleanup := testutils.NewInMemEntityServer(t)
	defer cleanup()

	ctx := context.Background()
	log := testutils.TestLogger(t)

	// Running migration with no entities should succeed
	err := MigrateServiceToTarget(ctx, log, server.EAC)
	require.NoError(t, err)
}

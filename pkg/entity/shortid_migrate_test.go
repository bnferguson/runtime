package entity

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestMigrateShortIds(t *testing.T) {
	r := require.New(t)

	client, basePrefix := setupTestEtcd(t)
	ctx := context.Background()

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	prefix := basePrefix + "/migrate-shortids/"

	// Create entity with entity/kind but no short-id (should get one)
	ent1 := New(
		Ref(DBId, "deployment-CZ1eUgSgNd28ed6vt2DgY"),
		Ref(EntityKind, "core.deployment"),
		String("deployment/name", "web"),
	)
	data1, err := Encode(ent1)
	r.NoError(err)
	_, err = client.Put(ctx, prefix+"entity/deployment-CZ1eUgSgNd28ed6vt2DgY", string(data1))
	r.NoError(err)

	// Create entity that already has a short-id (should be skipped)
	ent2 := New(
		Ref(DBId, "sandbox/blog-web-CZAtBvhsMNbG38MceikkB"),
		Ref(EntityKind, "compute.sandbox"),
		String(DBShortId, "kkB"),
	)
	data2, err := Encode(ent2)
	r.NoError(err)
	_, err = client.Put(ctx, prefix+"entity/sandbox-blog-web-CZAtBvhsMNbG38MceikkB", string(data2))
	r.NoError(err)

	// Create a system entity with no entity/kind (should be skipped)
	ent3 := New(
		Ref(DBId, "db/type.str"),
		String(Doc, "String type"),
	)
	data3, err := Encode(ent3)
	r.NoError(err)
	_, err = client.Put(ctx, prefix+"entity/db-type-str", string(data3))
	r.NoError(err)

	// Run migration
	migrated, skipped, err := MigrateShortIds(ctx, log, client, MigrateShortIdOptions{
		Prefix: prefix,
		DryRun: false,
	})
	r.NoError(err)
	r.Equal(1, migrated, "should migrate 1 entity")
	r.Equal(2, skipped, "should skip 2 entities (one with short-id, one system)")

	// Verify the migrated entity got a short-id
	resp, err := client.Get(ctx, prefix+"entity/deployment-CZ1eUgSgNd28ed6vt2DgY")
	r.NoError(err)
	r.Len(resp.Kvs, 1)

	var migratedEnt Entity
	err = Decode(resp.Kvs[0].Value, &migratedEnt)
	r.NoError(err)

	shortId := migratedEnt.ShortId()
	r.NotEmpty(shortId, "migrated entity should have a short-id")
	// Should be derived from the base58 suffix "CZ1eUgSgNd28ed6vt2DgY" → last 3 chars "DgY"
	r.Equal("DgY", shortId)

	// Verify the unique key was created
	shortIdAttr := String(DBShortId, shortId)
	uniqueKey := prefix + "unique/" + shortIdAttr.CAS()
	uniqueResp, err := client.Get(ctx, uniqueKey)
	r.NoError(err)
	r.Len(uniqueResp.Kvs, 1)
	r.Equal("deployment-CZ1eUgSgNd28ed6vt2DgY", string(uniqueResp.Kvs[0].Value))

	// Run migration again — should be idempotent
	migrated2, skipped2, err := MigrateShortIds(ctx, log, client, MigrateShortIdOptions{
		Prefix: prefix,
		DryRun: false,
	})
	r.NoError(err)
	r.Equal(0, migrated2, "second run should migrate 0 entities")
	r.Equal(3, skipped2, "second run should skip all 3 entities")
}

func TestMigrateShortIdsDryRun(t *testing.T) {
	r := require.New(t)

	client, basePrefix := setupTestEtcd(t)
	ctx := context.Background()

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	prefix := basePrefix + "/migrate-shortids-dry/"

	// Create entity needing a short-id
	ent := New(
		Ref(DBId, "deployment-CZ1eUgSgNd28ed6vt2DgY"),
		Ref(EntityKind, "core.deployment"),
	)
	data, err := Encode(ent)
	r.NoError(err)
	_, err = client.Put(ctx, prefix+"entity/deployment-CZ1eUgSgNd28ed6vt2DgY", string(data))
	r.NoError(err)

	// Dry run should report migration needed but not write
	migrated, _, err := MigrateShortIds(ctx, log, client, MigrateShortIdOptions{
		Prefix: prefix,
		DryRun: true,
	})
	r.NoError(err)
	r.Equal(1, migrated)

	// Verify entity was NOT modified
	resp, err := client.Get(ctx, prefix+"entity/deployment-CZ1eUgSgNd28ed6vt2DgY")
	r.NoError(err)
	r.Len(resp.Kvs, 1)

	var unchangedEnt Entity
	err = Decode(resp.Kvs[0].Value, &unchangedEnt)
	r.NoError(err)
	r.Empty(unchangedEnt.ShortId(), "dry run should not write short-id")

	// Verify no unique keys were created
	uniqueResp, err := client.Get(ctx, prefix+"unique/", clientv3.WithPrefix(), clientv3.WithCountOnly())
	r.NoError(err)
	r.Equal(int64(0), uniqueResp.Count, "dry run should not create unique entries")
}

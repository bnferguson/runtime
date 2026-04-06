package entity

import (
	"log/slog"
	"testing"

	"github.com/mr-tron/base58"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clientv3 "go.etcd.io/etcd/client/v3"
	"miren.dev/runtime/pkg/etcdtest"
)

func setupReindexTestStore(t *testing.T) (*EtcdStore, *clientv3.Client) {
	t.Helper()
	client, prefix := etcdtest.TestEtcdClient(t)
	store, err := NewEtcdStore(t.Context(), slog.Default(), client, prefix)
	require.NoError(t, err)
	return store, client
}

func TestReindex_BasicIndexing(t *testing.T) {
	store, client := setupReindexTestStore(t)
	ctx := t.Context()

	// Create an indexed attribute schema
	_, err := store.CreateEntity(ctx, New(
		Ident, "test/kind",
		Doc, "a kind",
		Cardinality, CardinalityOne,
		Type, TypeStr,
		Index, true,
	))
	require.NoError(t, err)

	// Create entities with the indexed attribute
	e1, err := store.CreateEntity(ctx, New(
		Ident, "entity-1",
		String(Id("test/kind"), "widget"),
	))
	require.NoError(t, err)

	e2, err := store.CreateEntity(ctx, New(
		Ident, "entity-2",
		String(Id("test/kind"), "widget"),
	))
	require.NoError(t, err)

	// Verify indexes exist via ListIndex
	ids, err := store.ListIndex(ctx, String(Id("test/kind"), "widget"))
	require.NoError(t, err)
	assert.Len(t, ids, 2)

	// Now manually delete the collection entries to simulate missing indexes
	collectionPrefix := store.Prefix() + "/collections/"
	_, err = client.Delete(ctx, collectionPrefix, clientv3.WithPrefix())
	require.NoError(t, err)

	// Verify indexes are now gone
	ids, err = store.ListIndex(ctx, String(Id("test/kind"), "widget"))
	require.NoError(t, err)
	assert.Len(t, ids, 0)

	// Run reindex
	stats, err := store.Reindex(ctx, slog.Default(), ReindexOptions{
		DryRun:       false,
		CleanupStale: false,
	})
	require.NoError(t, err)
	assert.Greater(t, stats.EntitiesProcessed, int64(0))
	assert.Greater(t, stats.IndexesRebuilt, int64(0))

	// Verify indexes are back
	ids, err = store.ListIndex(ctx, String(Id("test/kind"), "widget"))
	require.NoError(t, err)
	assert.Len(t, ids, 2)

	foundIDs := map[Id]bool{ids[0]: true, ids[1]: true}
	assert.True(t, foundIDs[e1.Id()])
	assert.True(t, foundIDs[e2.Id()])
}

func TestReindex_Idempotent(t *testing.T) {
	store, _ := setupReindexTestStore(t)
	ctx := t.Context()

	// Create an indexed attribute schema
	_, err := store.CreateEntity(ctx, New(
		Ident, "test/kind",
		Doc, "a kind",
		Cardinality, CardinalityOne,
		Type, TypeStr,
		Index, true,
	))
	require.NoError(t, err)

	// Create entities
	_, err = store.CreateEntity(ctx, New(
		Ident, "entity-1",
		String(Id("test/kind"), "widget"),
	))
	require.NoError(t, err)

	// Run reindex twice
	stats1, err := store.Reindex(ctx, slog.Default(), ReindexOptions{
		DryRun:       false,
		CleanupStale: false,
	})
	require.NoError(t, err)

	stats2, err := store.Reindex(ctx, slog.Default(), ReindexOptions{
		DryRun:       false,
		CleanupStale: false,
	})
	require.NoError(t, err)

	// Same entities processed both times
	assert.Equal(t, stats1.EntitiesProcessed, stats2.EntitiesProcessed)

	// Verify indexes still work correctly
	ids, err := store.ListIndex(ctx, String(Id("test/kind"), "widget"))
	require.NoError(t, err)
	assert.Len(t, ids, 1)
}

func TestReindex_StaleCleanup(t *testing.T) {
	store, client := setupReindexTestStore(t)
	ctx := t.Context()

	// Create an indexed attribute schema
	_, err := store.CreateEntity(ctx, New(
		Ident, "test/kind",
		Doc, "a kind",
		Cardinality, CardinalityOne,
		Type, TypeStr,
		Index, true,
	))
	require.NoError(t, err)

	// Create an entity
	e1, err := store.CreateEntity(ctx, New(
		Ident, "entity-1",
		String(Id("test/kind"), "widget"),
	))
	require.NoError(t, err)

	// Manually insert a stale collection entry pointing to a non-existent entity
	fakeEntityID := Id("fake/nonexistent")
	fakeKey := base58.Encode([]byte(fakeEntityID))
	staleKey := store.Prefix() + "/collections/test_kind_widget/" + fakeKey
	_, err = client.Put(ctx, staleKey, string(fakeEntityID))
	require.NoError(t, err)

	// Run reindex with stale cleanup
	stats, err := store.Reindex(ctx, slog.Default(), ReindexOptions{
		DryRun:       false,
		CleanupStale: true,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), stats.StaleEntriesFound)
	assert.Equal(t, int64(1), stats.StaleEntriesRemoved)

	// Verify only the real entity is in the index
	ids, err := store.ListIndex(ctx, String(Id("test/kind"), "widget"))
	require.NoError(t, err)
	assert.Len(t, ids, 1)
	assert.Equal(t, e1.Id(), ids[0])
}

func TestReindex_DryRun(t *testing.T) {
	store, client := setupReindexTestStore(t)
	ctx := t.Context()

	// Create an indexed attribute schema
	_, err := store.CreateEntity(ctx, New(
		Ident, "test/kind",
		Doc, "a kind",
		Cardinality, CardinalityOne,
		Type, TypeStr,
		Index, true,
	))
	require.NoError(t, err)

	// Create entities
	_, err = store.CreateEntity(ctx, New(
		Ident, "entity-1",
		String(Id("test/kind"), "widget"),
	))
	require.NoError(t, err)

	// Delete all collection entries
	collectionPrefix := store.Prefix() + "/collections/"
	_, err = client.Delete(ctx, collectionPrefix, clientv3.WithPrefix())
	require.NoError(t, err)

	// Run dry-run reindex
	stats, err := store.Reindex(ctx, slog.Default(), ReindexOptions{
		DryRun:       true,
		CleanupStale: false,
	})
	require.NoError(t, err)
	assert.Greater(t, stats.EntitiesProcessed, int64(0))
	assert.Equal(t, int64(0), stats.IndexesRebuilt) // No writes in dry-run

	// Verify indexes are still missing (dry-run didn't write)
	ids, err := store.ListIndex(ctx, String(Id("test/kind"), "widget"))
	require.NoError(t, err)
	assert.Len(t, ids, 0)
}

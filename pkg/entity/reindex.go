package entity

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/mr-tron/base58"
	"miren.dev/runtime/pkg/cond"
)

// ReindexStats holds statistics about a reindex operation.
type ReindexStats struct {
	EntitiesProcessed        int64
	IndexesRebuilt           int64
	CollectionEntriesScanned int64
	StaleEntriesFound        int64
	StaleEntriesRemoved      int64
}

// ReindexOptions controls the behavior of a reindex operation.
type ReindexOptions struct {
	DryRun       bool
	CleanupStale bool
}

// Reindex rebuilds all index (collection) entries for every entity in the store.
// If opts.CleanupStale is true, it also scans for and removes stale collection entries
// that point to non-existent entities.
func (s *EtcdStore) Reindex(ctx context.Context, log *slog.Logger, opts ReindexOptions) (*ReindexStats, error) {
	s.ClearSchemaCache()

	stats := &ReindexStats{}

	// Phase 1: List all entities and rebuild indexes
	allEntityIDs, err := s.ListAllEntityIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list entities: %w", err)
	}

	log.Info("reindex: found entities", "count", len(allEntityIDs))

	log.Info("reindex: rebuilding indexes for current entities")
	for i, id := range allEntityIDs {
		select {
		case <-ctx.Done():
			return stats, ctx.Err()
		default:
		}

		ent, err := s.GetEntity(ctx, id)
		if err != nil {
			if errors.Is(err, cond.ErrNotFound{}) {
				continue
			}
			log.Warn("reindex: failed to get entity", "id", id, "error", err)
			continue
		}

		if !opts.DryRun {
			indexedAttrs := collectIndexedAttributesTolerant(ctx, s, ent.Attrs())
			for _, attrs := range indexedAttrs {
				for _, attr := range attrs {
					err := s.addToCollectionDirect(ctx, ent, attr.CAS())
					if err != nil {
						log.Warn("reindex: failed to add to collection", "id", id, "attr", attr.ID, "error", err)
					} else {
						stats.IndexesRebuilt++
					}
				}
			}
		}

		stats.EntitiesProcessed++

		if (i+1)%100 == 0 {
			log.Info("reindex: progress",
				"processed", stats.EntitiesProcessed,
				"total", len(allEntityIDs),
				"percent", (i+1)*100/len(allEntityIDs))
		}
	}

	// Phase 2: Clean up stale index entries (optional). Delegates to the shared,
	// CAS-guarded cleanup used by the background sweeper so both paths remove
	// orphans the same safe way. Manual reindex is the "big hammer": unbounded
	// deletes, no pacing.
	if opts.CleanupStale {
		log.Info("reindex: cleaning up stale index entries")
		cleanup, err := s.CleanupStaleCollectionEntries(ctx, log, CleanupOptions{DryRun: opts.DryRun})
		if err != nil {
			log.Warn("reindex: stale cleanup failed", "error", err)
		}
		if cleanup != nil {
			stats.CollectionEntriesScanned = cleanup.CollectionEntriesScanned
			stats.StaleEntriesFound = cleanup.StaleEntriesFound
			stats.StaleEntriesRemoved = cleanup.StaleEntriesRemoved
		}
	}

	log.Info("reindex: complete",
		"entities_processed", stats.EntitiesProcessed,
		"indexes_rebuilt", stats.IndexesRebuilt,
		"collection_entries_scanned", stats.CollectionEntriesScanned,
		"stale_entries_found", stats.StaleEntriesFound,
		"stale_entries_removed", stats.StaleEntriesRemoved)

	return stats, nil
}

// collectIndexedAttributesTolerant is like EtcdStore.collectIndexedAttributes but
// skips attributes whose schema cannot be looked up, rather than returning an error.
// This is appropriate for reindex where some attribute schemas may be missing.
func collectIndexedAttributesTolerant(ctx context.Context, store Store, attrs []Attr) map[Id][]Attr {
	indexedAttrs := make(map[Id][]Attr)
	allAttrs := enumerateAllAttrs(attrs)
	for _, attr := range allAttrs {
		schema, err := store.GetAttributeSchema(ctx, attr.ID)
		if err != nil {
			continue
		}
		if schema.Index {
			indexedAttrs[attr.ID] = append(indexedAttrs[attr.ID], attr)
		}
	}
	return indexedAttrs
}

var colReplacer = strings.NewReplacer("/", "_", ":", "_")

// addToCollectionDirect writes a single collection entry for the given entity and collection key.
func (s *EtcdStore) addToCollectionDirect(ctx context.Context, ent *Entity, collection string) error {
	key := base58.Encode([]byte(ent.Id()))
	colKey := colReplacer.Replace(collection)

	key = fmt.Sprintf("%s/collections/%s/%s", s.prefix, colKey, key)

	_, err := s.client.Put(ctx, key, ent.Id().String())
	return err
}

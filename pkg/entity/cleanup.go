package entity

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"miren.dev/runtime/pkg/cond"
)

// cleanupDeleteBatchSize is how many stale entries we delete between rate-limit
// pauses. Deletes are issued one CAS'd transaction at a time (see
// CleanupStaleCollectionEntries); this only governs how often we sleep for
// BatchPause, not how many keys share a transaction.
const cleanupDeleteBatchSize = 100

// CleanupOptions bounds a single stale-cleanup pass so it can run continuously
// in the background without overwhelming the store.
type CleanupOptions struct {
	// DryRun scans and counts stale entries but deletes nothing.
	DryRun bool
	// MaxDeletes caps how many stale entries a single pass removes. A large
	// legacy backlog then drains over several passes rather than one thundering
	// sweep. Zero means unbounded (the manual reindex "big hammer").
	MaxDeletes int
	// BatchPause is slept after every cleanupDeleteBatchSize deletes to
	// rate-limit write pressure. Zero disables pacing.
	BatchPause time.Duration
}

// CleanupStats reports what a stale-cleanup pass observed and did. RemovedByCollection
// breaks removals down by index/collection so a persistent, post-drain leak shows
// up as a specific collection that keeps re-accumulating orphans rather than
// hiding in an aggregate count.
type CleanupStats struct {
	CollectionEntriesScanned int64
	StaleEntriesFound        int64
	StaleEntriesRemoved      int64
	// CASConflicts counts entries that changed between scan and delete (the slot
	// was legitimately recreated/re-leased), so the CAS guard skipped them.
	CASConflicts        int64
	RemovedByCollection map[string]int64
}

// CleanupStaleCollectionEntries scans collection (index) entries and removes those
// whose backing entity is authoritatively absent under a linearizable read.
//
// Safety rests on creates being atomic: CreateEntity writes the entity key and its
// collection entries in one transaction, so a collection entry whose entity is
// absent is genuinely orphaned, with no create-in-flight race. Each delete is still
// CAS'd on the entry's mod-revision captured at scan time, so a slot legitimately
// recreated between scan and delete fails the compare and is left untouched (the
// ABA guard). The pass is idempotent and bounded, so it is safe to run repeatedly
// as a background sweep; repetition is what makes it resumable.
func (s *EtcdStore) CleanupStaleCollectionEntries(ctx context.Context, log *slog.Logger, opts CleanupOptions) (*CleanupStats, error) {
	stats := &CleanupStats{RemovedByCollection: map[string]int64{}}

	// Build the set of live entity ids so we only pay a per-entry GetEntity for
	// collection entries that look orphaned. After a store has drained, almost
	// every entry is in this set and the pass does no point reads at all.
	allEntityIDs, err := s.ListAllEntityIDs(ctx)
	if err != nil {
		return stats, fmt.Errorf("cleanup: failed to list entities: %w", err)
	}
	validEntityIDs := make(map[Id]bool, len(allEntityIDs))
	for _, id := range allEntityIDs {
		validEntityIDs[id] = true
	}

	collectionPrefix := s.prefix + "/collections/"

	// pending buffers confirmed-stale entries until we have a batch to delete.
	// We scan and delete incrementally rather than materializing the whole
	// keyspace: memory stays bounded by one page + one batch, and if the pass is
	// cut short (deadline/shutdown) everything deleted before that point sticks.
	var pending []staleCollectionEntry

	flush := func() error {
		if len(pending) == 0 {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		s.deleteStaleBatch(ctx, log, pending, stats)
		pending = pending[:0]
		if opts.BatchPause > 0 {
			select {
			case <-time.After(opts.BatchPause):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	}

	scanErr := scanPagedFunc(ctx, s.client, collectionPrefix, func(kv *mvccpb.KeyValue) error {
		stats.CollectionEntriesScanned++

		entityID := Id(kv.Value)
		if validEntityIDs[entityID] {
			return nil
		}

		// Not in the snapshot's live set; confirm authoritatively before
		// treating it as stale. A linearizable not-found is the safe signal;
		// any other error means we couldn't verify, so we leave it alone.
		if _, err := s.GetEntity(ctx, entityID); err != nil {
			if !errors.Is(err, cond.ErrNotFound{}) {
				log.Warn("cleanup: could not verify entity existence",
					"entity_id", entityID,
					"key", string(kv.Key),
					"error", err)
				return nil
			}
		} else {
			return nil
		}

		stats.StaleEntriesFound++
		if opts.DryRun {
			return nil
		}

		pending = append(pending, staleCollectionEntry{
			key:        string(kv.Key),
			modRev:     kv.ModRevision,
			collection: collectionFromKey(string(kv.Key), collectionPrefix),
		})

		// Flush a full batch, or a short one that would otherwise carry us past
		// MaxDeletes, so a bounded pass removes at most MaxDeletes and no more.
		atBudget := opts.MaxDeletes > 0 && int(stats.StaleEntriesRemoved)+len(pending) >= opts.MaxDeletes
		if len(pending) >= cleanupDeleteBatchSize || atBudget {
			if err := flush(); err != nil {
				return err
			}
		}
		if opts.MaxDeletes > 0 && stats.StaleEntriesRemoved >= int64(opts.MaxDeletes) {
			return errStopScan
		}
		return nil
	})

	if scanErr != nil && !errors.Is(scanErr, errStopScan) {
		// Persist whatever we already confirmed before surfacing the error;
		// partial progress is the whole point of streaming the sweep.
		if flushErr := flush(); flushErr != nil {
			log.Warn("cleanup: failed to flush after scan error", "flush_error", flushErr)
		}
		return stats, fmt.Errorf("cleanup: scan failed: %w", scanErr)
	}

	if err := flush(); err != nil {
		return stats, err
	}

	return stats, nil
}

// errStopScan is a sentinel returned by the scan callback to halt scanning once
// the per-pass delete budget is reached, without treating it as a real error.
var errStopScan = errors.New("cleanup: delete budget reached")

// staleCollectionEntry is a collection entry confirmed to point at an absent
// entity, along with the mod-revision it carried at scan time (the CAS guard).
type staleCollectionEntry struct {
	key        string
	modRev     int64
	collection string
}

// deleteStaleBatch removes a batch of confirmed-stale entries, CAS-guarded on the
// mod-revision each carried at scan time. It first tries one transaction guarded
// on every entry's mod-revision; if that succeeds (the common case for a static
// legacy backlog) the whole batch is removed in a single round trip. If any
// entry changed since the scan the batched compare fails, so it falls back to a
// per-entry CAS to remove the still-unchanged ones and count the conflicts. It
// records removals, CAS conflicts, and the per-collection breakdown into stats;
// per-entry failures are logged, never returned.
func (s *EtcdStore) deleteStaleBatch(ctx context.Context, log *slog.Logger, batch []staleCollectionEntry, stats *CleanupStats) {
	if len(batch) == 0 {
		return
	}

	cmps := make([]clientv3.Cmp, len(batch))
	ops := make([]clientv3.Op, len(batch))
	for i, e := range batch {
		cmps[i] = clientv3.Compare(clientv3.ModRevision(e.key), "=", e.modRev)
		ops[i] = clientv3.OpDelete(e.key)
	}

	resp, err := s.client.Txn(ctx).If(cmps...).Then(ops...).Commit()
	if err == nil && resp.Succeeded {
		for _, e := range batch {
			stats.StaleEntriesRemoved++
			stats.RemovedByCollection[e.collection]++
		}
		return
	}
	if err != nil {
		log.Warn("cleanup: batched delete failed, retrying per entry", "count", len(batch), "error", err)
	}

	// At least one slot changed (or the batch errored): CAS each entry on its own
	// so an ABA recreate is skipped rather than dragging down the rest of the batch.
	for _, e := range batch {
		r, err := s.client.Txn(ctx).
			If(clientv3.Compare(clientv3.ModRevision(e.key), "=", e.modRev)).
			Then(clientv3.OpDelete(e.key)).
			Commit()
		if err != nil {
			log.Warn("cleanup: failed to delete stale entry", "key", e.key, "error", err)
			continue
		}
		if !r.Succeeded {
			stats.CASConflicts++
			continue
		}
		stats.StaleEntriesRemoved++
		stats.RemovedByCollection[e.collection]++
	}
}

// collectionFromKey extracts the collection segment from a collection entry key.
// Keys are "{prefix}/collections/{colKey}/{base58(id)}" and colKey has its own
// slashes replaced (see addToCollectionDirect), so the collection is everything
// up to the first slash after the prefix.
func collectionFromKey(key, collectionPrefix string) string {
	rest := strings.TrimPrefix(key, collectionPrefix)
	if collection, _, found := strings.Cut(rest, "/"); found {
		return collection
	}
	return rest
}

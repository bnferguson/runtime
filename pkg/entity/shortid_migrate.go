package entity

import (
	"context"
	"fmt"
	"log/slog"
	"path"
	"strings"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// MigrateShortIdOptions configures the short-id migration behavior.
type MigrateShortIdOptions struct {
	DryRun bool
	Prefix string
}

// MigrateShortIds backfills db/short-id for all entities that don't have one.
// This is idempotent — entities that already have a short-id are skipped.
func MigrateShortIds(ctx context.Context, log *slog.Logger, client *clientv3.Client, opts MigrateShortIdOptions) (migrated int, skipped int, err error) {
	prefix := path.Join(opts.Prefix, "entity")
	uniquePrefix := path.Join(opts.Prefix, "unique") + "/"

	log.Info("starting short-id migration", "prefix", prefix, "dry_run", opts.DryRun)

	kvs, err := scanPaged(ctx, client, prefix)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to list entities: %w", err)
	}

	log.Info("found entities to scan for short-id migration", "count", len(kvs))

	// Build a set of existing unique keys for collision checking.
	// We use the full unique key (attrCAS-based) to avoid collisions
	// between different unique attributes.
	uniqueKvs, err := scanPaged(ctx, client, uniquePrefix, withKeysOnly())
	if err != nil {
		return 0, 0, fmt.Errorf("failed to list existing unique keys: %w", err)
	}

	existingUnique := make(map[string]struct{})
	for _, kv := range uniqueKvs {
		existingUnique[string(kv.Key)] = struct{}{}
	}

	var errorCount int

	for _, kv := range kvs {
		key := string(kv.Key)

		// Skip session keys
		if strings.Contains(key, "/session/") {
			continue
		}

		var ent Entity
		if err := Decode(kv.Value, &ent); err != nil {
			log.Warn("failed to decode entity during short-id migration", "key", key, "error", err)
			errorCount++
			continue
		}

		// Skip entities that already have a short-id
		if ent.ShortId() != "" {
			skipped++
			continue
		}

		// Skip system/schema entities (no entity/kind attribute)
		if _, hasKind := ent.Get(EntityKind); !hasKind {
			skipped++
			continue
		}

		entityId := string(ent.Id())
		if entityId == "" {
			skipped++
			continue
		}

		// Allocate short-id using in-memory set for collision checking.
		// We build the full unique key for each candidate to check against
		// the in-memory set.
		shortId, allocErr := AllocateShortId(entityId, func(candidate string) (bool, error) {
			candidateAttr := String(DBShortId, candidate)
			candidateKey := fmt.Sprintf("%s%s", uniquePrefix, candidateAttr.CAS())
			_, exists := existingUnique[candidateKey]
			return exists, nil
		})
		if allocErr != nil {
			log.Warn("failed to allocate short-id", "entity", entityId, "error", allocErr)
			errorCount++
			continue
		}

		if opts.DryRun {
			log.Info("dry-run: would assign short-id", "entity", entityId, "short_id", shortId)
			migrated++
			shortIdAttr := String(DBShortId, shortId)
			existingUnique[fmt.Sprintf("%s%s", uniquePrefix, shortIdAttr.CAS())] = struct{}{}
			continue
		}

		// Set the short-id on the entity
		ent.Set(String(DBShortId, shortId))

		newData, encErr := Encode(&ent)
		if encErr != nil {
			log.Warn("failed to encode entity with short-id", "entity", entityId, "error", encErr)
			errorCount++
			continue
		}

		// Write the updated entity and unique key atomically.
		// Guard with ModRevision to avoid clobbering concurrent updates.
		shortIdAttr := String(DBShortId, shortId)
		uniqueKey := fmt.Sprintf("%s%s", uniquePrefix, shortIdAttr.CAS())
		txnResp, txnErr := client.Txn(ctx).
			If(
				clientv3.Compare(clientv3.ModRevision(key), "=", kv.ModRevision),
				clientv3.Compare(clientv3.CreateRevision(uniqueKey), "=", 0),
			).
			Then(
				clientv3.OpPut(key, string(newData)),
				clientv3.OpPut(uniqueKey, entityId),
			).
			Commit()

		if txnErr != nil {
			log.Warn("failed to write short-id migration", "entity", entityId, "error", txnErr)
			errorCount++
			continue
		}

		if !txnResp.Succeeded {
			log.Warn("short-id migration skipped entity (concurrent modification or collision)",
				"entity", entityId, "short_id", shortId)
			errorCount++
			continue
		}

		existingUnique[uniqueKey] = struct{}{}
		migrated++

		if migrated%100 == 0 {
			log.Info("short-id migration progress", "migrated", migrated, "skipped", skipped)
		}
	}

	if errorCount > 0 {
		log.Warn("short-id migration completed with errors",
			"migrated", migrated, "skipped", skipped, "errors", errorCount)
		return migrated, skipped, fmt.Errorf("short-id migration completed with %d errors", errorCount)
	}

	log.Info("short-id migration completed", "migrated", migrated, "skipped", skipped)
	return migrated, skipped, nil
}

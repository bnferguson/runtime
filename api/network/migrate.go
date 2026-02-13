package network

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"miren.dev/runtime/api/entityserver/entityserver_v1alpha"
	"miren.dev/runtime/api/network/network_v1alpha"
	"miren.dev/runtime/pkg/entity"
)

// Old attribute IDs that no longer exist as constants after the rename.
const (
	oldKindService       = entity.Id("dev.miren.network/kind.service")
	oldServiceIp         = entity.Id("dev.miren.network/service.ip")
	oldServiceMatch      = entity.Id("dev.miren.network/service.match")
	oldServicePort       = entity.Id("dev.miren.network/service.port")
	oldEndpointsService  = entity.Id("dev.miren.network/endpoints.service")
	oldServicePortPrefix = "dev.miren.network/service.port"
)

// MigrateServiceToTarget renames stored Service entities to Target entities
// and updates Endpoints references from service to target.
func MigrateServiceToTarget(ctx context.Context, log *slog.Logger, eac *entityserver_v1alpha.EntityAccessClient) error {
	log.Info("migrating network service entities to target")

	if err := migrateServiceEntities(ctx, log, eac); err != nil {
		return fmt.Errorf("failed to migrate service entities: %w", err)
	}

	if err := migrateEndpointsEntities(ctx, log, eac); err != nil {
		return fmt.Errorf("failed to migrate endpoints entities: %w", err)
	}

	return nil
}

func migrateServiceEntities(ctx context.Context, log *slog.Logger, eac *entityserver_v1alpha.EntityAccessClient) error {
	resp, err := eac.List(ctx, entity.Ref(entity.EntityKind, oldKindService))
	if err != nil {
		return fmt.Errorf("failed to list service entities: %w", err)
	}

	migratedCount := 0

	for _, e := range resp.Values() {
		oldAttrs := e.Entity().Attrs()
		newAttrs := make([]entity.Attr, 0, len(oldAttrs))

		for _, attr := range oldAttrs {
			switch attr.ID {
			case entity.EntityKind:
				// Change kind ref from service to target
				if attr.Value.Id() == oldKindService {
					newAttrs = append(newAttrs, entity.Ref(entity.EntityKind, network_v1alpha.KindTarget))
				} else {
					newAttrs = append(newAttrs, attr)
				}
			case oldServiceIp:
				attr.ID = network_v1alpha.TargetIpId
				newAttrs = append(newAttrs, attr)
			case oldServiceMatch:
				attr.ID = network_v1alpha.TargetMatchId
				newAttrs = append(newAttrs, attr)
			case oldServicePort:
				attr.ID = network_v1alpha.TargetPortId
				newAttrs = append(newAttrs, attr)
			default:
				// Rename sub-component builder paths: service.port.* → target.port.*
				if strings.HasPrefix(string(attr.ID), oldServicePortPrefix) {
					newID := "dev.miren.network/target.port" + strings.TrimPrefix(string(attr.ID), oldServicePortPrefix)
					attr.ID = entity.Id(newID)
				}
				newAttrs = append(newAttrs, attr)
			}
		}

		// Use Replace to do a full entity replacement. Put uses UpdateEntity
		// which merges by attribute ID, leaving old renamed IDs in place.
		if _, err := eac.Replace(ctx, newAttrs, e.Revision()); err != nil {
			log.Error("failed to migrate service entity to target", "error", err, "id", e.Id())
			continue
		}

		migratedCount++
	}

	if migratedCount > 0 {
		log.Info("migrated service entities to target", "count", migratedCount)
	}

	return nil
}

func migrateEndpointsEntities(ctx context.Context, log *slog.Logger, eac *entityserver_v1alpha.EntityAccessClient) error {
	resp, err := eac.List(ctx, entity.Ref(entity.EntityKind, network_v1alpha.KindEndpoints))
	if err != nil {
		return fmt.Errorf("failed to list endpoints entities: %w", err)
	}

	migratedCount := 0

	for _, e := range resp.Values() {
		oldAttrs := e.Entity().Attrs()
		needsMigration := false

		for _, attr := range oldAttrs {
			if attr.ID == oldEndpointsService {
				needsMigration = true
				break
			}
		}

		if !needsMigration {
			continue
		}

		newAttrs := make([]entity.Attr, 0, len(oldAttrs))
		for _, attr := range oldAttrs {
			if attr.ID == oldEndpointsService {
				attr.ID = network_v1alpha.EndpointsTargetId
			}
			newAttrs = append(newAttrs, attr)
		}

		if _, err := eac.Replace(ctx, newAttrs, e.Revision()); err != nil {
			log.Error("failed to migrate endpoints entity", "error", err, "id", e.Id())
			continue
		}

		migratedCount++
	}

	if migratedCount > 0 {
		log.Info("migrated endpoints entities (service → target)", "count", migratedCount)
	}

	return nil
}

package compute

import (
	"context"
	"fmt"
	"log/slog"

	"miren.dev/runtime/api/core/core_v1alpha"
	"miren.dev/runtime/api/entityserver/entityserver_v1alpha"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/entity/types"
)

// MigrateAppVersionToConfigVersion creates ConfigVersion entities for
// existing AppVersion entities that still use inline Config (no ConfigVersion).
// This allows old entities to work after Phase 4 stops writing inline Config.
func MigrateAppVersionToConfigVersion(ctx context.Context, log *slog.Logger, eac *entityserver_v1alpha.EntityAccessClient) error {
	log.Info("migrating app versions to use ConfigVersion entities")

	resp, err := eac.List(ctx, entity.Ref(entity.EntityKind, core_v1alpha.KindAppVersion))
	if err != nil {
		return fmt.Errorf("failed to list app versions for migration: %w", err)
	}

	migratedCount := 0
	skippedCount := 0

	for _, e := range resp.Values() {
		var ver core_v1alpha.AppVersion
		ver.Decode(e.Entity())

		// Skip versions that already have a ConfigVersion
		if ver.ConfigVersion != "" {
			skippedCount++
			continue
		}

		// Skip versions with empty config (nothing to migrate)
		if len(ver.Config.Services) == 0 && len(ver.Config.Variable) == 0 &&
			len(ver.Config.Commands) == 0 && ver.Config.Port == 0 &&
			ver.Config.Entrypoint == "" && ver.Config.StartDirectory == "" {
			skippedCount++
			continue
		}

		log.Info("migrating app version to ConfigVersion",
			"version", ver.ID,
			"app_version", ver.Version)

		// Convert inline config to ConfigVersion
		configSpec := ConfigSpecFromConfig(&ver.Config)
		configVer := &core_v1alpha.ConfigVersion{
			App:  ver.App,
			Spec: configSpec,
		}

		// Create the ConfigVersion entity using the AppVersion's unique ID
		cvName := ver.ID.String() + "-cfg"
		var cvEntity entityserver_v1alpha.Entity
		cvEntity.SetAttrs(entity.New(
			(&core_v1alpha.Metadata{Name: cvName}).Encode,
			configVer.Encode,
			entity.Ident, types.Keyword("config_version/"+cvName),
		).Attrs())

		pr, err := eac.Put(ctx, &cvEntity)
		if err != nil {
			log.Error("failed to create ConfigVersion for app version",
				"version", ver.ID,
				"error", err)
			continue
		}

		// Update the AppVersion to reference the new ConfigVersion
		ver.ConfigVersion = entity.Id(pr.Id())

		var verEntity entityserver_v1alpha.Entity
		verEntity.SetId(ver.ID.String())
		verEntity.SetAttrs(entity.New(ver.Encode).Attrs())

		if _, err := eac.Put(ctx, &verEntity); err != nil {
			log.Error("failed to update app version with ConfigVersion",
				"version", ver.ID,
				"error", err)
			continue
		}

		migratedCount++
	}

	log.Info("completed app version to ConfigVersion migration",
		"migrated", migratedCount,
		"skipped", skippedCount)

	return nil
}

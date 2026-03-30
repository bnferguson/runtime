package mysql

import (
	"context"
	"fmt"

	"miren.dev/runtime/api/addon/addon_v1alpha"
	"miren.dev/runtime/pkg/addon"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/saga"
)

// mysqlServerCounter implements dbsaga.ServerCounter for MySQL.
type mysqlServerCounter struct{}

func (mysqlServerCounter) GetAssociationCount(ctx context.Context, serverID entity.Id) (int64, int64, error) {
	fw := saga.Get[*addon.ProviderFramework](ctx)

	var server addon_v1alpha.MysqlServer
	ent, err := fw.EC.GetByIdWithEntity(ctx, serverID, &server)
	if err != nil {
		return 0, 0, fmt.Errorf("getting mysql server: %w", err)
	}

	return server.AssociationCount, ent.Revision(), nil
}

func (mysqlServerCounter) PatchAssociationCount(ctx context.Context, serverID entity.Id, revision int64, newCount int64) error {
	fw := saga.Get[*addon.ProviderFramework](ctx)

	return fw.EC.Patch(ctx, serverID, revision,
		entity.Int64(addon_v1alpha.MysqlServerAssociationCountId, newCount),
	)
}

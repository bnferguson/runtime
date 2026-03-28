package mysql

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"

	"miren.dev/runtime/pkg/addon"
)

type Provider struct {
	fw  *addon.ProviderFramework
	log *slog.Logger
}

func NewProvider(fw *addon.ProviderFramework) *Provider {
	return &Provider{
		fw:  fw,
		log: fw.Log.With("addon", AddonName),
	}
}

func (p *Provider) LocalityMode() addon.LocalityMode {
	return addon.OnCluster
}

func (p *Provider) Provision(ctx context.Context, app addon.App, variant addon.Variant) (*addon.ProvisionResult, error) {
	if IsSharedVariant(variant.Name) {
		return p.provisionShared(ctx, app, variant)
	}
	return p.provisionDedicated(ctx, app, variant)
}

func (p *Provider) AdjustEnvVars(ctx context.Context, result *addon.ProvisionResult, assoc addon.AddonAssociation, collisions []string) ([]addon.Variable, error) {
	return result.EnvVars, nil
}

func (p *Provider) Deprovision(ctx context.Context, assoc addon.AddonAssociation) error {
	variant := assoc.Variant
	if IsSharedVariant(variant) {
		return p.deprovisionShared(ctx, assoc)
	}
	return p.deprovisionDedicated(ctx, assoc)
}

func buildDatabaseURL(host string, port int, user, password, database string) string {
	u := &url.URL{
		Scheme:   "mysql",
		User:     url.UserPassword(user, password),
		Host:     fmt.Sprintf("%s:%d", host, port),
		Path:     database,
	}
	return u.String()
}

func buildEnvVars(host string, port int, user, password, database string) []addon.Variable {
	return []addon.Variable{
		{Key: "DATABASE_URL", Value: buildDatabaseURL(host, port, user, password, database), Sensitive: true},
		{Key: "MYSQL_HOST", Value: host},
		{Key: "MYSQL_PORT", Value: fmt.Sprintf("%d", port)},
		{Key: "MYSQL_USER", Value: user},
		{Key: "MYSQL_PASSWORD", Value: password, Sensitive: true},
		{Key: "MYSQL_DATABASE", Value: database},
	}
}

package ingress

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"miren.dev/runtime/api/entityserver"
	"miren.dev/runtime/api/entityserver/entityserver_v1alpha"
	"miren.dev/runtime/api/ingress/ingress_v1alpha"
	"miren.dev/runtime/pkg/cond"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/rpc"
)

// Client provides a domain-specific client for HttpRoute entities
type Client struct {
	log *slog.Logger
	ec  *entityserver.Client
	eac *entityserver_v1alpha.EntityAccessClient
}

// NewClient creates a new Ingress client from an RPC client
func NewClient(log *slog.Logger, client rpc.Client) *Client {
	eac := entityserver_v1alpha.NewEntityAccessClient(client)
	entityClient := entityserver.NewClient(log, eac)

	return &Client{
		log: log.With("module", "ingress-client"),
		ec:  entityClient,
		eac: eac,
	}
}

// GetEntityStore returns the underlying entity store
func (c *Client) GetEntityStore() *entityserver.Client {
	return c.ec
}

// Lookup finds an http_route by hostname, returns nil if not found
func (c *Client) Lookup(ctx context.Context, host string) (*ingress_v1alpha.HttpRoute, error) {
	ia := entity.String(ingress_v1alpha.HttpRouteHostId, strings.ToLower(host))

	var route ingress_v1alpha.HttpRoute
	err := c.ec.OneAtIndex(ctx, ia, &route)
	if err != nil {
		if errors.Is(err, cond.ErrNotFound{}) {
			return nil, nil
		} else {
			return nil, fmt.Errorf("failed to lookup route for host %s: %w", host, err)
		}
	}

	return &route, nil
}

// LookupWithWildcard finds an http_route by hostname with wildcard fallback.
// It tries in order: exact match, then wildcard subdomain (*.rest).
// A wildcard like *.example.com matches foo.example.com but not example.com itself.
func (c *Client) LookupWithWildcard(ctx context.Context, host string) (*ingress_v1alpha.HttpRoute, error) {
	host = strings.ToLower(host)

	// Step 1: exact match
	route, err := c.Lookup(ctx, host)
	if err != nil {
		return nil, err
	}
	if route != nil {
		return route, nil
	}

	// Step 2: replace first label with wildcard (e.g., foo.example.com → *.example.com)
	if idx := strings.Index(host, "."); idx > 0 {
		wildcard := "*" + host[idx:]
		route, err = c.Lookup(ctx, wildcard)
		if err != nil {
			return nil, err
		}
		if route != nil {
			return route, nil
		}
	}

	return nil, nil
}

// ValidateWildcardHost validates a wildcard host pattern.
// Valid patterns: *.example.com, *.sub.example.com
// Invalid: *.com, foo.*.com, **, *
func ValidateWildcardHost(host string) error {
	if !strings.HasPrefix(host, "*.") {
		return nil
	}
	remainder := host[2:]
	if remainder == "" || strings.Contains(remainder, "*") {
		return fmt.Errorf("invalid wildcard pattern: %s (must be *.domain.tld)", host)
	}
	if !strings.Contains(remainder, ".") {
		return fmt.Errorf("invalid wildcard pattern: %s (must have at least two domain labels after *)", host)
	}
	return nil
}

// ExtractSubdomainLabel extracts an ephemeral label from a request host
// by comparing it against the route's configured host pattern. For example,
// if requestHost is "feat-x.app.example.com" and the route host is
// "*.app.example.com", it returns "feat-x". Returns an empty string if
// the route is not a wildcard or if there's no subdomain prefix.
func ExtractSubdomainLabel(requestHost, routeHost string) string {
	requestHost = strings.ToLower(requestHost)
	routeHost = strings.ToLower(routeHost)

	if !strings.HasPrefix(routeHost, "*.") {
		return ""
	}

	// routeHost is "*.base.example.com", base is "base.example.com"
	base := routeHost[2:]

	if !strings.HasSuffix(requestHost, "."+base) {
		return ""
	}

	// Extract the prefix: "feat-x.app.example.com" minus ".app.example.com"
	label := requestHost[:len(requestHost)-len(base)-1]

	// Only return single-label prefixes (no dots)
	if strings.Contains(label, ".") {
		return ""
	}

	return label
}

// LookupDefault finds the default http_route
func (c *Client) LookupDefault(ctx context.Context) (*ingress_v1alpha.HttpRoute, error) {
	var route ingress_v1alpha.HttpRoute
	err := c.ec.OneAtIndex(ctx, entity.Bool(ingress_v1alpha.HttpRouteDefaultId, true), &route)
	if err != nil {
		if errors.Is(err, cond.ErrNotFound{}) {
			return nil, nil
		} else {
			return nil, fmt.Errorf("failed to lookup default route: %w", err)
		}
	}
	return &route, nil
}

// SetDefault sets the default route to the provided app
func (c *Client) SetDefault(ctx context.Context, appId entity.Id) (*ingress_v1alpha.HttpRoute, error) {
	// Since host is blank for default routes, and it's normally used for the ID field, we make a special ID format
	routeId := fmt.Sprintf("default-%s", appId)

	route := &ingress_v1alpha.HttpRoute{
		ID:      entity.Id(routeId),
		App:     appId,
		Default: true,
	}
	if _, err := c.ec.CreateOrUpdate(ctx, routeId, route); err != nil {
		return nil, fmt.Errorf("failed to create default route: %w", err)
	}

	return route, nil
}

// UnsetDefault unsets the default route, if any. It returns the route that it unset the default from.
func (c *Client) UnsetDefault(ctx context.Context) (*ingress_v1alpha.HttpRoute, error) {
	route, err := c.LookupDefault(ctx)
	if err != nil {
		return nil, err
	}

	if route == nil {
		return nil, nil
	}

	if err := c.ec.Delete(ctx, route.ID); err != nil {
		return nil, fmt.Errorf("failed to delete default route: %w", err)
	}

	return route, nil
}

// EnsureSingleDefault removes any default routes but the one specified
func (c *Client) EnsureSingleDefault(ctx context.Context, routeToKeep *ingress_v1alpha.HttpRoute) error {
	resp, err := c.ec.List(ctx, entity.Bool(ingress_v1alpha.HttpRouteDefaultId, true))
	if err != nil {
		return fmt.Errorf("failed to list default routes: %w", err)
	}

	for resp.Next() {
		var route ingress_v1alpha.HttpRoute
		if err := resp.Read(&route); err != nil {
			c.log.Error("Failed to read route", "error", err)
			continue
		}

		// Skip the route we want to keep as default
		if route.ID == routeToKeep.ID {
			continue
		}

		c.log.Info("Deleting old default route", "route", route.ID)
		if err := c.ec.Delete(ctx, route.ID); err != nil {
			return fmt.Errorf("failed to delete old default route %s: %w", route.ID, err)
		}
	}

	return nil
}

// RouteWithMeta includes an http_route with its metadata
type RouteWithMeta struct {
	Route     *ingress_v1alpha.HttpRoute
	CreatedAt int64
	UpdatedAt int64
}

// List returns all http_routes with metadata
func (c *Client) List(ctx context.Context) ([]*RouteWithMeta, error) {
	kindRes, err := c.eac.LookupKind(ctx, "http_route")
	if err != nil {
		return nil, fmt.Errorf("failed to lookup http_route kind: %w", err)
	}

	res, err := c.eac.List(ctx, kindRes.Attr())
	if err != nil {
		return nil, fmt.Errorf("failed to list routes: %w", err)
	}

	var routes []*RouteWithMeta
	for _, e := range res.Values() {
		var route ingress_v1alpha.HttpRoute
		route.Decode(e.Entity())
		routes = append(routes, &RouteWithMeta{
			Route:     &route,
			CreatedAt: e.CreatedAt(),
			UpdatedAt: e.UpdatedAt(),
		})
	}

	return routes, nil
}

// SetRoute creates or updates an http_route for the given host and app
func (c *Client) SetRoute(ctx context.Context, host string, appId entity.Id) (*ingress_v1alpha.HttpRoute, error) {
	route := &ingress_v1alpha.HttpRoute{
		Host: strings.ToLower(host),
		App:  appId,
	}

	// Use the host as the route name/ID
	_, err := c.ec.CreateOrUpdate(ctx, host, route)
	if err != nil {
		return nil, fmt.Errorf("failed to create/update route: %w", err)
	}

	return route, nil
}

// DeleteByHost deletes an http_route by hostname
func (c *Client) DeleteByHost(ctx context.Context, host string) error {
	route, err := c.Lookup(ctx, host)
	if err != nil {
		return err
	}

	if route == nil {
		return fmt.Errorf("route not found: %s", host)
	}

	if err := c.ec.Delete(ctx, route.ID); err != nil {
		return fmt.Errorf("failed to delete route: %w", err)
	}

	return nil
}

// CreateOrUpdateOIDCProvider creates or updates an OIDC provider
func (c *Client) CreateOrUpdateOIDCProvider(ctx context.Context, provider *ingress_v1alpha.OidcProvider) (*ingress_v1alpha.OidcProvider, error) {
	if provider.Name == "" {
		return nil, fmt.Errorf("provider name is required")
	}

	_, err := c.ec.CreateOrUpdate(ctx, provider.Name, provider)
	if err != nil {
		return nil, fmt.Errorf("failed to create/update OIDC provider: %w", err)
	}

	return provider, nil
}

// GetOIDCProvider looks up an OIDC provider by name
func (c *Client) GetOIDCProvider(ctx context.Context, name string) (*ingress_v1alpha.OidcProvider, error) {
	ia := entity.String(ingress_v1alpha.OidcProviderNameId, name)

	var provider ingress_v1alpha.OidcProvider
	err := c.ec.OneAtIndex(ctx, ia, &provider)
	if err != nil {
		if errors.Is(err, cond.ErrNotFound{}) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to lookup OIDC provider %s: %w", name, err)
	}

	return &provider, nil
}

// ListOIDCProviders returns all OIDC providers
func (c *Client) ListOIDCProviders(ctx context.Context) ([]*ingress_v1alpha.OidcProvider, error) {
	kindRes, err := c.eac.LookupKind(ctx, "oidc_provider")
	if err != nil {
		return nil, fmt.Errorf("failed to lookup oidc_provider kind: %w", err)
	}

	res, err := c.eac.List(ctx, kindRes.Attr())
	if err != nil {
		return nil, fmt.Errorf("failed to list OIDC providers: %w", err)
	}

	var providers []*ingress_v1alpha.OidcProvider
	for _, e := range res.Values() {
		var provider ingress_v1alpha.OidcProvider
		provider.Decode(e.Entity())
		providers = append(providers, &provider)
	}

	return providers, nil
}

// DeleteOIDCProvider deletes an OIDC provider by name
func (c *Client) DeleteOIDCProvider(ctx context.Context, name string) error {
	provider, err := c.GetOIDCProvider(ctx, name)
	if err != nil {
		return err
	}

	if provider == nil {
		return fmt.Errorf("OIDC provider not found: %s", name)
	}

	if err := c.ec.Delete(ctx, provider.ID); err != nil {
		return fmt.Errorf("failed to delete OIDC provider: %w", err)
	}

	return nil
}

// AttachOIDCProvider associates an OIDC provider with a route and sets claim mappings
func (c *Client) AttachOIDCProvider(ctx context.Context, host string, providerName string, claimMappings []ingress_v1alpha.ClaimMappings) (*ingress_v1alpha.HttpRoute, error) {
	route, err := c.Lookup(ctx, host)
	if err != nil {
		return nil, err
	}

	if route == nil {
		return nil, fmt.Errorf("route not found: %s", host)
	}

	return c.AttachOIDCProviderToRoute(ctx, route, providerName, claimMappings)
}

// AttachOIDCProviderToRoute associates an OIDC provider with an already-resolved route
func (c *Client) AttachOIDCProviderToRoute(ctx context.Context, route *ingress_v1alpha.HttpRoute, providerName string, claimMappings []ingress_v1alpha.ClaimMappings) (*ingress_v1alpha.HttpRoute, error) {
	// Look up provider
	provider, err := c.GetOIDCProvider(ctx, providerName)
	if err != nil {
		return nil, err
	}

	if provider == nil {
		return nil, fmt.Errorf("OIDC provider not found: %s", providerName)
	}

	// Update route with provider reference and claim mappings
	route.OidcProvider = provider.ID
	route.ClaimMappings = claimMappings

	// Update the route
	err = c.ec.Update(ctx, route)
	if err != nil {
		return nil, fmt.Errorf("failed to attach OIDC provider to route: %w", err)
	}

	return route, nil
}

// DetachOIDCProvider removes OIDC provider association from a route
func (c *Client) DetachOIDCProvider(ctx context.Context, host string) (*ingress_v1alpha.HttpRoute, error) {
	route, err := c.Lookup(ctx, host)
	if err != nil {
		return nil, err
	}

	if route == nil {
		return nil, fmt.Errorf("route not found: %s", host)
	}

	return c.DetachOIDCProviderFromRoute(ctx, route)
}

// DetachOIDCProviderFromRoute removes OIDC provider association from an already-resolved route
func (c *Client) DetachOIDCProviderFromRoute(ctx context.Context, route *ingress_v1alpha.HttpRoute) (*ingress_v1alpha.HttpRoute, error) {
	// Clear provider reference and claim mappings
	route.OidcProvider = ""
	route.ClaimMappings = nil

	// Update the route
	err := c.ec.Update(ctx, route)
	if err != nil {
		return nil, fmt.Errorf("failed to detach OIDC provider from route: %w", err)
	}

	return route, nil
}

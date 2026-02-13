package ingress

import (
	"context"
	"log/slog"
	"strings"
	"sync"

	"miren.dev/runtime/api/entityserver/entityserver_v1alpha"
	"miren.dev/runtime/api/ingress/ingress_v1alpha"
	"miren.dev/runtime/components/autotls"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/rpc/stream"
)

// RouteWatcher watches http_route entities and maintains an in-memory set
// of hosts with configured routes. It implements autotls.RouteWatcher.
type RouteWatcher struct {
	log   *slog.Logger
	eac   *entityserver_v1alpha.EntityAccessClient
	hosts *autotls.RouteSet

	// routeHosts tracks which host each route is configured for (route ID → host).
	// This enables proper reference counting when routes are updated or deleted.
	routeHosts   map[string]string
	routeHostsMu sync.RWMutex
}

// NewRouteWatcher creates a new RouteWatcher.
func NewRouteWatcher(log *slog.Logger, eac *entityserver_v1alpha.EntityAccessClient) *RouteWatcher {
	return &RouteWatcher{
		log:        log.With("module", "route-watcher"),
		eac:        eac,
		hosts:      autotls.NewRouteSet(),
		routeHosts: make(map[string]string),
	}
}

// RouteSet returns the underlying RouteSet for use with autotls.ServeTLS.
func (rw *RouteWatcher) RouteSet() *autotls.RouteSet {
	return rw.hosts
}

// LoadExistingRoutes loads all existing http_route entities into the set.
// This is synchronous and should be called before starting the TLS server
// to avoid a race where cert requests arrive before routes are loaded.
func (rw *RouteWatcher) LoadExistingRoutes(ctx context.Context) error {
	return rw.loadExistingRoutes(ctx)
}

// Watch watches for route changes and updates the host set.
// It blocks until the context is cancelled.
// Call LoadExistingRoutes first to populate the initial set.
func (rw *RouteWatcher) Watch(ctx context.Context) error {
	rw.log.Info("starting route watcher")

	index := entity.Ref(entity.EntityKind, ingress_v1alpha.KindHttpRoute)

	_, err := rw.eac.WatchIndex(ctx, index, stream.Callback(func(op *entityserver_v1alpha.EntityOp) error {
		if op == nil {
			return nil
		}

		routeID := op.EntityId()

		switch op.OperationType() {
		case entityserver_v1alpha.EntityOperationCreate, entityserver_v1alpha.EntityOperationUpdate:
			if op.Entity() == nil {
				return nil
			}

			var route ingress_v1alpha.HttpRoute
			route.Decode(op.Entity().Entity())

			host := strings.ToLower(strings.TrimSpace(route.Host))

			if op.OperationType() == entityserver_v1alpha.EntityOperationCreate {
				if host == "" {
					return nil
				}
				rw.log.Debug("route created", "id", routeID, "host", host)
				rw.recordRouteHost(routeID, host)
			} else {
				rw.updateRouteHost(routeID, host)
			}

		case entityserver_v1alpha.EntityOperationDelete:
			rw.removeRouteHost(routeID)
		}

		return nil
	}))

	return err
}

// recordRouteHost records a new route's host and adds it to the host set.
func (rw *RouteWatcher) recordRouteHost(routeID, host string) {
	rw.routeHostsMu.Lock()
	defer rw.routeHostsMu.Unlock()

	rw.routeHosts[routeID] = host
	rw.hosts.Add(host)
}

// updateRouteHost handles a route update, properly managing host changes.
func (rw *RouteWatcher) updateRouteHost(routeID, newHost string) {
	rw.routeHostsMu.Lock()
	defer rw.routeHostsMu.Unlock()

	oldHost, existed := rw.routeHosts[routeID]

	// Handle case where route previously had no host (or wasn't tracked)
	if !existed || oldHost == "" {
		if newHost != "" {
			rw.routeHosts[routeID] = newHost
			rw.hosts.Add(newHost)
			rw.log.Debug("route updated, host added", "id", routeID, "host", newHost)
		}
		return
	}

	// Handle case where route now has no host
	if newHost == "" {
		delete(rw.routeHosts, routeID)
		if !rw.hostHasOtherRoutesLocked(oldHost, routeID) {
			rw.hosts.Remove(oldHost)
			rw.log.Debug("route updated, host removed", "id", routeID, "oldHost", oldHost)
		}
		return
	}

	// Host changed
	if oldHost != newHost {
		rw.routeHosts[routeID] = newHost
		rw.hosts.Add(newHost)
		if !rw.hostHasOtherRoutesLocked(oldHost, routeID) {
			rw.hosts.Remove(oldHost)
		}
		rw.log.Debug("route updated, host changed", "id", routeID, "oldHost", oldHost, "newHost", newHost)
	}
}

// removeRouteHost handles a route deletion, only removing the host if no other routes use it.
func (rw *RouteWatcher) removeRouteHost(routeID string) {
	rw.routeHostsMu.Lock()
	defer rw.routeHostsMu.Unlock()

	host, existed := rw.routeHosts[routeID]
	if !existed || host == "" {
		return
	}

	delete(rw.routeHosts, routeID)

	if !rw.hostHasOtherRoutesLocked(host, routeID) {
		rw.hosts.Remove(host)
		rw.log.Debug("route deleted, host removed", "id", routeID, "host", host)
	} else {
		rw.log.Debug("route deleted, host retained (other routes exist)", "id", routeID, "host", host)
	}
}

// hostHasOtherRoutesLocked checks if any route other than excludeID uses the given host.
// Must be called with routeHostsMu held.
func (rw *RouteWatcher) hostHasOtherRoutesLocked(host, excludeID string) bool {
	for id, h := range rw.routeHosts {
		if id != excludeID && h == host {
			return true
		}
	}
	return false
}

// Start loads all existing routes and begins watching for changes.
// It blocks until the context is cancelled.
// For more control over timing, use LoadExistingRoutes followed by Watch.
func (rw *RouteWatcher) Start(ctx context.Context) error {
	if err := rw.LoadExistingRoutes(ctx); err != nil {
		return err
	}
	return rw.Watch(ctx)
}

// loadExistingRoutes loads all existing http_route entities into the set.
func (rw *RouteWatcher) loadExistingRoutes(ctx context.Context) error {
	res, err := rw.eac.List(ctx, entity.Ref(entity.EntityKind, ingress_v1alpha.KindHttpRoute))
	if err != nil {
		return err
	}

	rw.routeHostsMu.Lock()
	defer rw.routeHostsMu.Unlock()

	// Reset tracking state
	rw.routeHosts = make(map[string]string)

	var hosts []string
	for _, ent := range res.Values() {
		routeID := ent.Id()

		var route ingress_v1alpha.HttpRoute
		route.Decode(ent.Entity())

		host := strings.ToLower(strings.TrimSpace(route.Host))
		if host != "" {
			hosts = append(hosts, host)
			rw.routeHosts[routeID] = host
		}
	}

	rw.hosts.Replace(hosts)
	rw.log.Info("loaded existing routes", "count", len(hosts))

	return nil
}

// StartBackground starts the watcher in a goroutine and returns immediately.
// Use this when you need the watcher running but don't want to block.
func (rw *RouteWatcher) StartBackground(ctx context.Context, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := rw.Start(ctx); err != nil {
			rw.log.Error("route watcher error", "error", err)
		}
	}()
}

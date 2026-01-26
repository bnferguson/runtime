package ingress

import (
	"log/slog"
	"os"
	"testing"

	"miren.dev/runtime/components/autotls"
)

func TestRouteWatcher_ReferenceCountingHelpers(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	t.Run("recordRouteHost adds host to set", func(t *testing.T) {
		rw := &RouteWatcher{
			log:        log,
			hosts:      autotls.NewRouteSet(),
			routeHosts: make(map[string]string),
		}

		rw.recordRouteHost("route-1", "example.com")

		if !rw.hosts.HasRoute("example.com") {
			t.Error("expected example.com to be in host set")
		}
		if rw.routeHosts["route-1"] != "example.com" {
			t.Error("expected route-1 to be tracked")
		}
	})

	t.Run("multiple routes same host", func(t *testing.T) {
		rw := &RouteWatcher{
			log:        log,
			hosts:      autotls.NewRouteSet(),
			routeHosts: make(map[string]string),
		}

		rw.recordRouteHost("route-1", "example.com")
		rw.recordRouteHost("route-2", "example.com")

		// Delete one route - host should remain
		rw.removeRouteHost("route-1")

		if !rw.hosts.HasRoute("example.com") {
			t.Error("expected example.com to still be in host set after deleting one route")
		}

		// Delete second route - host should be removed
		rw.removeRouteHost("route-2")

		if rw.hosts.HasRoute("example.com") {
			t.Error("expected example.com to be removed after deleting all routes")
		}
	})

	t.Run("update route host changes tracking", func(t *testing.T) {
		rw := &RouteWatcher{
			log:        log,
			hosts:      autotls.NewRouteSet(),
			routeHosts: make(map[string]string),
		}

		rw.recordRouteHost("route-1", "old.example.com")

		// Update to new host
		rw.updateRouteHost("route-1", "new.example.com")

		if rw.hosts.HasRoute("old.example.com") {
			t.Error("expected old.example.com to be removed")
		}
		if !rw.hosts.HasRoute("new.example.com") {
			t.Error("expected new.example.com to be added")
		}
		if rw.routeHosts["route-1"] != "new.example.com" {
			t.Error("expected route tracking to be updated")
		}
	})

	t.Run("update host shared by multiple routes", func(t *testing.T) {
		rw := &RouteWatcher{
			log:        log,
			hosts:      autotls.NewRouteSet(),
			routeHosts: make(map[string]string),
		}

		rw.recordRouteHost("route-1", "shared.example.com")
		rw.recordRouteHost("route-2", "shared.example.com")

		// Update route-1 to different host
		rw.updateRouteHost("route-1", "new.example.com")

		// shared.example.com should still exist (route-2 uses it)
		if !rw.hosts.HasRoute("shared.example.com") {
			t.Error("expected shared.example.com to remain (route-2 still uses it)")
		}
		if !rw.hosts.HasRoute("new.example.com") {
			t.Error("expected new.example.com to be added")
		}
	})

	t.Run("update removes host when going empty", func(t *testing.T) {
		rw := &RouteWatcher{
			log:        log,
			hosts:      autotls.NewRouteSet(),
			routeHosts: make(map[string]string),
		}

		rw.recordRouteHost("route-1", "example.com")

		// Update to empty host (simulates removing host from route config)
		rw.updateRouteHost("route-1", "")

		if rw.hosts.HasRoute("example.com") {
			t.Error("expected example.com to be removed when route host becomes empty")
		}
		if _, exists := rw.routeHosts["route-1"]; exists {
			t.Error("expected route tracking to be removed")
		}
	})

	t.Run("update adds host when previously empty", func(t *testing.T) {
		rw := &RouteWatcher{
			log:        log,
			hosts:      autotls.NewRouteSet(),
			routeHosts: make(map[string]string),
		}

		// Update a route that wasn't tracked (had no host)
		rw.updateRouteHost("route-1", "example.com")

		if !rw.hosts.HasRoute("example.com") {
			t.Error("expected example.com to be added")
		}
		if rw.routeHosts["route-1"] != "example.com" {
			t.Error("expected route to be tracked")
		}
	})

	t.Run("delete untracked route is no-op", func(t *testing.T) {
		rw := &RouteWatcher{
			log:        log,
			hosts:      autotls.NewRouteSet(),
			routeHosts: make(map[string]string),
		}

		// This should not panic or cause issues
		rw.removeRouteHost("nonexistent-route")
	})
}

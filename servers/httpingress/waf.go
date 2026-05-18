package httpingress

import (
	"context"
	"net/http"
	"time"

	"miren.dev/runtime/api/ingress/ingress_v1alpha"
	"miren.dev/runtime/pkg/entity"
)

const wafProfileCacheTTL = 30 * time.Second

type wafProfileEntry struct {
	paranoiaLevel int
	fetchedAt     time.Time
}

func (s *Server) wafMiddleware(route *ingress_v1alpha.HttpRoute, next http.HandlerFunc) http.HandlerFunc {
	if entity.Empty(route.WafProfile) {
		return next
	}

	return func(w http.ResponseWriter, r *http.Request) {
		level := s.resolveWAFProfileLevel(r.Context(), route.WafProfile)
		if level <= 0 {
			next(w, r)
			return
		}

		handler, err := s.wafEngine.Handler(level, http.HandlerFunc(next))
		if err != nil {
			s.Log.Error("failed to create WAF handler", "error", err, "level", level)
			next(w, r)
			return
		}

		handler.ServeHTTP(w, r)
	}
}

func (s *Server) resolveWAFProfileLevel(ctx context.Context, profileID entity.Id) int {
	s.wafProfileMu.RLock()
	if entry, ok := s.wafProfileCache[profileID]; ok && time.Since(entry.fetchedAt) < wafProfileCacheTTL {
		s.wafProfileMu.RUnlock()
		return entry.paranoiaLevel
	}
	s.wafProfileMu.RUnlock()

	profile, err := s.ingressClient.GetWAFProfileByID(ctx, profileID)
	if err != nil {
		s.Log.Error("failed to resolve WAF profile", "error", err, "profile_id", profileID)
		return 0
	}
	if profile == nil {
		s.Log.Warn("WAF profile not found", "profile_id", profileID)
		return 0
	}

	level := int(profile.ParanoiaLevel)

	s.wafProfileMu.Lock()
	s.wafProfileCache[profileID] = &wafProfileEntry{
		paranoiaLevel: level,
		fetchedAt:     time.Now(),
	}
	s.wafProfileMu.Unlock()

	return level
}

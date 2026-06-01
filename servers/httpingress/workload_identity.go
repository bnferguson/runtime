package httpingress

import "net/http"

func (h *Server) handleOIDCDiscovery(w http.ResponseWriter, req *http.Request) {
	if h.workloadIssuer == nil {
		http.NotFound(w, req)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write(h.workloadIssuer.DiscoveryDocument())
}

func (h *Server) handleJWKS(w http.ResponseWriter, req *http.Request) {
	if h.workloadIssuer == nil {
		http.NotFound(w, req)
		return
	}
	data, err := h.workloadIssuer.JWKSDocument()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/jwk-set+json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write(data)
}

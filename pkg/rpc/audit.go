package rpc

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
)

// certFingerprint returns the hex-encoded SHA-256 of a certificate's raw DER,
// the standard stable identifier for a cert in an audit trail.
func certFingerprint(cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(sum[:])
}

// minLevelHandler wraps a slog.Handler and enforces its own minimum level,
// independent of the wrapped handler's configured level. Records below min are
// dropped even if the wrapped handler would emit them; records at or above min
// are passed straight to the wrapped handler's Handle, which does the
// formatting and writing without re-checking the level.
//
// The Info floor is therefore only as reliable as that last assumption: it
// relies on the wrapped sink not re-gating in Handle. That is the slog.Handler
// contract (the standard library handlers gate only in Enabled, which the
// Logger checks before calling Handle), so any well-behaved sink is fine, but a
// sink that re-checks the level in Handle would silently break the floor.
//
// The audit trail uses this to hold a fixed Info floor regardless of the
// process-wide verbosity. Managed servers run at -vv (Debug), so a plain shared
// logger would never suppress the audit stream's own Debug records (loopback
// internal traffic); conversely the default is Warn, at which a shared logger
// would drop audit Info entirely. Pinning the floor at Info decouples the audit
// trail from the operational -v knob in both directions.
type minLevelHandler struct {
	slog.Handler
	min slog.Level
}

func (h minLevelHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.min
}

func (h minLevelHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return minLevelHandler{Handler: h.Handler.WithAttrs(attrs), min: h.min}
}

func (h minLevelHandler) WithGroup(name string) slog.Handler {
	return minLevelHandler{Handler: h.Handler.WithGroup(name), min: h.min}
}

// newAuditLogger derives the security audit logger from the server's base
// logger. It shares the base's sink (so audit records flow to the same log
// collection) but pins the level floor at Info via minLevelHandler and tags the
// stream module=audit so it can be filtered and routed downstream.
func newAuditLogger(base *slog.Logger) *slog.Logger {
	return slog.New(minLevelHandler{Handler: base.Handler(), min: slog.LevelInfo}).
		With("module", "audit")
}

// isLoopbackAddr reports whether addr (an *http.Request RemoteAddr in "host:port"
// form) is a loopback address — i.e. a same-host internal component rather than
// a network peer. Loopback traffic is the coordinator's own services talking to
// each other (e.g. the API server polling its own entity store) and dominates
// the log; audit records for it are demoted to Debug so the Info stream stays
// the network attack surface. An unparseable address is treated as non-loopback
// so we err toward recording at the higher level.
func isLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// logCertAuth emits a single durable audit record for a successful cert-method
// authentication.
//
// Note on scope: the listener uses tls.VerifyClientCertIfGiven, so any client
// cert reaching this point has already chained to the cluster CA and a forged
// cert is rejected during the TLS handshake, before any authenticator runs.
// This record therefore only ever captures legitimate cert auth (in practice,
// internal component mTLS) — it can never see an attacker. It exists so that
// legitimate cert use is attributable after the fact, not as a tripwire; the
// per-request access log (logAccess) is the auth-method-agnostic audit trail
// that survives a bypass on any path. Kept deliberately to one line.
//
// Logged at Info for network peers and Debug for loopback (same-host internal
// component mTLS), so the Info stream isn't drowned by the coordinator's own
// services authenticating to each other. See isLoopbackAddr.
func logCertAuth(ctx context.Context, log *slog.Logger, r *http.Request) {
	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		return
	}
	cert := r.TLS.PeerCertificates[0]

	level := slog.LevelInfo
	if isLoopbackAddr(r.RemoteAddr) {
		level = slog.LevelDebug
	}

	log.Log(ctx, level, "cert auth",
		"remote", r.RemoteAddr,
		"subject", cert.Subject.String(),
		"issuer", cert.Issuer.String(),
		"serial", cert.SerialNumber.String(),
		"fingerprint", certFingerprint(cert),
		"verified", len(r.TLS.VerifiedChains) > 0,
		"chains", len(r.TLS.VerifiedChains),
	)
}

// logAccess emits a per-request RPC access record for a non-public method.
// It is intentionally auth-method-agnostic: every line carries the source IP,
// the authenticated subject, and the auth method, so the trail stays useful no
// matter which auth path a caller (or a future bypass) came in on. outcome is
// "ok" for an authorized dispatch, or "unauthorized"/"forbidden" for a rejected
// one; non-ok outcomes log at Warn so denials surface without a level filter.
//
// Callers invoke this only for non-public methods. Public methods (e.g. health
// checks and runner Join, which are public precisely because they carry no
// caller identity) are intentionally left out of the audit trail rather than
// logging identity-less lines for them.
func logAccess(ctx context.Context, log *slog.Logger, r *http.Request, mm Method, outcome string, extra ...any) {
	var subject, method string
	if id := IdentityFromContext(ctx); id != nil {
		subject = id.Subject
		method = string(id.Method)
	}

	// Denials are security-relevant wherever they come from, so they always
	// surface at Warn. A successful call from a network peer is the audit
	// signal we care about (the MIR-1323 exfil was a remote read) and logs at
	// Info; the same call over loopback is trusted same-host component chatter
	// and drops to Debug. See isLoopbackAddr.
	level := slog.LevelInfo
	switch {
	case outcome != "ok":
		level = slog.LevelWarn
	case isLoopbackAddr(r.RemoteAddr):
		level = slog.LevelDebug
	}

	attrs := []any{
		"remote", r.RemoteAddr,
		"rpc", mm.InterfaceName + "." + mm.Name,
		"subject", subject,
		"auth_method", method,
		"outcome", outcome,
	}
	attrs = append(attrs, extra...)

	log.Log(ctx, level, "rpc access", attrs...)
}

// logAuthReject records a rejected capability-signature request (a bad, expired,
// or forged rpc-signature on the ed25519 capability path in authRequest). These
// rejections happen before any Identity is established, so they don't flow
// through logAccess, but a forged or replayed capability is exactly the kind of
// event the audit trail exists to capture, so it must not be confined to the
// general log. Always Warn, since every one of these is a failed auth attempt.
func logAuthReject(log *slog.Logger, r *http.Request, oid OID, reason string) {
	log.Warn("auth rejected",
		"remote", r.RemoteAddr,
		"oid", string(oid),
		"reason", reason,
	)
}

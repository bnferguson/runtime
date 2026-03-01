package httpingress

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"miren.dev/runtime/api/app"
	"miren.dev/runtime/api/core/core_v1alpha"
	"miren.dev/runtime/api/entityserver/entityserver_v1alpha"
	"miren.dev/runtime/api/httpingress/httpingress_v1alpha"
	"miren.dev/runtime/api/ingress"
	"miren.dev/runtime/components/activator"
	"miren.dev/runtime/metrics"
	"miren.dev/runtime/observability"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/httputil"
	"miren.dev/runtime/pkg/oidc"
	"miren.dev/runtime/pkg/rpc"
)

// idleTimeoutConn wraps a net.Conn and sets a read deadline before each
// Read call. If no data arrives within the idle timeout, the read fails
// with a timeout error. Each successful read resets the deadline, so
// active streams (SSE, WebSocket, chunked) are unaffected as long as
// data keeps flowing.
type idleTimeoutConn struct {
	net.Conn
	idleTimeout time.Duration
}

func (c *idleTimeoutConn) Read(p []byte) (int, error) {
	c.SetReadDeadline(time.Now().Add(c.idleTimeout))
	return c.Conn.Read(p)
}

var httpingressTracer = otel.Tracer("miren.dev/runtime/httpingress")

const (
	timeoutMessage = "Request timeout"
	// leaseAcquisitionTimeout is the maximum time to wait for sandbox boot
	// This is longer than request timeout to prevent dangling resources
	leaseAcquisitionTimeout = 2 * time.Minute
	// minLeaseTTL is the minimum time a lease is kept in cache after its last
	// use before it becomes eligible for eviction. This prevents low-traffic
	// apps from having their leases evicted on every 30s tick, which would
	// force every request through the full entity store + activator pipeline.
	minLeaseTTL = 5 * time.Minute
)

type IngressConfig struct {
	RequestTimeout time.Duration
	DataPath       string
}

type Server struct {
	Log *slog.Logger

	config        IngressConfig
	rpcClient     rpc.Client
	eac           *entityserver_v1alpha.EntityAccessClient
	ingressClient *ingress.Client
	appClient     *app.Client

	aa        activator.AppActivator
	transport http.RoundTripper

	httpMetrics *metrics.HTTPMetrics
	logWriter   observability.LogWriter

	mu   sync.Mutex
	apps map[string]*appUsage

	oidcSessionManager *oidc.SessionManager
	oidcMu             sync.RWMutex
	oidcHandlers       map[string]*oidcHandler
}

type appUsage struct {
	leases []*lease
}

func NewServer(
	ctx context.Context,
	log *slog.Logger,
	config IngressConfig,
	rpcClient rpc.Client,
	aa activator.AppActivator,
	httpMetrics *metrics.HTTPMetrics,
	logWriter observability.LogWriter,
) *Server {
	eac := entityserver_v1alpha.NewEntityAccessClient(rpcClient)

	if config.RequestTimeout <= 0 {
		if config.RequestTimeout < 0 {
			log.Warn("invalid request timeout; using default 60s", "configured", config.RequestTimeout)
		}
		config.RequestTimeout = 60 * time.Second
	}

	baseTransport := http.DefaultTransport.(*http.Transport).Clone()
	baseTransport.ResponseHeaderTimeout = config.RequestTimeout
	baseTransport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, err := (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext(ctx, network, addr)
		if err != nil {
			return nil, err
		}
		return &idleTimeoutConn{Conn: conn, idleTimeout: config.RequestTimeout}, nil
	}

	var signingKey []byte
	if config.DataPath != "" {
		var err error
		signingKey, err = loadOrGenerateSigningKey(config.DataPath)
		if err != nil {
			log.Error("failed to load OIDC signing key, sessions will not survive restarts", "error", err)
		}
	}

	serv := &Server{
		Log:                log.With("module", "httpingress"),
		config:             config,
		rpcClient:          rpcClient,
		eac:                eac,
		ingressClient:      ingress.NewClient(log, rpcClient),
		appClient:          app.NewClient(log, rpcClient),
		aa:                 aa,
		transport:          baseTransport,
		httpMetrics:        httpMetrics,
		logWriter:          logWriter,
		apps:               make(map[string]*appUsage),
		oidcSessionManager: oidc.NewSessionManager(false, "", signingKey),
		oidcHandlers:       make(map[string]*oidcHandler),
	}

	if httpMetrics == nil {
		serv.Log.Warn("HTTPMetrics is nil in httpingress")
	} else {
		serv.Log.Debug("HTTPMetrics initialized in httpingress")
	}

	go serv.checkLeases(ctx)

	return serv
}

func (h *Server) checkLeases(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.expireLeases(ctx)
		}
	}
}

func (h *Server) expireLeases(ctx context.Context) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for app, ar := range h.apps {
		var newLeases []*lease

		for i, l := range ar.leases {
			if l.Uses == 0 && time.Since(l.LastUsed) > minLeaseTTL {
				h.Log.Debug("expiring lease", "app", app, "url", l.Lease.URL)
				h.aa.ReleaseLease(ctx, l.Lease)
				continue
			}

			// Renew all retained leases — both active (Uses > 0) and idle
			// but within TTL. This validates with the activator that the
			// sandbox is still alive, so we never serve a stale route.
			h.Log.Debug("renewing lease", "app", app, "url", l.Lease.URL, "uses", l.Uses)
			lease, err := h.aa.RenewLease(ctx, l.Lease)
			if err != nil {
				h.Log.Error("error renewing lease", "error", err, "app", app, "url", l.Lease.URL)
				h.aa.ReleaseLease(ctx, l.Lease)
				continue
			}

			ar.leases[i].Lease = lease
			newLeases = append(newLeases, ar.leases[i])
		}

		if len(newLeases) == 0 {
			h.Log.Debug("No application leases left", "app", app)
			delete(h.apps, app)
		} else {
			ar.leases = newLeases
		}
	}
}

func (h *Server) DeriveApp(host string) (string, bool) {
	if host == "" {
		return "", false
	}

	_, err := netip.ParseAddr(host)
	if err == nil {
		return "", false
	}

	if app, _, ok := strings.Cut(host, "."); ok {
		return app, true
	}

	// Ok, it's JUST a name, so let's try it.
	return host, true
}

type lease struct {
	Uses     int
	LastUsed time.Time
	Lease    *activator.Lease
}

func (h *Server) retainLease(ctx context.Context, app string, l *activator.Lease) *lease {
	h.mu.Lock()
	defer h.mu.Unlock()

	ll := &lease{
		Lease:    l,
		Uses:     1,
		LastUsed: time.Now(),
	}

	ar, ok := h.apps[app]
	if ok {
		ar.leases = append(ar.leases, ll)
	} else {
		h.apps[app] = &appUsage{
			leases: []*lease{ll},
		}
	}

	return ll
}

func (h *Server) useLease(ctx context.Context, app string) (*lease, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	ar, ok := h.apps[app]
	if !ok {
		return nil, nil
	}

	if len(ar.leases) == 0 {
		return nil, nil
	}

	for _, l := range ar.leases {
		if l.Uses <= l.Lease.Size {
			l.Uses++
			l.LastUsed = time.Now()
			return l, nil
		}
	}

	return nil, nil
}

func (h *Server) releaseLease(ctx context.Context, lease *lease) {
	h.mu.Lock()
	defer h.mu.Unlock()

	lease.Uses--
}

// invalidateLease removes a lease from the cache entirely.
// This is called when a proxy error indicates the sandbox is dead.
// The lease is removed immediately rather than waiting for expireLeases.
func (h *Server) invalidateLease(ctx context.Context, app string, lease *lease) {
	h.mu.Lock()
	defer h.mu.Unlock()

	ar, ok := h.apps[app]
	if !ok {
		return
	}

	for i, l := range ar.leases {
		if l == lease {
			h.Log.Info("invalidating stale lease due to proxy error", "app", app, "url", lease.Lease.URL)
			// Release the lease back to the activator
			h.aa.ReleaseLease(ctx, l.Lease)
			// Remove from our cache
			ar.leases = append(ar.leases[:i], ar.leases[i+1:]...)
			break
		}
	}

	if len(ar.leases) == 0 {
		delete(h.apps, app)
	}
}

func (h *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	h.handleRequest(w, req)
}

func (h *Server) handleRequest(w http.ResponseWriter, req *http.Request) {
	defer func() {
		if r := recover(); r != nil {
			h.Log.Error("panic in request handler",
				"error", r,
				"stack", string(debug.Stack()),
				"method", req.Method,
				"path", req.URL.Path,
				"host", req.Host,
			)
			panic(r)
		}
	}()

	// Extract inbound trace context (traceparent/tracestate headers)
	ctx := otel.GetTextMapPropagator().Extract(req.Context(), propagation.HeaderCarrier(req.Header))

	ctx, span := httpingressTracer.Start(ctx, "httpingress",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("http.method", req.Method),
			attribute.String("url.path", req.URL.Path),
			attribute.String("server.address", req.Host),
		))
	defer span.End()
	req = req.WithContext(ctx)

	// Handle Miren server health check endpoint before routing
	// Using .well-known per RFC 8615 to avoid collision with app routes
	if req.URL.Path == "/.well-known/miren/health" {
		h.handleHealth(w, req)
		return
	}

	start := time.Now()

	var appName string
	var statusCode int
	var bytesWritten int

	rw := &responseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK, // default if not explicitly set
	}

	h.serveHTTPWithMetrics(rw, req, &appName)

	statusCode = rw.statusCode
	bytesWritten = rw.bytesWritten

	span.SetAttributes(
		attribute.Int("http.response.status_code", statusCode),
		attribute.String("miren.app.name", appName),
	)

	if h.httpMetrics != nil {
		if appName == "" {
			appName = "unknown"
		}

		duration := time.Since(start)
		// Use background context to ensure metrics are recorded even if request context is cancelled
		metricsCtx := context.Background()
		err := h.httpMetrics.RecordRequest(metricsCtx, metrics.HTTPRequest{
			Timestamp:    start,
			App:          appName,
			Method:       req.Method,
			Path:         req.URL.Path,
			StatusCode:   statusCode,
			DurationMs:   duration.Milliseconds(),
			ResponseSize: int64(bytesWritten),
		})
		if err != nil {
			h.Log.Error("Failed to record HTTP request", "error", err, "app", appName)
		}
	}
}

func (h *Server) serveHTTPWithMetrics(w http.ResponseWriter, req *http.Request, appName *string) {
	// Block public access to the admin endpoint - it's only accessible via internal requests
	if req.URL.Path == "/.well-known/miren/admin" {
		http.NotFound(w, req)
		return
	}

	onlyHost, _, err := net.SplitHostPort(req.Host)
	if err != nil {
		onlyHost = req.Host
	}

	ctx := req.Context()

	// CRITICAL TO KNOW
	// The context on requset is closed automaticaly when the client on the over side closes!
	// So if you're doing critical work, don't use this context! Use a separate context and ping
	// this one to figure out if you should continue with your critical work or clean up.

	// Use ingress client to lookup route
	route, err := h.ingressClient.Lookup(ctx, onlyHost)
	if err != nil {
		h.Log.Error("error looking up http route", "error", err, "host", onlyHost)
		http.Error(w, fmt.Sprintf("error looking up http route: %s", onlyHost), http.StatusInternalServerError)
		return
	}

	var targetAppId entity.Id
	var routeType string

	if route != nil {
		// Use the http route if found
		targetAppId = route.App
		routeType = "route"
		h.Log.Debug("using http route", "host", onlyHost, "app", targetAppId)

		// Check if OIDC authentication is required
		if !entity.Empty(route.OidcProvider) {
			// Wrap the request handler with OIDC middleware
			oidcWrapped := h.oidcMiddleware(route, func(w http.ResponseWriter, r *http.Request) {
				// Continue with normal request handling after auth
				h.serveAuthenticatedRequest(w, r, targetAppId, routeType, appName)
			})
			oidcWrapped(w, req)
			return
		}
	} else {
		// No route found, try to find a default route
		h.Log.Debug("no http route found, checking for default route", "host", onlyHost)

		defaultRoute, err := h.ingressClient.LookupDefault(ctx)
		if err != nil {
			h.Log.Error("error looking up default route", "error", err)
			http.Error(w, fmt.Sprintf("no http route found: %s", onlyHost), http.StatusNotFound)
			return
		}

		if defaultRoute == nil {
			h.Log.Debug("no default route found", "host", onlyHost)
			http.Error(w, fmt.Sprintf("no http route found: %s", onlyHost), http.StatusNotFound)
			return
		}

		// Use the default route
		targetAppId = defaultRoute.App
		routeType = "default"
		h.Log.Debug("using default route", "host", onlyHost, "app", targetAppId)

		// Check if OIDC authentication is required for default route
		if !entity.Empty(defaultRoute.OidcProvider) {
			// Update route reference
			route = defaultRoute
			// Wrap with OIDC middleware
			oidcWrapped := h.oidcMiddleware(route, func(w http.ResponseWriter, r *http.Request) {
				h.serveAuthenticatedRequest(w, r, targetAppId, routeType, appName)
			})
			oidcWrapped(w, req)
			return
		}
	}

	// Continue with normal request handling
	h.serveAuthenticatedRequest(w, req, targetAppId, routeType, appName)
}

// serveAuthenticatedRequest handles the request after authentication (if any)
func (h *Server) serveAuthenticatedRequest(w http.ResponseWriter, req *http.Request, targetAppId entity.Id, routeType string, appName *string) {
	ctx := req.Context()

	// Get app details first to have the name for metrics
	gr, err := h.eac.Get(ctx, targetAppId.String())
	if err != nil {
		h.Log.Error("error looking up application", "error", err, "app", targetAppId)
		http.Error(w, fmt.Sprintf("error looking up application: %s", targetAppId), http.StatusInternalServerError)
		return
	}

	var app core_v1alpha.App
	app.Decode(gr.Entity().Entity())

	var appMD core_v1alpha.Metadata
	appMD.Decode(gr.Entity().Entity())

	// Store app name for metrics
	*appName = appMD.Name

	ctx, leaseSpan := httpingressTracer.Start(ctx, "httpingress.lease",
		trace.WithAttributes(
			attribute.String("miren.app.id", targetAppId.String()),
			attribute.String("miren.app.name", *appName),
			attribute.String("miren.route.type", routeType),
		))
	defer leaseSpan.End()

	// Retry loop: if a cached lease fails with a connection error (stale sandbox),
	// invalidate it and retry once — the next iteration acquires a fresh lease.
	const maxRetries = 1
	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Try to use a cached lease
		curLease, err := h.useLease(ctx, targetAppId.String())
		if err != nil {
			h.Log.Error("error taking lease", "error", err, "app", targetAppId)
			http.Error(w, fmt.Sprintf("error taking lease: %s", targetAppId), http.StatusInternalServerError)
			return
		}

		if curLease != nil {
			leaseSpan.SetAttributes(
				attribute.Bool("miren.lease.cached", true),
				attribute.String("miren.lease.url", curLease.Lease.URL),
			)
			req = req.WithContext(ctx)
			// On non-final attempts, suppress error response so we can retry
			writeErr := attempt == maxRetries
			err = h.proxyToLease(w, req, curLease.Lease.URL, targetAppId.String(), *appName, writeErr)
			if err != nil && isProxyConnectionError(err) {
				// Cached lease pointed at a dead sandbox — invalidate and retry
				h.invalidateLease(context.Background(), targetAppId.String(), curLease)
				h.Log.Warn("stale lease, retrying with fresh lease",
					"stale_url", curLease.Lease.URL,
					"attempt", attempt,
					"app", targetAppId)
				continue
			}
			if err != nil {
				h.invalidateLease(context.Background(), targetAppId.String(), curLease)
			} else {
				h.releaseLease(ctx, curLease)
			}
			return
		}

		// No cached lease — acquire a fresh one
		leaseSpan.SetAttributes(attribute.Bool("miren.lease.cached", false))

		if app.ActiveVersion == "" {
			h.Log.Debug("no active version for app", "app", targetAppId)
			http.Error(w, fmt.Sprintf("no active version for app: %s", targetAppId), http.StatusNotFound)
			return
		}

		vr, err := h.eac.Get(ctx, app.ActiveVersion.String())
		if err != nil {
			h.Log.Error("error looking up application version", "error", err, "version", app.ActiveVersion)
			http.Error(w, fmt.Sprintf("error looking up application version: %s", app.ActiveVersion), http.StatusInternalServerError)
			return
		}

		var av core_v1alpha.AppVersion
		av.Decode(vr.Entity().Entity())

		leaseSpan.SetAttributes(
			attribute.String("miren.app.version", app.ActiveVersion.String()),
		)

		// Give lease acquisition a generous timeout to complete sandbox boot
		// even if the client request times out. This prevents dangling resources.
		actContext, actCancel := context.WithTimeout(context.Background(), leaseAcquisitionTimeout)
		defer actCancel()

		actLease, err := h.aa.AcquireLease(actContext, &av, "web")
		if err != nil {
			if errors.Is(err, activator.ErrSandboxDiedEarly) {
				h.Log.Error("sandbox died early while acquiring lease", "error", err, "app", targetAppId)
				http.Error(w, fmt.Sprintf("The application %s failed to boot. Please check the applications logs.\n", targetAppId), http.StatusRequestTimeout)
			} else {
				h.Log.Error("error acquiring lease", "error", err, "app", targetAppId)
				http.Error(w, fmt.Sprintf("error acquiring lease: %s", targetAppId), http.StatusInternalServerError)
			}
			return
		}

		if actLease == nil {
			h.Log.Debug("no lease available for app", "app", targetAppId)
			http.Error(w, fmt.Sprintf("no lease available for app: %s", targetAppId), http.StatusServiceUnavailable)
			return
		}

		leaseSpan.SetAttributes(attribute.String("miren.lease.url", actLease.URL))

		localLease := h.retainLease(ctx, targetAppId.String(), actLease)

		req = req.WithContext(ctx)
		// Fresh lease — always write error response (no retry on fresh lease failure)
		err = h.proxyToLease(w, req, actLease.URL, targetAppId.String(), *appName, true)
		if err != nil {
			// Connection error on a fresh lease - the sandbox may have died
			// between lease acquisition and proxy. Invalidate immediately.
			h.invalidateLease(context.Background(), targetAppId.String(), localLease)
		} else {
			h.releaseLease(ctx, localLease)
		}
		return
	}
}

func (h *Server) logRequestFromStats(appEntityID, appName string, stats httputil.ProxyStats) {
	if h.logWriter == nil {
		return
	}

	// Build full path with query string
	path := stats.RequestPath
	if stats.RequestQuery != "" {
		path = path + "?" + stats.RequestQuery
	}

	// Format in Heroku logfmt style with response data
	logMsg := fmt.Sprintf("method=%s path=\"%s\" host=%s source_ip=%s body=%d status=%d response=%d duration_ms=%d",
		stats.RequestMethod, path, stats.RequestHost, stats.RemoteAddr, stats.ContentLength,
		stats.StatusCode, stats.ResponseBytes, stats.Duration.Milliseconds())

	err := h.logWriter.WriteEntry(appEntityID, observability.LogEntry{
		Timestamp: time.Now(),
		Stream:    observability.UserOOB,
		Body:      logMsg,
		Attributes: map[string]string{
			"source": "router",
			"method": stats.RequestMethod,
			"path":   stats.RequestPath,
			"host":   stats.RequestHost,
		},
	})
	if err != nil {
		h.Log.Error("failed to write request log entry", "error", err, "app", appName)
	}
}

// proxyToLease proxies the request to the target URL and returns any connection error.
// If the proxy fails with a connection error (connection refused, no route to host, etc.),
// it returns the error so the caller can invalidate the lease.
//
// When writeErrorResponse is false and a connection error occurs, the ErrorHandler
// captures the error but does NOT write to the ResponseWriter, allowing the caller
// to retry with a fresh lease. This is safe because connection errors happen during
// TCP dial, before any response bytes are sent.
func (h *Server) proxyToLease(w http.ResponseWriter, req *http.Request, targetURL, appEntityID, appName string, writeErrorResponse bool) error {
	targetParsed, err := url.Parse(targetURL)
	if err != nil {
		h.Log.Error("failed to parse target URL", "error", err, "url", targetURL)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return nil // Not a connection error, don't invalidate
	}

	// Capture any proxy error for the caller
	var proxyErr error

	proxy := &httputil.ReverseProxy{
		Transport: h.transport,
		Director: func(outReq *http.Request) {
			outReq.URL.Scheme = targetParsed.Scheme
			outReq.URL.Host = targetParsed.Host

			// Set X-Forwarded-Proto to indicate the original protocol
			if req.TLS == nil {
				outReq.Header.Set("X-Forwarded-Proto", "http")
			} else {
				outReq.Header.Set("X-Forwarded-Proto", "https")
			}

			outReq.Header.Set("X-Forwarded-Host", req.Host)

			// Mark this as a public request (strip any client-provided value first)
			outReq.Header.Set("X-Miren-Access", "public")

			// Inject trace context so user apps can continue the trace
			otel.GetTextMapPropagator().Inject(outReq.Context(), propagation.HeaderCarrier(outReq.Header))
		},
		ErrorHandler: func(rw http.ResponseWriter, r *http.Request, err error) {
			proxyErr = err
			if !writeErrorResponse && isProxyConnectionError(err) {
				h.Log.Warn("proxy connection error to sandbox (will retry)", "error", err, "url", targetURL, "app", appName)
				return
			}
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				h.Log.Warn("request timeout", "url", targetURL, "app", appName)
				http.Error(rw, timeoutMessage, http.StatusServiceUnavailable)
				return
			}
			if isProxyConnectionError(err) {
				h.Log.Warn("proxy connection error to sandbox", "error", err, "url", targetURL, "app", appName)
			} else {
				h.Log.Error("proxy error", "error", err, "url", targetURL, "app", appName)
			}
			rw.WriteHeader(http.StatusBadGateway)
		},
		Callback: func(stats httputil.ProxyStats) {
			h.logRequestFromStats(appEntityID, appName, stats)
		},
	}

	proxy.ServeHTTP(w, req)

	// Return connection errors so the caller can invalidate the lease
	if proxyErr != nil && isProxyConnectionError(proxyErr) {
		return proxyErr
	}
	return nil
}

/*
func (h *LeaseHTTP) extractEndpoint(ctx context.Context, container containerd.Container) (discovery.Endpoint, error) {
	labels, err := container.Labels(ctx)
	if err == nil {
		if host, ok := labels[httpHostLabel]; ok {
			h.Log.Info("http endpoint found", "id", container.ID(), "host", host)
			var ep discovery.Endpoint

			if dir, ok := labels[staticDirLabel]; ok {
				h.Log.Info("using local container endpoint for static_dir", "id", container.ID())
				ep = &discovery.LocalContainerEndpoint{
					Log: h.Log,
					HTTP: discovery.HTTPEndpoint{
						Host: "http://" + host,
					},
					Client:    h.CC,
					Namespace: h.Namespace,
					Dir:       dir,
					Id:        container.ID(),
				}
			} else {
				ep = &discovery.HTTPEndpoint{
					Host: "http://" + host,
				}
			}

			return ep, nil
		}
	}

	return nil, fmt.Errorf("unable to derive endpoint")
}
*/

// responseWriter wraps http.ResponseWriter to capture status code and response size
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += n
	return n, err
}

// Unwrap returns the underlying ResponseWriter for middleware compatibility
func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// isProxyConnectionError checks if an error indicates the backend is unreachable.
// This includes connection refused, no route to host, and similar network failures
// that indicate the sandbox is dead or gone.
func isProxyConnectionError(err error) bool {
	if err == nil {
		return false
	}

	// Check for net.OpError which wraps most connection failures
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		// Check for syscall errors (connection refused, no route to host, etc.)
		var syscallErr *os.SyscallError
		if errors.As(opErr.Err, &syscallErr) {
			if errno, ok := syscallErr.Err.(syscall.Errno); ok {
				switch errno {
				case syscall.ECONNREFUSED: // connection refused
					return true
				case syscall.EHOSTUNREACH: // no route to host
					return true
				case syscall.ENETUNREACH: // network unreachable
					return true
				case syscall.ECONNRESET: // connection reset by peer
					return true
				case syscall.ECONNABORTED: // connection aborted
					return true
				}
			}
		}
	}

	return false
}

// HealthResponse represents the JSON response for /health endpoint
type HealthResponse struct {
	Status string                 `json:"status"`
	Checks map[string]HealthCheck `json:"checks"`
}

// HealthCheck represents a single component health check
type HealthCheck struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// handleHealth responds to /.well-known/miren/health endpoint with component health checks
// Uses .well-known URI per RFC 8615 to avoid collision with application routes
func (h *Server) handleHealth(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	response := HealthResponse{
		Status: "healthy",
		Checks: make(map[string]HealthCheck),
	}

	// Check etcd connection by listing apps (lightweight query)
	if h.eac != nil {
		_, err := h.eac.List(ctx, entity.Ref(entity.EntityKind, core_v1alpha.KindApp))
		if err != nil {
			response.Status = "unhealthy"
			response.Checks["etcd"] = HealthCheck{
				Status: "unhealthy",
				Error:  err.Error(),
			}
		} else {
			response.Checks["etcd"] = HealthCheck{
				Status: "healthy",
			}
		}
	}

	// Set response headers and status
	w.Header().Set("Content-Type", "application/json")
	if response.Status == "healthy" {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	// Encode response as JSON
	json.NewEncoder(w).Encode(response)
}

// DoRequest handles internal HTTP requests to app sandboxes. This method reuses
// the same lease management infrastructure as the HTTP proxy but is called
// directly via Go method invocation rather than through the HTTP listener.
func (h *Server) DoRequest(ctx context.Context, req *httpingress_v1alpha.InternalHttpRequest) (*httpingress_v1alpha.InternalHttpResponse, error) {
	startTime := time.Now()
	resp := &httpingress_v1alpha.InternalHttpResponse{}

	// Validate required fields
	appId := req.AppId()
	if appId == "" {
		resp.SetError("app_id is required")
		return resp, nil
	}

	method := req.Method()
	if method == "" {
		method = "GET"
	}

	path := req.Path()
	if path == "" {
		path = "/"
	}

	service := req.Service()
	if service == "" {
		service = "web"
	}

	// Apply timeout if specified
	if req.TimeoutMs() > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(req.TimeoutMs())*time.Millisecond)
		defer cancel()
	}

	// Look up the app
	gr, err := h.eac.Get(ctx, appId)
	if err != nil {
		resp.SetError(fmt.Sprintf("error looking up app: %v", err))
		return resp, nil
	}

	var appEntity core_v1alpha.App
	appEntity.Decode(gr.Entity().Entity())

	if appEntity.ActiveVersion == "" {
		resp.SetError("no active version for app")
		return resp, nil
	}

	// Look up the app version
	vr, err := h.eac.Get(ctx, appEntity.ActiveVersion.String())
	if err != nil {
		resp.SetError(fmt.Sprintf("error looking up app version: %v", err))
		return resp, nil
	}

	var av core_v1alpha.AppVersion
	av.Decode(vr.Entity().Entity())

	// Try to use an existing lease, with retry on stale cached lease
	curLease, err := h.useLease(ctx, appId)
	if err != nil {
		resp.SetError(fmt.Sprintf("error taking lease: %v", err))
		return resp, nil
	}

	// If we got a cached lease, try it — but retry with a fresh one on connection error
	if curLease != nil {
		httpResp, err := h.executeInternalRequest(ctx, curLease, req, method, path, appId)
		if err != nil && isProxyConnectionError(err) {
			// Stale cached lease — invalidate and fall through to acquire fresh
			h.invalidateLease(context.Background(), appId, curLease)
			h.Log.Warn("stale lease on internal request, retrying with fresh lease",
				"stale_url", curLease.Lease.URL, "app", appId)
			curLease = nil
		} else if err != nil {
			h.releaseLease(ctx, curLease)
			resp.SetError(fmt.Sprintf("request failed: %v", err))
			return resp, nil
		} else {
			defer httpResp.Body.Close()
			h.releaseLease(ctx, curLease)
			return h.buildInternalResponse(resp, httpResp, appId, method, path, startTime)
		}
	}

	// No cached lease (or stale one was invalidated) — acquire fresh
	actContext, actCancel := context.WithTimeout(context.Background(), leaseAcquisitionTimeout)
	defer actCancel()

	actLease, err := h.aa.AcquireLease(actContext, &av, service)
	if err != nil {
		if errors.Is(err, activator.ErrSandboxDiedEarly) {
			resp.SetError("sandbox died early while acquiring lease")
		} else {
			resp.SetError(fmt.Sprintf("error acquiring lease: %v", err))
		}
		return resp, nil
	}

	if actLease == nil {
		resp.SetError("no lease available for app")
		return resp, nil
	}

	curLease = h.retainLease(ctx, appId, actLease)

	// Execute the HTTP request with fresh lease (no retry)
	httpResp, err := h.executeInternalRequest(ctx, curLease, req, method, path, appId)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			h.releaseLease(ctx, curLease)
		} else if isProxyConnectionError(err) {
			h.invalidateLease(context.Background(), appId, curLease)
		} else {
			h.releaseLease(ctx, curLease)
		}
		resp.SetError(fmt.Sprintf("request failed: %v", err))
		return resp, nil
	}
	defer httpResp.Body.Close()

	// Release the lease on success
	h.releaseLease(ctx, curLease)

	return h.buildInternalResponse(resp, httpResp, appId, method, path, startTime)
}

// buildInternalResponse populates the InternalHttpResponse from the HTTP response.
func (h *Server) buildInternalResponse(
	resp *httpingress_v1alpha.InternalHttpResponse,
	httpResp *http.Response,
	appId, method, path string,
	startTime time.Time,
) (*httpingress_v1alpha.InternalHttpResponse, error) {
	resp.SetStatusCode(int32(httpResp.StatusCode))

	// Copy response headers
	var respHeaders []*httpingress_v1alpha.HttpHeader
	for key, values := range httpResp.Header {
		for _, value := range values {
			hdr := &httpingress_v1alpha.HttpHeader{}
			hdr.SetKey(key)
			hdr.SetValue(value)
			respHeaders = append(respHeaders, hdr)
		}
	}
	if len(respHeaders) > 0 {
		resp.SetHeaders(respHeaders)
	}

	// Read response body
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.SetError(fmt.Sprintf("error reading response body: %v", err))
		return resp, nil
	}
	resp.SetBody(&body)

	// Log the internal request
	h.logInternalRequest(appId, method, path, int(httpResp.StatusCode), len(body), startTime)

	return resp, nil
}

// logInternalRequest logs an internal HTTP request in a format similar to public requests
func (h *Server) logInternalRequest(appEntityID, method, path string, statusCode, responseBytes int, startTime time.Time) {
	if h.logWriter == nil {
		return
	}

	duration := time.Since(startTime)

	// Format in Heroku logfmt style, indicating this is an internal request
	logMsg := fmt.Sprintf("method=%s path=\"%s\" access=internal status=%d response=%d duration_ms=%d",
		method, path, statusCode, responseBytes, duration.Milliseconds())

	err := h.logWriter.WriteEntry(appEntityID, observability.LogEntry{
		Timestamp: time.Now(),
		Stream:    observability.UserOOB,
		Body:      logMsg,
		Attributes: map[string]string{
			"source": "router",
			"access": "internal",
			"method": method,
			"path":   path,
		},
	})
	if err != nil {
		h.Log.Error("failed to write internal request log entry", "error", err, "app", appEntityID)
	}
}

// executeInternalRequest performs the actual HTTP request to the sandbox
func (h *Server) executeInternalRequest(
	ctx context.Context,
	lease *lease,
	req *httpingress_v1alpha.InternalHttpRequest,
	method, path, appId string,
) (*http.Response, error) {
	targetURL := lease.Lease.URL + path

	var bodyReader io.Reader
	if req.HasBody() && req.Body() != nil {
		bodyReader = bytes.NewReader(*req.Body())
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, targetURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Copy headers from the request
	for _, hdr := range req.Headers() {
		httpReq.Header.Add(hdr.Key(), hdr.Value())
	}

	// Mark this as an internal request
	httpReq.Header.Set("X-Miren-Access", "internal")

	// Execute the request
	client := &http.Client{
		Timeout: h.config.RequestTimeout,
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		if isProxyConnectionError(err) {
			return nil, err
		}
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

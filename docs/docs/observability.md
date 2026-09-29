---
title: Observability
description: Managed application metrics and OpenTelemetry distributed tracing in Miren.
keywords: [observability, metrics, prometheus, remote write, tracing, opentelemetry, spans]
---

# Observability

Miren can scrape a Prometheus-compatible metrics endpoint from every running
replica of an application service. It also instruments the request lifecycle
with [OpenTelemetry](https://opentelemetry.io/) distributed tracing.

## Managed application metrics

First, configure a remote-write destination for your cluster in
[`server.toml`](./server-config.md#managed-app-metrics). An application then opts a
service in through `.miren/app.toml`:

```toml
[services.web.metrics]
enabled = true
path = "/metrics"
port = 3000
interval = "30s"
```

Miren scrapes each running sandbox over its private overlay address and adds
`miren_app`, `miren_app_version`, `miren_service`, `miren_sandbox`,
`miren_runner`, and `miren_cluster` labels. Runtime labels take precedence over
labels emitted by the application, so a workload cannot impersonate another
replica or cluster. Registered clusters use their Miren Cloud cluster ID for
`miren_cluster`; other clusters use the configured cluster name.

The metrics path is private by default. Public requests to that exact path get
a 404 even when the app has a public route. Set `public = true` only when you
intend the app's normal public ingress and authentication policy to apply.

Miren sends these samples directly to the configured destination with a
short-lived workload identity token. It does not retain a second copy in the
cluster's embedded VictoriaMetrics instance.

The coordinator owns both scraping and remote write, so failures appear in the
managed scraper's system logs:

```bash
miren logs system vmagent --last 15m
```

This includes endpoint errors, invalid or oversized responses, authentication
failures, and delivery retries. `miren logs app` includes only output written by
the application itself.

### Pushing metrics

A service with nothing to scrape, like a background worker with no HTTP
listener, can push its metrics instead. No `app.toml` setting is needed. On a
cluster with a remote-write destination, every sandbox gets two URLs:

| Variable | Scope |
| --- | --- |
| `MIREN_METRICS_PUSH_URL` | Sandbox: the same labels a scrape of this sandbox would carry |
| `MIREN_METRICS_SHARED_PUSH_URL` | App: `miren_app`, `miren_service` and `miren_cluster` only. See [shared state](#metrics-from-shared-state) |

Each URL is a [Pushgateway](https://github.com/prometheus/pushgateway) base, so
the push helpers in Prometheus client libraries work as they are. Authenticate
with the sandbox's `MIREN_IDENTITY_TOKEN_SECRET`, either as a Bearer token or as
the password of Basic auth:

```go
pusher := push.New(os.Getenv("MIREN_METRICS_PUSH_URL"), "worker").
	Header(http.Header{"Authorization": {"Bearer " + os.Getenv("MIREN_IDENTITY_TOKEN_SECRET")}}).
	Gatherer(prometheus.DefaultGatherer)

// Push on an interval, like a scrape would read.
for range time.Tick(30 * time.Second) {
	if err := pusher.Push(); err != nil {
		log.Printf("pushing metrics: %v", err)
	}
}
```

```python
from prometheus_client import push_to_gateway
from prometheus_client.exposition import basic_auth_handler

def auth(url, method, timeout, headers, data):
    return basic_auth_handler(url, method, timeout, headers, data,
                              "", os.environ["MIREN_IDENTITY_TOKEN_SECRET"])

push_to_gateway(os.environ["MIREN_METRICS_PUSH_URL"], job="worker",
                registry=registry, handler=auth)
```

Pushed samples travel the same pipeline as scraped ones, so they need the same
remote-write destination. On a cluster without one, none of these variables are
set, so an app can take their presence to mean pushing works.

Each sandbox may push about once a second, with bursts of up to ten. Pushing
faster gets a `429` with `Retry-After`. That is far more often than metrics
need: a scrape reads every 15 to 60 seconds, and OTel exporters default to once
a minute.

Unlike a Pushgateway, nothing is stored between pushes. Each push is recorded
when it arrives, so push on a regular interval rather than once. A series that
stops being pushed goes stale the way a scraped one does when its target
disappears.

The runtime sets every `miren_*` label from the sandbox's identity, and a push
that uses a `miren_*` label itself is refused with a `400` naming the label.
The job and any other grouping labels in the URL are added to every sample.
Timestamps are refused too: a pushed sample is stamped when it arrives.

The URLs also accept OpenTelemetry metrics over OTLP/HTTP with protobuf, at the
scope's URL plus `/otlp/v1/metrics`. An app already instrumented with an
OpenTelemetry SDK needs no setup: Miren sets `OTEL_EXPORTER_OTLP_METRICS_ENDPOINT`,
`OTEL_EXPORTER_OTLP_METRICS_PROTOCOL` and `OTEL_EXPORTER_OTLP_METRICS_HEADERS`
to export at sandbox scope. It leaves all three unset if the app sets any
`OTEL_EXPORTER_OTLP_*` variable of its own, so an app with its own collector
keeps sending to it. To push shared-state gauges over OTLP, give that meter its
own exporter pointed at `$MIREN_METRICS_SHARED_PUSH_URL/otlp/v1/metrics`.

### Metrics from shared state

Some values are the same no matter which replica reads them: a count of rows,
the depth of a shared queue. If every replica of a service exports one, each
lands in its own series, since each carries its own `miren_sandbox`. `sum()`
across them returns the real value multiplied by the number of replicas, and the
multiplier changes whenever the service scales.

Push these at app scope, through `MIREN_METRICS_SHARED_PUSH_URL`, instead. App
scope leaves off `miren_sandbox`, `miren_runner` and `miren_app_version`, so
every writer lands on the same series and `sum()` reads the value once. That
holds however many replicas push it, through a deploy, and when a different
replica takes over. It keeps `miren_service`, though, so push a given value from
one service: the same gauge pushed from two services is two series.

That makes the natural home for a shared-state gauge a periodic job: most job
systems can run something on a cadence at most once per tick, and it doesn't
matter which worker picks it up. The number is still right without one. If
every replica pushes, the series is correct and the only cost is each replica
running the same query.

App scope takes gauges only, and refuses counters, histograms and summaries.
Counters from different processes can't share a series: each resets on its own
schedule, so their interleaved values make `rate()` meaningless. Push those at
sandbox scope and `sum()` them. For Prometheus pushes, an untyped metric whose
name ends in `_total`, `_count`, `_sum`, `_bucket` or `_created` is treated as a
counter. For OTLP, a non-monotonic cumulative sum (an `UpDownCounter`) counts as
a gauge.

The series goes stale if nothing pushes it for a few minutes, so whatever pushes
it has to keep running. A job that only runs while there is work to do will
leave gaps.

### Runtime operational metrics

When a remote-write destination is configured, the coordinator also ships its
own operational series through the same pipeline: the control process's Go
heap, goroutine count and resident memory (`go_*`, `process_resident_memory_bytes`),
embedded etcd backend health (`etcd_db_size_bytes`, `etcd_nospace_alarm`,
`etcd_nospace_recovery_total`, and friends), host usage (`node_*`), and reconcile
controller queue depths (`reconcile_controller_*`). Shipped copies carry
`miren_cluster` and `miren_runner` so series pooled from many clusters stay
distinct. These series also remain in the cluster's embedded VictoriaMetrics,
unlabeled, where the node is implicit.

There is no separate switch. Configuring the destination turns on application
metrics and runtime metrics together, and delivery failures for both appear in
the same `miren logs system vmagent` output.

## Distributed tracing

### Minimum working example

Miren's side needs no setup — every request is already traced, and the `traceparent` header is forwarded to your app. To join your app's spans to those traces, point an OpenTelemetry SDK at your backend in `.miren/app.toml`:

```toml
[[env]]
key = "OTEL_EXPORTER_OTLP_ENDPOINT"
value = "https://your-otel-collector:4318"

[[env]]
key = "OTEL_SERVICE_NAME"
value = "my-app"
```

The SDK picks up `traceparent` from incoming requests, so your app's spans land in the same trace as Miren's routing and cold-start spans.

### What Miren Traces

Every HTTP request that arrives at Miren generates a trace with spans covering:

- **httpingress** — The full request lifecycle: routing, lease acquisition, and proxying to your app
- **httpingress.lease** — Sandbox lease management, including whether a cached lease was used or a cold start was required
- **RPC calls** — Internal service-to-service communication within Miren
- **containerd gRPC** — Container operations like image pulls, container creation, and task management
- **BuildKit** — The embedded build daemon's own spans for each deploy's image build: each solve step, cache lookups, and the image export

The most useful spans for app developers are `httpingress` (overall request latency) and `httpingress.lease` (cold start visibility). The RPC, containerd, and BuildKit spans are primarily useful for operators debugging Miren itself.

### Configuring Miren's own export

Operators turn on Miren's trace export by setting `OTEL_EXPORTER_OTLP_ENDPOINT` (and `OTEL_EXPORTER_OTLP_HEADERS` if the collector needs credentials) in the environment of the `miren server` process. Miren always exports over OTLP/HTTP (`http/protobuf`), and it hands the embedded BuildKit daemon the same endpoint with the protocol pinned to match, so the collector only needs to accept HTTP. Setting `OTEL_EXPORTER_OTLP_PROTOCOL` yourself overrides that pin for BuildKit, but Miren's own exporter stays on HTTP regardless, so a gRPC-only collector will never see Miren's spans.

:::note[BuildKit metrics are off by default]
BuildKit can also push its own OTLP metrics, but most trace backends do not accept them, so Miren disables that exporter unless you opt in. Set `OTEL_EXPORTER_OTLP_METRICS_ENDPOINT`, or `OTEL_METRICS_EXPORTER=otlp`, if you have somewhere to send them.
:::

### Trace Context Propagation

Miren participates in [W3C Trace Context](https://www.w3.org/TR/trace-context/) propagation in both directions:

**Inbound:** If your request includes a `traceparent` header, Miren continues that trace rather than starting a new one. This means requests from an instrumented frontend or upstream service produce a single connected trace that includes Miren's processing.

**Outbound:** When Miren forwards a request to your app, it injects a `traceparent` header. Your app can pick this up to create child spans that appear in the same trace as the Miren infrastructure spans.

### Connecting Your App's Traces

Miren's tracing and your app's tracing are configured independently — Miren handles its own trace export, and your app handles its own. The `traceparent` header is what connects them: when both sides send traces to the same backend, they show up as one unified trace because they share the same trace ID.

To participate, add an OpenTelemetry SDK to your app and point it at your OTLP-compatible backend. The SDK will automatically read the `traceparent` header from incoming requests and create child spans.

Set these environment variables on your app in `.miren/app.toml`:

```toml
[[env]]
key = "OTEL_EXPORTER_OTLP_ENDPOINT"
value = "https://your-otel-collector:4318"

[[env]]
key = "OTEL_EXPORTER_OTLP_HEADERS"
value = "Authorization=Bearer your-api-key"

[[env]]
key = "OTEL_SERVICE_NAME"
value = "my-app"
```

This works with any OTel-compatible backend: Grafana Tempo, Honeycomb, Datadog, Jaeger, and others. You can use the same backend as your Miren cluster or a different one — as long as traces with the same trace ID end up in the same place, they'll be correlated.

### Python Example

Using the OpenTelemetry auto-instrumentation for Flask:

```bash
pip install opentelemetry-distro opentelemetry-exporter-otlp
opentelemetry-bootstrap -a install
```

```toml
[[env]]
key = "OTEL_EXPORTER_OTLP_ENDPOINT"
value = "https://your-otel-collector:4318"

[[env]]
key = "OTEL_SERVICE_NAME"
value = "my-flask-app"
```

```text
# Procfile
web: opentelemetry-instrument flask run --host 0.0.0.0 --port 3000
```

The `opentelemetry-instrument` wrapper automatically reads the `traceparent` header from incoming requests and creates spans for your Flask routes.

### Node.js Example

Using the OpenTelemetry auto-instrumentation for Node.js:

```bash
npm install @opentelemetry/auto-instrumentations-node
```

```toml
[[env]]
key = "OTEL_EXPORTER_OTLP_ENDPOINT"
value = "https://your-otel-collector:4318"

[[env]]
key = "OTEL_SERVICE_NAME"
value = "my-node-app"

[[env]]
key = "NODE_OPTIONS"
value = "--require @opentelemetry/auto-instrumentations-node/register"
```

The `--require` flag loads the auto-instrumentation before your app starts, automatically instrumenting HTTP, Express, and other common libraries.

### What a Trace Looks Like

A typical request trace shows the full path through Miren:

```text
httpingress                          [350ms]
├─ httpingress.lease                 [200ms]  (cold start)
│  ├─ rpc.call.AcquireLease         [195ms]
│  │  ├─ containerd...Images/Pull   [150ms]
│  │  └─ containerd...Tasks/Create  [40ms]
├─ [proxy to app]                   [150ms]
│  └─ my-app: GET /api/users        [145ms]  (your app's span)
```

On a warm request where a sandbox is already running:

```text
httpingress                          [15ms]
├─ httpingress.lease                 [0.1ms]  (cached lease)
├─ [proxy to app]                   [14ms]
│  └─ my-app: GET /api/users        [12ms]
```

## Next Steps

- [Logs](./logs.md) — View and filter application, build, and system logs
- [Services](./services.md) — Configure your app's services
- [Application Scaling](./scaling.md) — Understand cold starts and autoscaling

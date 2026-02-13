package otelproxy

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"

	"miren.dev/runtime/api/telemetry/telemetry_v1alpha"
	"miren.dev/runtime/pkg/rpc"
)

// proxyClient implements otlptrace.Client by forwarding serialized spans
// over the RPC telemetry service. Errors are swallowed (logged at debug)
// so that the batcher never blocks the calling command.
type proxyClient struct {
	tc  *telemetry_v1alpha.TelemetryClient
	log *slog.Logger
}

var _ otlptrace.Client = (*proxyClient)(nil)

func (p *proxyClient) Start(context.Context) error { return nil }
func (p *proxyClient) Stop(context.Context) error  { return nil }

func (p *proxyClient) UploadTraces(ctx context.Context, protoSpans []*tracepb.ResourceSpans) error {
	req := &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: protoSpans,
	}

	data, err := proto.Marshal(req)
	if err != nil {
		p.log.Debug("failed to marshal trace export request", "error", err)
		return nil
	}

	_, err = p.tc.ReportSpans(ctx, data)
	if err != nil {
		p.log.Debug("failed to send spans via proxy", "error", err)
	}
	return nil
}

// SetupProxyTracing configures OpenTelemetry tracing that ships spans through
// the server's RPC telemetry service. This allows CLI commands to export traces
// without needing OTEL_EXPORTER_OTLP_ENDPOINT set locally.
func SetupProxyTracing(ctx context.Context, rpcClient *rpc.NetworkClient, log *slog.Logger, attrs ...attribute.KeyValue) (shutdown func(context.Context) error, err error) {
	tc := telemetry_v1alpha.NewTelemetryClient(rpcClient)

	pc := &proxyClient{tc: tc, log: log}
	exporter, err := otlptrace.New(ctx, pc)
	if err != nil {
		return nil, err
	}

	hasServiceName := false
	for _, a := range attrs {
		if a.Key == semconv.ServiceNameKey {
			hasServiceName = true
			break
		}
	}
	resAttrs := make([]attribute.KeyValue, 0, len(attrs)+1)
	if !hasServiceName {
		resAttrs = append(resAttrs, semconv.ServiceName("miren-cli"))
	}
	resAttrs = append(resAttrs, attrs...)

	res, err := sdkresource.New(ctx,
		sdkresource.WithAttributes(resAttrs...),
	)
	if err != nil {
		_ = exporter.Shutdown(ctx)
		return nil, err
	}

	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}

package telemetry_v1alpha

import (
	"context"
	"encoding/json"

	"github.com/fxamacker/cbor/v2"
	rpc "miren.dev/runtime/pkg/rpc"
)

type telemetryReportSpansArgsData struct {
	SpanData *[]byte `cbor:"0,keyasint,omitempty" json:"span_data,omitempty"`
}

type TelemetryReportSpansArgs struct {
	call rpc.Call
	data telemetryReportSpansArgsData
}

func (v *TelemetryReportSpansArgs) HasSpanData() bool {
	return v.data.SpanData != nil
}

func (v *TelemetryReportSpansArgs) SpanData() []byte {
	if v.data.SpanData == nil {
		return nil
	}
	return *v.data.SpanData
}

func (v *TelemetryReportSpansArgs) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *TelemetryReportSpansArgs) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *TelemetryReportSpansArgs) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *TelemetryReportSpansArgs) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type telemetryReportSpansResultsData struct{}

type TelemetryReportSpansResults struct {
	call rpc.Call
	data telemetryReportSpansResultsData
}

func (v *TelemetryReportSpansResults) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *TelemetryReportSpansResults) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *TelemetryReportSpansResults) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *TelemetryReportSpansResults) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type TelemetryReportSpans struct {
	rpc.Call
	args    TelemetryReportSpansArgs
	results TelemetryReportSpansResults
}

func (t *TelemetryReportSpans) Args() *TelemetryReportSpansArgs {
	args := &t.args
	if args.call != nil {
		return args
	}
	args.call = t.Call
	t.Call.Args(args)
	return args
}

func (t *TelemetryReportSpans) Results() *TelemetryReportSpansResults {
	results := &t.results
	if results.call != nil {
		return results
	}
	results.call = t.Call
	t.Call.Results(results)
	return results
}

type Telemetry interface {
	ReportSpans(ctx context.Context, state *TelemetryReportSpans) error
}

type reexportTelemetry struct {
	client rpc.Client
}

func (reexportTelemetry) ReportSpans(ctx context.Context, state *TelemetryReportSpans) error {
	panic("not implemented")
}

func (t reexportTelemetry) CapabilityClient() rpc.Client {
	return t.client
}

func AdaptTelemetry(t Telemetry) *rpc.Interface {
	methods := []rpc.Method{
		{
			Name:          "reportSpans",
			InterfaceName: "Telemetry",
			Index:         0,
			Public:        false,
			Handler: func(ctx context.Context, call rpc.Call) error {
				return t.ReportSpans(ctx, &TelemetryReportSpans{Call: call})
			},
		},
	}

	return rpc.NewInterface(methods, t)
}

type TelemetryClient struct {
	rpc.Client
}

func NewTelemetryClient(client rpc.Client) *TelemetryClient {
	return &TelemetryClient{Client: client}
}

func (c TelemetryClient) Export() Telemetry {
	return reexportTelemetry{client: c.Client}
}

type TelemetryClientReportSpansResults struct {
	client rpc.Client
	data   telemetryReportSpansResultsData
}

func (v TelemetryClient) ReportSpans(ctx context.Context, span_data []byte) (*TelemetryClientReportSpansResults, error) {
	args := TelemetryReportSpansArgs{}
	args.data.SpanData = &span_data

	var ret telemetryReportSpansResultsData

	err := v.Call(ctx, "reportSpans", &args, &ret)
	if err != nil {
		return nil, err
	}

	return &TelemetryClientReportSpansResults{client: v.Client, data: ret}, nil
}

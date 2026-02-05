package outboard_v1alpha

import (
	"context"
	"encoding/json"

	"github.com/fxamacker/cbor/v2"
	rpc "miren.dev/runtime/pkg/rpc"
	"miren.dev/runtime/pkg/rpc/standard"
)

type outboardHealthStatusData struct {
	Healthy   *bool               `cbor:"0,keyasint,omitempty" json:"healthy,omitempty"`
	Timestamp *standard.Timestamp `cbor:"1,keyasint,omitempty" json:"timestamp,omitempty"`
	Pid       *int32              `cbor:"2,keyasint,omitempty" json:"pid,omitempty"`
	Uptime    *standard.Duration  `cbor:"3,keyasint,omitempty" json:"uptime,omitempty"`
	LastError *string             `cbor:"4,keyasint,omitempty" json:"last_error,omitempty"`
}

type OutboardHealthStatus struct {
	data outboardHealthStatusData
}

func (v *OutboardHealthStatus) HasHealthy() bool {
	return v.data.Healthy != nil
}

func (v *OutboardHealthStatus) Healthy() bool {
	if v.data.Healthy == nil {
		return false
	}
	return *v.data.Healthy
}

func (v *OutboardHealthStatus) SetHealthy(healthy bool) {
	v.data.Healthy = &healthy
}

func (v *OutboardHealthStatus) HasTimestamp() bool {
	return v.data.Timestamp != nil
}

func (v *OutboardHealthStatus) Timestamp() *standard.Timestamp {
	return v.data.Timestamp
}

func (v *OutboardHealthStatus) SetTimestamp(timestamp *standard.Timestamp) {
	v.data.Timestamp = timestamp
}

func (v *OutboardHealthStatus) HasPid() bool {
	return v.data.Pid != nil
}

func (v *OutboardHealthStatus) Pid() int32 {
	if v.data.Pid == nil {
		return 0
	}
	return *v.data.Pid
}

func (v *OutboardHealthStatus) SetPid(pid int32) {
	v.data.Pid = &pid
}

func (v *OutboardHealthStatus) HasUptime() bool {
	return v.data.Uptime != nil
}

func (v *OutboardHealthStatus) Uptime() *standard.Duration {
	return v.data.Uptime
}

func (v *OutboardHealthStatus) SetUptime(uptime *standard.Duration) {
	v.data.Uptime = uptime
}

func (v *OutboardHealthStatus) HasLastError() bool {
	return v.data.LastError != nil
}

func (v *OutboardHealthStatus) LastError() string {
	if v.data.LastError == nil {
		return ""
	}
	return *v.data.LastError
}

func (v *OutboardHealthStatus) SetLastError(last_error string) {
	v.data.LastError = &last_error
}

func (v *OutboardHealthStatus) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *OutboardHealthStatus) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *OutboardHealthStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *OutboardHealthStatus) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type outboardVersionResultData struct {
	CurrentVersion *uint64 `cbor:"0,keyasint,omitempty" json:"current_version,omitempty"`
	NeedsRestart   *bool   `cbor:"1,keyasint,omitempty" json:"needs_restart,omitempty"`
}

type OutboardVersionResult struct {
	data outboardVersionResultData
}

func (v *OutboardVersionResult) HasCurrentVersion() bool {
	return v.data.CurrentVersion != nil
}

func (v *OutboardVersionResult) CurrentVersion() uint64 {
	if v.data.CurrentVersion == nil {
		return 0
	}
	return *v.data.CurrentVersion
}

func (v *OutboardVersionResult) SetCurrentVersion(current_version uint64) {
	v.data.CurrentVersion = &current_version
}

func (v *OutboardVersionResult) HasNeedsRestart() bool {
	return v.data.NeedsRestart != nil
}

func (v *OutboardVersionResult) NeedsRestart() bool {
	if v.data.NeedsRestart == nil {
		return false
	}
	return *v.data.NeedsRestart
}

func (v *OutboardVersionResult) SetNeedsRestart(needs_restart bool) {
	v.data.NeedsRestart = &needs_restart
}

func (v *OutboardVersionResult) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *OutboardVersionResult) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *OutboardVersionResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *OutboardVersionResult) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type outboardControlHealthArgsData struct{}

type OutboardControlHealthArgs struct {
	call rpc.Call
	data outboardControlHealthArgsData
}

func (v *OutboardControlHealthArgs) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *OutboardControlHealthArgs) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *OutboardControlHealthArgs) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *OutboardControlHealthArgs) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type outboardControlHealthResultsData struct {
	Status *OutboardHealthStatus `cbor:"0,keyasint,omitempty" json:"status,omitempty"`
}

type OutboardControlHealthResults struct {
	call rpc.Call
	data outboardControlHealthResultsData
}

func (v *OutboardControlHealthResults) SetStatus(status *OutboardHealthStatus) {
	v.data.Status = status
}

func (v *OutboardControlHealthResults) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *OutboardControlHealthResults) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *OutboardControlHealthResults) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *OutboardControlHealthResults) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type outboardControlCheckVersionArgsData struct {
	ExpectedVersion *uint64 `cbor:"0,keyasint,omitempty" json:"expected_version,omitempty"`
}

type OutboardControlCheckVersionArgs struct {
	call rpc.Call
	data outboardControlCheckVersionArgsData
}

func (v *OutboardControlCheckVersionArgs) HasExpectedVersion() bool {
	return v.data.ExpectedVersion != nil
}

func (v *OutboardControlCheckVersionArgs) ExpectedVersion() uint64 {
	if v.data.ExpectedVersion == nil {
		return 0
	}
	return *v.data.ExpectedVersion
}

func (v *OutboardControlCheckVersionArgs) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *OutboardControlCheckVersionArgs) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *OutboardControlCheckVersionArgs) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *OutboardControlCheckVersionArgs) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type outboardControlCheckVersionResultsData struct {
	Result *OutboardVersionResult `cbor:"0,keyasint,omitempty" json:"result,omitempty"`
}

type OutboardControlCheckVersionResults struct {
	call rpc.Call
	data outboardControlCheckVersionResultsData
}

func (v *OutboardControlCheckVersionResults) SetResult(result *OutboardVersionResult) {
	v.data.Result = result
}

func (v *OutboardControlCheckVersionResults) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *OutboardControlCheckVersionResults) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *OutboardControlCheckVersionResults) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *OutboardControlCheckVersionResults) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type OutboardControlHealth struct {
	rpc.Call
	args    OutboardControlHealthArgs
	results OutboardControlHealthResults
}

func (t *OutboardControlHealth) Args() *OutboardControlHealthArgs {
	args := &t.args
	if args.call != nil {
		return args
	}
	args.call = t.Call
	t.Call.Args(args)
	return args
}

func (t *OutboardControlHealth) Results() *OutboardControlHealthResults {
	results := &t.results
	if results.call != nil {
		return results
	}
	results.call = t.Call
	t.Call.Results(results)
	return results
}

type OutboardControlCheckVersion struct {
	rpc.Call
	args    OutboardControlCheckVersionArgs
	results OutboardControlCheckVersionResults
}

func (t *OutboardControlCheckVersion) Args() *OutboardControlCheckVersionArgs {
	args := &t.args
	if args.call != nil {
		return args
	}
	args.call = t.Call
	t.Call.Args(args)
	return args
}

func (t *OutboardControlCheckVersion) Results() *OutboardControlCheckVersionResults {
	results := &t.results
	if results.call != nil {
		return results
	}
	results.call = t.Call
	t.Call.Results(results)
	return results
}

type OutboardControl interface {
	Health(ctx context.Context, state *OutboardControlHealth) error
	CheckVersion(ctx context.Context, state *OutboardControlCheckVersion) error
}

type reexportOutboardControl struct {
	client rpc.Client
}

func (reexportOutboardControl) Health(ctx context.Context, state *OutboardControlHealth) error {
	panic("not implemented")
}

func (reexportOutboardControl) CheckVersion(ctx context.Context, state *OutboardControlCheckVersion) error {
	panic("not implemented")
}

func (t reexportOutboardControl) CapabilityClient() rpc.Client {
	return t.client
}

func AdaptOutboardControl(t OutboardControl) *rpc.Interface {
	methods := []rpc.Method{
		{
			Name:          "health",
			InterfaceName: "OutboardControl",
			Index:         0,
			Public:        false,
			Handler: func(ctx context.Context, call rpc.Call) error {
				return t.Health(ctx, &OutboardControlHealth{Call: call})
			},
		},
		{
			Name:          "checkVersion",
			InterfaceName: "OutboardControl",
			Index:         0,
			Public:        false,
			Handler: func(ctx context.Context, call rpc.Call) error {
				return t.CheckVersion(ctx, &OutboardControlCheckVersion{Call: call})
			},
		},
	}

	return rpc.NewInterface(methods, t)
}

type OutboardControlClient struct {
	rpc.Client
}

func NewOutboardControlClient(client rpc.Client) *OutboardControlClient {
	return &OutboardControlClient{Client: client}
}

func (c OutboardControlClient) Export() OutboardControl {
	return reexportOutboardControl{client: c.Client}
}

type OutboardControlClientHealthResults struct {
	client rpc.Client
	data   outboardControlHealthResultsData
}

func (v *OutboardControlClientHealthResults) HasStatus() bool {
	return v.data.Status != nil
}

func (v *OutboardControlClientHealthResults) Status() *OutboardHealthStatus {
	return v.data.Status
}

func (v OutboardControlClient) Health(ctx context.Context) (*OutboardControlClientHealthResults, error) {
	args := OutboardControlHealthArgs{}

	var ret outboardControlHealthResultsData

	err := v.Call(ctx, "health", &args, &ret)
	if err != nil {
		return nil, err
	}

	return &OutboardControlClientHealthResults{client: v.Client, data: ret}, nil
}

type OutboardControlClientCheckVersionResults struct {
	client rpc.Client
	data   outboardControlCheckVersionResultsData
}

func (v *OutboardControlClientCheckVersionResults) HasResult() bool {
	return v.data.Result != nil
}

func (v *OutboardControlClientCheckVersionResults) Result() *OutboardVersionResult {
	return v.data.Result
}

func (v OutboardControlClient) CheckVersion(ctx context.Context, expected_version uint64) (*OutboardControlClientCheckVersionResults, error) {
	args := OutboardControlCheckVersionArgs{}
	args.data.ExpectedVersion = &expected_version

	var ret outboardControlCheckVersionResultsData

	err := v.Call(ctx, "checkVersion", &args, &ret)
	if err != nil {
		return nil, err
	}

	return &OutboardControlClientCheckVersionResults{client: v.Client, data: ret}, nil
}

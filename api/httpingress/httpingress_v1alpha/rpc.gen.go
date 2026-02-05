package httpingress_v1alpha

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/fxamacker/cbor/v2"
	rpc "miren.dev/runtime/pkg/rpc"
)

type httpHeaderData struct {
	Key   *string `cbor:"0,keyasint,omitempty" json:"key,omitempty"`
	Value *string `cbor:"1,keyasint,omitempty" json:"value,omitempty"`
}

type HttpHeader struct {
	data httpHeaderData
}

func (v *HttpHeader) HasKey() bool {
	return v.data.Key != nil
}

func (v *HttpHeader) Key() string {
	if v.data.Key == nil {
		return ""
	}
	return *v.data.Key
}

func (v *HttpHeader) SetKey(key string) {
	v.data.Key = &key
}

func (v *HttpHeader) HasValue() bool {
	return v.data.Value != nil
}

func (v *HttpHeader) Value() string {
	if v.data.Value == nil {
		return ""
	}
	return *v.data.Value
}

func (v *HttpHeader) SetValue(value string) {
	v.data.Value = &value
}

func (v *HttpHeader) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *HttpHeader) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *HttpHeader) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *HttpHeader) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type internalHttpRequestData struct {
	Method    *string        `cbor:"0,keyasint,omitempty" json:"method,omitempty"`
	Path      *string        `cbor:"1,keyasint,omitempty" json:"path,omitempty"`
	Headers   *[]*HttpHeader `cbor:"2,keyasint,omitempty" json:"headers,omitempty"`
	Body      *[]byte        `cbor:"3,keyasint,omitempty" json:"body,omitempty"`
	AppId     *string        `cbor:"4,keyasint,omitempty" json:"app_id,omitempty"`
	Service   *string        `cbor:"5,keyasint,omitempty" json:"service,omitempty"`
	TimeoutMs *int32         `cbor:"6,keyasint,omitempty" json:"timeout_ms,omitempty"`
}

type InternalHttpRequest struct {
	data internalHttpRequestData
}

func (v *InternalHttpRequest) HasMethod() bool {
	return v.data.Method != nil
}

func (v *InternalHttpRequest) Method() string {
	if v.data.Method == nil {
		return ""
	}
	return *v.data.Method
}

func (v *InternalHttpRequest) SetMethod(method string) {
	v.data.Method = &method
}

func (v *InternalHttpRequest) HasPath() bool {
	return v.data.Path != nil
}

func (v *InternalHttpRequest) Path() string {
	if v.data.Path == nil {
		return ""
	}
	return *v.data.Path
}

func (v *InternalHttpRequest) SetPath(path string) {
	v.data.Path = &path
}

func (v *InternalHttpRequest) HasHeaders() bool {
	return v.data.Headers != nil
}

func (v *InternalHttpRequest) Headers() []*HttpHeader {
	if v.data.Headers == nil {
		return nil
	}
	return *v.data.Headers
}

func (v *InternalHttpRequest) SetHeaders(headers []*HttpHeader) {
	x := slices.Clone(headers)
	v.data.Headers = &x
}

func (v *InternalHttpRequest) HasBody() bool {
	return v.data.Body != nil
}

func (v *InternalHttpRequest) Body() *[]byte {
	return v.data.Body
}

func (v *InternalHttpRequest) SetBody(body *[]byte) {
	v.data.Body = body
}

func (v *InternalHttpRequest) HasAppId() bool {
	return v.data.AppId != nil
}

func (v *InternalHttpRequest) AppId() string {
	if v.data.AppId == nil {
		return ""
	}
	return *v.data.AppId
}

func (v *InternalHttpRequest) SetAppId(app_id string) {
	v.data.AppId = &app_id
}

func (v *InternalHttpRequest) HasService() bool {
	return v.data.Service != nil
}

func (v *InternalHttpRequest) Service() string {
	if v.data.Service == nil {
		return ""
	}
	return *v.data.Service
}

func (v *InternalHttpRequest) SetService(service string) {
	v.data.Service = &service
}

func (v *InternalHttpRequest) HasTimeoutMs() bool {
	return v.data.TimeoutMs != nil
}

func (v *InternalHttpRequest) TimeoutMs() int32 {
	if v.data.TimeoutMs == nil {
		return 0
	}
	return *v.data.TimeoutMs
}

func (v *InternalHttpRequest) SetTimeoutMs(timeout_ms int32) {
	v.data.TimeoutMs = &timeout_ms
}

func (v *InternalHttpRequest) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *InternalHttpRequest) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *InternalHttpRequest) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *InternalHttpRequest) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type internalHttpResponseData struct {
	StatusCode *int32         `cbor:"0,keyasint,omitempty" json:"status_code,omitempty"`
	Headers    *[]*HttpHeader `cbor:"1,keyasint,omitempty" json:"headers,omitempty"`
	Body       *[]byte        `cbor:"2,keyasint,omitempty" json:"body,omitempty"`
	Error      *string        `cbor:"3,keyasint,omitempty" json:"error,omitempty"`
}

type InternalHttpResponse struct {
	data internalHttpResponseData
}

func (v *InternalHttpResponse) HasStatusCode() bool {
	return v.data.StatusCode != nil
}

func (v *InternalHttpResponse) StatusCode() int32 {
	if v.data.StatusCode == nil {
		return 0
	}
	return *v.data.StatusCode
}

func (v *InternalHttpResponse) SetStatusCode(status_code int32) {
	v.data.StatusCode = &status_code
}

func (v *InternalHttpResponse) HasHeaders() bool {
	return v.data.Headers != nil
}

func (v *InternalHttpResponse) Headers() []*HttpHeader {
	if v.data.Headers == nil {
		return nil
	}
	return *v.data.Headers
}

func (v *InternalHttpResponse) SetHeaders(headers []*HttpHeader) {
	x := slices.Clone(headers)
	v.data.Headers = &x
}

func (v *InternalHttpResponse) HasBody() bool {
	return v.data.Body != nil
}

func (v *InternalHttpResponse) Body() *[]byte {
	return v.data.Body
}

func (v *InternalHttpResponse) SetBody(body *[]byte) {
	v.data.Body = body
}

func (v *InternalHttpResponse) HasError() bool {
	return v.data.Error != nil
}

func (v *InternalHttpResponse) Error() string {
	if v.data.Error == nil {
		return ""
	}
	return *v.data.Error
}

func (v *InternalHttpResponse) SetError(error string) {
	v.data.Error = &error
}

func (v *InternalHttpResponse) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *InternalHttpResponse) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *InternalHttpResponse) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *InternalHttpResponse) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type internalHttpDoRequestArgsData struct {
	Request *InternalHttpRequest `cbor:"0,keyasint,omitempty" json:"request,omitempty"`
}

type InternalHttpDoRequestArgs struct {
	call rpc.Call
	data internalHttpDoRequestArgsData
}

func (v *InternalHttpDoRequestArgs) HasRequest() bool {
	return v.data.Request != nil
}

func (v *InternalHttpDoRequestArgs) Request() *InternalHttpRequest {
	return v.data.Request
}

func (v *InternalHttpDoRequestArgs) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *InternalHttpDoRequestArgs) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *InternalHttpDoRequestArgs) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *InternalHttpDoRequestArgs) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type internalHttpDoRequestResultsData struct {
	Response *InternalHttpResponse `cbor:"0,keyasint,omitempty" json:"response,omitempty"`
}

type InternalHttpDoRequestResults struct {
	call rpc.Call
	data internalHttpDoRequestResultsData
}

func (v *InternalHttpDoRequestResults) SetResponse(response *InternalHttpResponse) {
	v.data.Response = response
}

func (v *InternalHttpDoRequestResults) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *InternalHttpDoRequestResults) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *InternalHttpDoRequestResults) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *InternalHttpDoRequestResults) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type InternalHttpDoRequest struct {
	rpc.Call
	args    InternalHttpDoRequestArgs
	results InternalHttpDoRequestResults
}

func (t *InternalHttpDoRequest) Args() *InternalHttpDoRequestArgs {
	args := &t.args
	if args.call != nil {
		return args
	}
	args.call = t.Call
	t.Call.Args(args)
	return args
}

func (t *InternalHttpDoRequest) Results() *InternalHttpDoRequestResults {
	results := &t.results
	if results.call != nil {
		return results
	}
	results.call = t.Call
	t.Call.Results(results)
	return results
}

type InternalHttp interface {
	DoRequest(ctx context.Context, state *InternalHttpDoRequest) error
}

type reexportInternalHttp struct {
	client rpc.Client
}

func (reexportInternalHttp) DoRequest(ctx context.Context, state *InternalHttpDoRequest) error {
	panic("not implemented")
}

func (t reexportInternalHttp) CapabilityClient() rpc.Client {
	return t.client
}

func AdaptInternalHttp(t InternalHttp) *rpc.Interface {
	methods := []rpc.Method{
		{
			Name:          "DoRequest",
			InterfaceName: "InternalHttp",
			Index:         0,
			Public:        false,
			Handler: func(ctx context.Context, call rpc.Call) error {
				return t.DoRequest(ctx, &InternalHttpDoRequest{Call: call})
			},
		},
	}

	return rpc.NewInterface(methods, t)
}

type InternalHttpClient struct {
	rpc.Client
}

func NewInternalHttpClient(client rpc.Client) *InternalHttpClient {
	return &InternalHttpClient{Client: client}
}

func (c InternalHttpClient) Export() InternalHttp {
	return reexportInternalHttp{client: c.Client}
}

type InternalHttpClientDoRequestResults struct {
	client rpc.Client
	data   internalHttpDoRequestResultsData
}

func (v *InternalHttpClientDoRequestResults) HasResponse() bool {
	return v.data.Response != nil
}

func (v *InternalHttpClientDoRequestResults) Response() *InternalHttpResponse {
	return v.data.Response
}

func (v InternalHttpClient) DoRequest(ctx context.Context, request *InternalHttpRequest) (*InternalHttpClientDoRequestResults, error) {
	args := InternalHttpDoRequestArgs{}
	args.data.Request = request

	var ret internalHttpDoRequestResultsData

	err := v.Call(ctx, "DoRequest", &args, &ret)
	if err != nil {
		return nil, err
	}

	return &InternalHttpClientDoRequestResults{client: v.Client, data: ret}, nil
}

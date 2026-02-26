package oidcbinding_v1alpha

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/fxamacker/cbor/v2"
	rpc "miren.dev/runtime/pkg/rpc"
)

type claimConditionData struct {
	Key     *string `cbor:"0,keyasint,omitempty" json:"key,omitempty"`
	Pattern *string `cbor:"1,keyasint,omitempty" json:"pattern,omitempty"`
}

type ClaimCondition struct {
	data claimConditionData
}

func (v *ClaimCondition) HasKey() bool {
	return v.data.Key != nil
}

func (v *ClaimCondition) Key() string {
	if v.data.Key == nil {
		return ""
	}
	return *v.data.Key
}

func (v *ClaimCondition) SetKey(key string) {
	v.data.Key = &key
}

func (v *ClaimCondition) HasPattern() bool {
	return v.data.Pattern != nil
}

func (v *ClaimCondition) Pattern() string {
	if v.data.Pattern == nil {
		return ""
	}
	return *v.data.Pattern
}

func (v *ClaimCondition) SetPattern(pattern string) {
	v.data.Pattern = &pattern
}

func (v *ClaimCondition) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *ClaimCondition) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *ClaimCondition) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *ClaimCondition) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type bindingInfoData struct {
	Id              *string            `cbor:"0,keyasint,omitempty" json:"id,omitempty"`
	App             *string            `cbor:"1,keyasint,omitempty" json:"app,omitempty"`
	Provider        *string            `cbor:"2,keyasint,omitempty" json:"provider,omitempty"`
	Issuer          *string            `cbor:"3,keyasint,omitempty" json:"issuer,omitempty"`
	SubjectPattern  *string            `cbor:"4,keyasint,omitempty" json:"subject_pattern,omitempty"`
	ClaimConditions *[]*ClaimCondition `cbor:"5,keyasint,omitempty" json:"claim_conditions,omitempty"`
	Description     *string            `cbor:"6,keyasint,omitempty" json:"description,omitempty"`
}

type BindingInfo struct {
	data bindingInfoData
}

func (v *BindingInfo) HasId() bool {
	return v.data.Id != nil
}

func (v *BindingInfo) Id() string {
	if v.data.Id == nil {
		return ""
	}
	return *v.data.Id
}

func (v *BindingInfo) SetId(id string) {
	v.data.Id = &id
}

func (v *BindingInfo) HasApp() bool {
	return v.data.App != nil
}

func (v *BindingInfo) App() string {
	if v.data.App == nil {
		return ""
	}
	return *v.data.App
}

func (v *BindingInfo) SetApp(app string) {
	v.data.App = &app
}

func (v *BindingInfo) HasProvider() bool {
	return v.data.Provider != nil
}

func (v *BindingInfo) Provider() string {
	if v.data.Provider == nil {
		return ""
	}
	return *v.data.Provider
}

func (v *BindingInfo) SetProvider(provider string) {
	v.data.Provider = &provider
}

func (v *BindingInfo) HasIssuer() bool {
	return v.data.Issuer != nil
}

func (v *BindingInfo) Issuer() string {
	if v.data.Issuer == nil {
		return ""
	}
	return *v.data.Issuer
}

func (v *BindingInfo) SetIssuer(issuer string) {
	v.data.Issuer = &issuer
}

func (v *BindingInfo) HasSubjectPattern() bool {
	return v.data.SubjectPattern != nil
}

func (v *BindingInfo) SubjectPattern() string {
	if v.data.SubjectPattern == nil {
		return ""
	}
	return *v.data.SubjectPattern
}

func (v *BindingInfo) SetSubjectPattern(subject_pattern string) {
	v.data.SubjectPattern = &subject_pattern
}

func (v *BindingInfo) HasClaimConditions() bool {
	return v.data.ClaimConditions != nil
}

func (v *BindingInfo) ClaimConditions() []*ClaimCondition {
	if v.data.ClaimConditions == nil {
		return nil
	}
	return *v.data.ClaimConditions
}

func (v *BindingInfo) SetClaimConditions(claim_conditions []*ClaimCondition) {
	x := slices.Clone(claim_conditions)
	v.data.ClaimConditions = &x
}

func (v *BindingInfo) HasDescription() bool {
	return v.data.Description != nil
}

func (v *BindingInfo) Description() string {
	if v.data.Description == nil {
		return ""
	}
	return *v.data.Description
}

func (v *BindingInfo) SetDescription(description string) {
	v.data.Description = &description
}

func (v *BindingInfo) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *BindingInfo) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *BindingInfo) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *BindingInfo) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type oidcBindingsAddArgsData struct {
	App             *string            `cbor:"0,keyasint,omitempty" json:"app,omitempty"`
	Provider        *string            `cbor:"1,keyasint,omitempty" json:"provider,omitempty"`
	Issuer          *string            `cbor:"2,keyasint,omitempty" json:"issuer,omitempty"`
	SubjectPattern  *string            `cbor:"3,keyasint,omitempty" json:"subject_pattern,omitempty"`
	ClaimConditions *[]*ClaimCondition `cbor:"4,keyasint,omitempty" json:"claim_conditions,omitempty"`
	Description     *string            `cbor:"5,keyasint,omitempty" json:"description,omitempty"`
}

type OidcBindingsAddArgs struct {
	call rpc.Call
	data oidcBindingsAddArgsData
}

func (v *OidcBindingsAddArgs) HasApp() bool {
	return v.data.App != nil
}

func (v *OidcBindingsAddArgs) App() string {
	if v.data.App == nil {
		return ""
	}
	return *v.data.App
}

func (v *OidcBindingsAddArgs) HasProvider() bool {
	return v.data.Provider != nil
}

func (v *OidcBindingsAddArgs) Provider() string {
	if v.data.Provider == nil {
		return ""
	}
	return *v.data.Provider
}

func (v *OidcBindingsAddArgs) HasIssuer() bool {
	return v.data.Issuer != nil
}

func (v *OidcBindingsAddArgs) Issuer() string {
	if v.data.Issuer == nil {
		return ""
	}
	return *v.data.Issuer
}

func (v *OidcBindingsAddArgs) HasSubjectPattern() bool {
	return v.data.SubjectPattern != nil
}

func (v *OidcBindingsAddArgs) SubjectPattern() string {
	if v.data.SubjectPattern == nil {
		return ""
	}
	return *v.data.SubjectPattern
}

func (v *OidcBindingsAddArgs) HasClaimConditions() bool {
	return v.data.ClaimConditions != nil
}

func (v *OidcBindingsAddArgs) ClaimConditions() []*ClaimCondition {
	if v.data.ClaimConditions == nil {
		return nil
	}
	return *v.data.ClaimConditions
}

func (v *OidcBindingsAddArgs) HasDescription() bool {
	return v.data.Description != nil
}

func (v *OidcBindingsAddArgs) Description() string {
	if v.data.Description == nil {
		return ""
	}
	return *v.data.Description
}

func (v *OidcBindingsAddArgs) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *OidcBindingsAddArgs) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *OidcBindingsAddArgs) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *OidcBindingsAddArgs) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type oidcBindingsAddResultsData struct {
	Binding *BindingInfo `cbor:"0,keyasint,omitempty" json:"binding,omitempty"`
	Error   *string      `cbor:"1,keyasint,omitempty" json:"error,omitempty"`
}

type OidcBindingsAddResults struct {
	call rpc.Call
	data oidcBindingsAddResultsData
}

func (v *OidcBindingsAddResults) SetBinding(binding *BindingInfo) {
	v.data.Binding = binding
}

func (v *OidcBindingsAddResults) SetError(error string) {
	v.data.Error = &error
}

func (v *OidcBindingsAddResults) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *OidcBindingsAddResults) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *OidcBindingsAddResults) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *OidcBindingsAddResults) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type oidcBindingsListArgsData struct {
	App *string `cbor:"0,keyasint,omitempty" json:"app,omitempty"`
}

type OidcBindingsListArgs struct {
	call rpc.Call
	data oidcBindingsListArgsData
}

func (v *OidcBindingsListArgs) HasApp() bool {
	return v.data.App != nil
}

func (v *OidcBindingsListArgs) App() string {
	if v.data.App == nil {
		return ""
	}
	return *v.data.App
}

func (v *OidcBindingsListArgs) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *OidcBindingsListArgs) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *OidcBindingsListArgs) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *OidcBindingsListArgs) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type oidcBindingsListResultsData struct {
	Bindings *[]*BindingInfo `cbor:"0,keyasint,omitempty" json:"bindings,omitempty"`
}

type OidcBindingsListResults struct {
	call rpc.Call
	data oidcBindingsListResultsData
}

func (v *OidcBindingsListResults) SetBindings(bindings []*BindingInfo) {
	x := slices.Clone(bindings)
	v.data.Bindings = &x
}

func (v *OidcBindingsListResults) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *OidcBindingsListResults) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *OidcBindingsListResults) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *OidcBindingsListResults) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type oidcBindingsRemoveArgsData struct {
	Id *string `cbor:"0,keyasint,omitempty" json:"id,omitempty"`
}

type OidcBindingsRemoveArgs struct {
	call rpc.Call
	data oidcBindingsRemoveArgsData
}

func (v *OidcBindingsRemoveArgs) HasId() bool {
	return v.data.Id != nil
}

func (v *OidcBindingsRemoveArgs) Id() string {
	if v.data.Id == nil {
		return ""
	}
	return *v.data.Id
}

func (v *OidcBindingsRemoveArgs) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *OidcBindingsRemoveArgs) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *OidcBindingsRemoveArgs) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *OidcBindingsRemoveArgs) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type oidcBindingsRemoveResultsData struct {
	Success *bool   `cbor:"0,keyasint,omitempty" json:"success,omitempty"`
	Error   *string `cbor:"1,keyasint,omitempty" json:"error,omitempty"`
}

type OidcBindingsRemoveResults struct {
	call rpc.Call
	data oidcBindingsRemoveResultsData
}

func (v *OidcBindingsRemoveResults) SetSuccess(success bool) {
	v.data.Success = &success
}

func (v *OidcBindingsRemoveResults) SetError(error string) {
	v.data.Error = &error
}

func (v *OidcBindingsRemoveResults) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *OidcBindingsRemoveResults) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *OidcBindingsRemoveResults) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *OidcBindingsRemoveResults) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type OidcBindingsAdd struct {
	rpc.Call
	args    OidcBindingsAddArgs
	results OidcBindingsAddResults
}

func (t *OidcBindingsAdd) Args() *OidcBindingsAddArgs {
	args := &t.args
	if args.call != nil {
		return args
	}
	args.call = t.Call
	t.Call.Args(args)
	return args
}

func (t *OidcBindingsAdd) Results() *OidcBindingsAddResults {
	results := &t.results
	if results.call != nil {
		return results
	}
	results.call = t.Call
	t.Call.Results(results)
	return results
}

type OidcBindingsList struct {
	rpc.Call
	args    OidcBindingsListArgs
	results OidcBindingsListResults
}

func (t *OidcBindingsList) Args() *OidcBindingsListArgs {
	args := &t.args
	if args.call != nil {
		return args
	}
	args.call = t.Call
	t.Call.Args(args)
	return args
}

func (t *OidcBindingsList) Results() *OidcBindingsListResults {
	results := &t.results
	if results.call != nil {
		return results
	}
	results.call = t.Call
	t.Call.Results(results)
	return results
}

type OidcBindingsRemove struct {
	rpc.Call
	args    OidcBindingsRemoveArgs
	results OidcBindingsRemoveResults
}

func (t *OidcBindingsRemove) Args() *OidcBindingsRemoveArgs {
	args := &t.args
	if args.call != nil {
		return args
	}
	args.call = t.Call
	t.Call.Args(args)
	return args
}

func (t *OidcBindingsRemove) Results() *OidcBindingsRemoveResults {
	results := &t.results
	if results.call != nil {
		return results
	}
	results.call = t.Call
	t.Call.Results(results)
	return results
}

type OidcBindings interface {
	Add(ctx context.Context, state *OidcBindingsAdd) error
	List(ctx context.Context, state *OidcBindingsList) error
	Remove(ctx context.Context, state *OidcBindingsRemove) error
}

type reexportOidcBindings struct {
	client rpc.Client
}

func (reexportOidcBindings) Add(ctx context.Context, state *OidcBindingsAdd) error {
	panic("not implemented")
}

func (reexportOidcBindings) List(ctx context.Context, state *OidcBindingsList) error {
	panic("not implemented")
}

func (reexportOidcBindings) Remove(ctx context.Context, state *OidcBindingsRemove) error {
	panic("not implemented")
}

func (t reexportOidcBindings) CapabilityClient() rpc.Client {
	return t.client
}

func AdaptOidcBindings(t OidcBindings) *rpc.Interface {
	methods := []rpc.Method{
		{
			Name:          "Add",
			InterfaceName: "OidcBindings",
			Index:         0,
			Public:        false,
			Handler: func(ctx context.Context, call rpc.Call) error {
				return t.Add(ctx, &OidcBindingsAdd{Call: call})
			},
		},
		{
			Name:          "List",
			InterfaceName: "OidcBindings",
			Index:         1,
			Public:        false,
			Handler: func(ctx context.Context, call rpc.Call) error {
				return t.List(ctx, &OidcBindingsList{Call: call})
			},
		},
		{
			Name:          "Remove",
			InterfaceName: "OidcBindings",
			Index:         2,
			Public:        false,
			Handler: func(ctx context.Context, call rpc.Call) error {
				return t.Remove(ctx, &OidcBindingsRemove{Call: call})
			},
		},
	}

	return rpc.NewInterface(methods, t)
}

type OidcBindingsClient struct {
	rpc.Client
}

func NewOidcBindingsClient(client rpc.Client) *OidcBindingsClient {
	return &OidcBindingsClient{Client: client}
}

func (c OidcBindingsClient) Export() OidcBindings {
	return reexportOidcBindings{client: c.Client}
}

type OidcBindingsClientAddResults struct {
	client rpc.Client
	data   oidcBindingsAddResultsData
}

func (v *OidcBindingsClientAddResults) HasBinding() bool {
	return v.data.Binding != nil
}

func (v *OidcBindingsClientAddResults) Binding() *BindingInfo {
	return v.data.Binding
}

func (v *OidcBindingsClientAddResults) HasError() bool {
	return v.data.Error != nil
}

func (v *OidcBindingsClientAddResults) Error() string {
	if v.data.Error == nil {
		return ""
	}
	return *v.data.Error
}

func (v OidcBindingsClient) Add(ctx context.Context, app string, provider string, issuer string, subject_pattern string, claim_conditions []*ClaimCondition, description string) (*OidcBindingsClientAddResults, error) {
	args := OidcBindingsAddArgs{}
	args.data.App = &app
	args.data.Provider = &provider
	args.data.Issuer = &issuer
	args.data.SubjectPattern = &subject_pattern
	x := slices.Clone(claim_conditions)
	args.data.ClaimConditions = &x
	args.data.Description = &description

	var ret oidcBindingsAddResultsData

	err := v.Call(ctx, "Add", &args, &ret)
	if err != nil {
		return nil, err
	}

	return &OidcBindingsClientAddResults{client: v.Client, data: ret}, nil
}

type OidcBindingsClientListResults struct {
	client rpc.Client
	data   oidcBindingsListResultsData
}

func (v *OidcBindingsClientListResults) HasBindings() bool {
	return v.data.Bindings != nil
}

func (v *OidcBindingsClientListResults) Bindings() []*BindingInfo {
	if v.data.Bindings == nil {
		return nil
	}
	return *v.data.Bindings
}

func (v OidcBindingsClient) List(ctx context.Context, app string) (*OidcBindingsClientListResults, error) {
	args := OidcBindingsListArgs{}
	args.data.App = &app

	var ret oidcBindingsListResultsData

	err := v.Call(ctx, "List", &args, &ret)
	if err != nil {
		return nil, err
	}

	return &OidcBindingsClientListResults{client: v.Client, data: ret}, nil
}

type OidcBindingsClientRemoveResults struct {
	client rpc.Client
	data   oidcBindingsRemoveResultsData
}

func (v *OidcBindingsClientRemoveResults) HasSuccess() bool {
	return v.data.Success != nil
}

func (v *OidcBindingsClientRemoveResults) Success() bool {
	if v.data.Success == nil {
		return false
	}
	return *v.data.Success
}

func (v *OidcBindingsClientRemoveResults) HasError() bool {
	return v.data.Error != nil
}

func (v *OidcBindingsClientRemoveResults) Error() string {
	if v.data.Error == nil {
		return ""
	}
	return *v.data.Error
}

func (v OidcBindingsClient) Remove(ctx context.Context, id string) (*OidcBindingsClientRemoveResults, error) {
	args := OidcBindingsRemoveArgs{}
	args.data.Id = &id

	var ret oidcBindingsRemoveResultsData

	err := v.Call(ctx, "Remove", &args, &ret)
	if err != nil {
		return nil, err
	}

	return &OidcBindingsClientRemoveResults{client: v.Client, data: ret}, nil
}

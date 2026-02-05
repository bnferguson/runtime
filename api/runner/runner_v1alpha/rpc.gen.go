package runner_v1alpha

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/fxamacker/cbor/v2"
	rpc "miren.dev/runtime/pkg/rpc"
	"miren.dev/runtime/pkg/rpc/standard"
)

type inviteInfoData struct {
	Id        *string             `cbor:"0,keyasint,omitempty" json:"id,omitempty"`
	Status    *string             `cbor:"1,keyasint,omitempty" json:"status,omitempty"`
	Labels    *[]string           `cbor:"2,keyasint,omitempty" json:"labels,omitempty"`
	ExpiresAt *standard.Timestamp `cbor:"3,keyasint,omitempty" json:"expires_at,omitempty"`
	CreatedAt *standard.Timestamp `cbor:"4,keyasint,omitempty" json:"created_at,omitempty"`
	ClaimedBy *string             `cbor:"5,keyasint,omitempty" json:"claimed_by,omitempty"`
	ClaimedAt *standard.Timestamp `cbor:"6,keyasint,omitempty" json:"claimed_at,omitempty"`
}

type InviteInfo struct {
	data inviteInfoData
}

func (v *InviteInfo) HasId() bool {
	return v.data.Id != nil
}

func (v *InviteInfo) Id() string {
	if v.data.Id == nil {
		return ""
	}
	return *v.data.Id
}

func (v *InviteInfo) SetId(id string) {
	v.data.Id = &id
}

func (v *InviteInfo) HasStatus() bool {
	return v.data.Status != nil
}

func (v *InviteInfo) Status() string {
	if v.data.Status == nil {
		return ""
	}
	return *v.data.Status
}

func (v *InviteInfo) SetStatus(status string) {
	v.data.Status = &status
}

func (v *InviteInfo) HasLabels() bool {
	return v.data.Labels != nil
}

func (v *InviteInfo) Labels() []string {
	if v.data.Labels == nil {
		return nil
	}
	return *v.data.Labels
}

func (v *InviteInfo) SetLabels(labels []string) {
	x := slices.Clone(labels)
	v.data.Labels = &x
}

func (v *InviteInfo) HasExpiresAt() bool {
	return v.data.ExpiresAt != nil
}

func (v *InviteInfo) ExpiresAt() *standard.Timestamp {
	return v.data.ExpiresAt
}

func (v *InviteInfo) SetExpiresAt(expires_at *standard.Timestamp) {
	v.data.ExpiresAt = expires_at
}

func (v *InviteInfo) HasCreatedAt() bool {
	return v.data.CreatedAt != nil
}

func (v *InviteInfo) CreatedAt() *standard.Timestamp {
	return v.data.CreatedAt
}

func (v *InviteInfo) SetCreatedAt(created_at *standard.Timestamp) {
	v.data.CreatedAt = created_at
}

func (v *InviteInfo) HasClaimedBy() bool {
	return v.data.ClaimedBy != nil
}

func (v *InviteInfo) ClaimedBy() string {
	if v.data.ClaimedBy == nil {
		return ""
	}
	return *v.data.ClaimedBy
}

func (v *InviteInfo) SetClaimedBy(claimed_by string) {
	v.data.ClaimedBy = &claimed_by
}

func (v *InviteInfo) HasClaimedAt() bool {
	return v.data.ClaimedAt != nil
}

func (v *InviteInfo) ClaimedAt() *standard.Timestamp {
	return v.data.ClaimedAt
}

func (v *InviteInfo) SetClaimedAt(claimed_at *standard.Timestamp) {
	v.data.ClaimedAt = claimed_at
}

func (v *InviteInfo) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *InviteInfo) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *InviteInfo) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *InviteInfo) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type runnerInfoData struct {
	Id           *string             `cbor:"0,keyasint,omitempty" json:"id,omitempty"`
	RunnerId     *string             `cbor:"1,keyasint,omitempty" json:"runner_id,omitempty"`
	Name         *string             `cbor:"2,keyasint,omitempty" json:"name,omitempty"`
	Status       *string             `cbor:"3,keyasint,omitempty" json:"status,omitempty"`
	Version      *string             `cbor:"4,keyasint,omitempty" json:"version,omitempty"`
	ApiAddress   *string             `cbor:"5,keyasint,omitempty" json:"api_address,omitempty"`
	Labels       *[]string           `cbor:"6,keyasint,omitempty" json:"labels,omitempty"`
	RegisteredAt *standard.Timestamp `cbor:"7,keyasint,omitempty" json:"registered_at,omitempty"`
}

type RunnerInfo struct {
	data runnerInfoData
}

func (v *RunnerInfo) HasId() bool {
	return v.data.Id != nil
}

func (v *RunnerInfo) Id() string {
	if v.data.Id == nil {
		return ""
	}
	return *v.data.Id
}

func (v *RunnerInfo) SetId(id string) {
	v.data.Id = &id
}

func (v *RunnerInfo) HasRunnerId() bool {
	return v.data.RunnerId != nil
}

func (v *RunnerInfo) RunnerId() string {
	if v.data.RunnerId == nil {
		return ""
	}
	return *v.data.RunnerId
}

func (v *RunnerInfo) SetRunnerId(runner_id string) {
	v.data.RunnerId = &runner_id
}

func (v *RunnerInfo) HasName() bool {
	return v.data.Name != nil
}

func (v *RunnerInfo) Name() string {
	if v.data.Name == nil {
		return ""
	}
	return *v.data.Name
}

func (v *RunnerInfo) SetName(name string) {
	v.data.Name = &name
}

func (v *RunnerInfo) HasStatus() bool {
	return v.data.Status != nil
}

func (v *RunnerInfo) Status() string {
	if v.data.Status == nil {
		return ""
	}
	return *v.data.Status
}

func (v *RunnerInfo) SetStatus(status string) {
	v.data.Status = &status
}

func (v *RunnerInfo) HasVersion() bool {
	return v.data.Version != nil
}

func (v *RunnerInfo) Version() string {
	if v.data.Version == nil {
		return ""
	}
	return *v.data.Version
}

func (v *RunnerInfo) SetVersion(version string) {
	v.data.Version = &version
}

func (v *RunnerInfo) HasApiAddress() bool {
	return v.data.ApiAddress != nil
}

func (v *RunnerInfo) ApiAddress() string {
	if v.data.ApiAddress == nil {
		return ""
	}
	return *v.data.ApiAddress
}

func (v *RunnerInfo) SetApiAddress(api_address string) {
	v.data.ApiAddress = &api_address
}

func (v *RunnerInfo) HasLabels() bool {
	return v.data.Labels != nil
}

func (v *RunnerInfo) Labels() []string {
	if v.data.Labels == nil {
		return nil
	}
	return *v.data.Labels
}

func (v *RunnerInfo) SetLabels(labels []string) {
	x := slices.Clone(labels)
	v.data.Labels = &x
}

func (v *RunnerInfo) HasRegisteredAt() bool {
	return v.data.RegisteredAt != nil
}

func (v *RunnerInfo) RegisteredAt() *standard.Timestamp {
	return v.data.RegisteredAt
}

func (v *RunnerInfo) SetRegisteredAt(registered_at *standard.Timestamp) {
	v.data.RegisteredAt = registered_at
}

func (v *RunnerInfo) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *RunnerInfo) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *RunnerInfo) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *RunnerInfo) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type runnerRegistrationCreateInviteArgsData struct {
	Labels         *[]string `cbor:"0,keyasint,omitempty" json:"labels,omitempty"`
	ExpiresInHours *int32    `cbor:"1,keyasint,omitempty" json:"expires_in_hours,omitempty"`
}

type RunnerRegistrationCreateInviteArgs struct {
	call rpc.Call
	data runnerRegistrationCreateInviteArgsData
}

func (v *RunnerRegistrationCreateInviteArgs) HasLabels() bool {
	return v.data.Labels != nil
}

func (v *RunnerRegistrationCreateInviteArgs) Labels() []string {
	if v.data.Labels == nil {
		return nil
	}
	return *v.data.Labels
}

func (v *RunnerRegistrationCreateInviteArgs) HasExpiresInHours() bool {
	return v.data.ExpiresInHours != nil
}

func (v *RunnerRegistrationCreateInviteArgs) ExpiresInHours() int32 {
	if v.data.ExpiresInHours == nil {
		return 0
	}
	return *v.data.ExpiresInHours
}

func (v *RunnerRegistrationCreateInviteArgs) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *RunnerRegistrationCreateInviteArgs) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *RunnerRegistrationCreateInviteArgs) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *RunnerRegistrationCreateInviteArgs) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type runnerRegistrationCreateInviteResultsData struct {
	Code      *string             `cbor:"0,keyasint,omitempty" json:"code,omitempty"`
	ExpiresAt *standard.Timestamp `cbor:"1,keyasint,omitempty" json:"expires_at,omitempty"`
}

type RunnerRegistrationCreateInviteResults struct {
	call rpc.Call
	data runnerRegistrationCreateInviteResultsData
}

func (v *RunnerRegistrationCreateInviteResults) SetCode(code string) {
	v.data.Code = &code
}

func (v *RunnerRegistrationCreateInviteResults) SetExpiresAt(expires_at *standard.Timestamp) {
	v.data.ExpiresAt = expires_at
}

func (v *RunnerRegistrationCreateInviteResults) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *RunnerRegistrationCreateInviteResults) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *RunnerRegistrationCreateInviteResults) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *RunnerRegistrationCreateInviteResults) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type runnerRegistrationJoinArgsData struct {
	Code       *string   `cbor:"0,keyasint,omitempty" json:"code,omitempty"`
	RunnerId   *string   `cbor:"1,keyasint,omitempty" json:"runner_id,omitempty"`
	ListenAddr *string   `cbor:"2,keyasint,omitempty" json:"listen_addr,omitempty"`
	Version    *string   `cbor:"3,keyasint,omitempty" json:"version,omitempty"`
	Labels     *[]string `cbor:"4,keyasint,omitempty" json:"labels,omitempty"`
}

type RunnerRegistrationJoinArgs struct {
	call rpc.Call
	data runnerRegistrationJoinArgsData
}

func (v *RunnerRegistrationJoinArgs) HasCode() bool {
	return v.data.Code != nil
}

func (v *RunnerRegistrationJoinArgs) Code() string {
	if v.data.Code == nil {
		return ""
	}
	return *v.data.Code
}

func (v *RunnerRegistrationJoinArgs) HasRunnerId() bool {
	return v.data.RunnerId != nil
}

func (v *RunnerRegistrationJoinArgs) RunnerId() string {
	if v.data.RunnerId == nil {
		return ""
	}
	return *v.data.RunnerId
}

func (v *RunnerRegistrationJoinArgs) HasListenAddr() bool {
	return v.data.ListenAddr != nil
}

func (v *RunnerRegistrationJoinArgs) ListenAddr() string {
	if v.data.ListenAddr == nil {
		return ""
	}
	return *v.data.ListenAddr
}

func (v *RunnerRegistrationJoinArgs) HasVersion() bool {
	return v.data.Version != nil
}

func (v *RunnerRegistrationJoinArgs) Version() string {
	if v.data.Version == nil {
		return ""
	}
	return *v.data.Version
}

func (v *RunnerRegistrationJoinArgs) HasLabels() bool {
	return v.data.Labels != nil
}

func (v *RunnerRegistrationJoinArgs) Labels() []string {
	if v.data.Labels == nil {
		return nil
	}
	return *v.data.Labels
}

func (v *RunnerRegistrationJoinArgs) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *RunnerRegistrationJoinArgs) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *RunnerRegistrationJoinArgs) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *RunnerRegistrationJoinArgs) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type runnerRegistrationJoinResultsData struct {
	CertPem         *[]byte `cbor:"0,keyasint,omitempty" json:"cert_pem,omitempty"`
	KeyPem          *[]byte `cbor:"1,keyasint,omitempty" json:"key_pem,omitempty"`
	CaPem           *[]byte `cbor:"2,keyasint,omitempty" json:"ca_pem,omitempty"`
	CoordinatorAddr *string `cbor:"3,keyasint,omitempty" json:"coordinator_addr,omitempty"`
	RunnerId        *string `cbor:"4,keyasint,omitempty" json:"runner_id,omitempty"`
	Error           *string `cbor:"5,keyasint,omitempty" json:"error,omitempty"`
}

type RunnerRegistrationJoinResults struct {
	call rpc.Call
	data runnerRegistrationJoinResultsData
}

func (v *RunnerRegistrationJoinResults) SetCertPem(cert_pem []byte) {
	x := slices.Clone(cert_pem)
	v.data.CertPem = &x
}

func (v *RunnerRegistrationJoinResults) SetKeyPem(key_pem []byte) {
	x := slices.Clone(key_pem)
	v.data.KeyPem = &x
}

func (v *RunnerRegistrationJoinResults) SetCaPem(ca_pem []byte) {
	x := slices.Clone(ca_pem)
	v.data.CaPem = &x
}

func (v *RunnerRegistrationJoinResults) SetCoordinatorAddr(coordinator_addr string) {
	v.data.CoordinatorAddr = &coordinator_addr
}

func (v *RunnerRegistrationJoinResults) SetRunnerId(runner_id string) {
	v.data.RunnerId = &runner_id
}

func (v *RunnerRegistrationJoinResults) SetError(error string) {
	v.data.Error = &error
}

func (v *RunnerRegistrationJoinResults) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *RunnerRegistrationJoinResults) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *RunnerRegistrationJoinResults) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *RunnerRegistrationJoinResults) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type runnerRegistrationListInvitesArgsData struct{}

type RunnerRegistrationListInvitesArgs struct {
	call rpc.Call
	data runnerRegistrationListInvitesArgsData
}

func (v *RunnerRegistrationListInvitesArgs) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *RunnerRegistrationListInvitesArgs) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *RunnerRegistrationListInvitesArgs) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *RunnerRegistrationListInvitesArgs) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type runnerRegistrationListInvitesResultsData struct {
	Invites *[]*InviteInfo `cbor:"0,keyasint,omitempty" json:"invites,omitempty"`
}

type RunnerRegistrationListInvitesResults struct {
	call rpc.Call
	data runnerRegistrationListInvitesResultsData
}

func (v *RunnerRegistrationListInvitesResults) SetInvites(invites []*InviteInfo) {
	x := slices.Clone(invites)
	v.data.Invites = &x
}

func (v *RunnerRegistrationListInvitesResults) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *RunnerRegistrationListInvitesResults) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *RunnerRegistrationListInvitesResults) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *RunnerRegistrationListInvitesResults) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type runnerRegistrationRevokeInviteArgsData struct {
	InviteId *string `cbor:"0,keyasint,omitempty" json:"invite_id,omitempty"`
}

type RunnerRegistrationRevokeInviteArgs struct {
	call rpc.Call
	data runnerRegistrationRevokeInviteArgsData
}

func (v *RunnerRegistrationRevokeInviteArgs) HasInviteId() bool {
	return v.data.InviteId != nil
}

func (v *RunnerRegistrationRevokeInviteArgs) InviteId() string {
	if v.data.InviteId == nil {
		return ""
	}
	return *v.data.InviteId
}

func (v *RunnerRegistrationRevokeInviteArgs) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *RunnerRegistrationRevokeInviteArgs) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *RunnerRegistrationRevokeInviteArgs) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *RunnerRegistrationRevokeInviteArgs) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type runnerRegistrationRevokeInviteResultsData struct {
	Success *bool   `cbor:"0,keyasint,omitempty" json:"success,omitempty"`
	Error   *string `cbor:"1,keyasint,omitempty" json:"error,omitempty"`
}

type RunnerRegistrationRevokeInviteResults struct {
	call rpc.Call
	data runnerRegistrationRevokeInviteResultsData
}

func (v *RunnerRegistrationRevokeInviteResults) SetSuccess(success bool) {
	v.data.Success = &success
}

func (v *RunnerRegistrationRevokeInviteResults) SetError(error string) {
	v.data.Error = &error
}

func (v *RunnerRegistrationRevokeInviteResults) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *RunnerRegistrationRevokeInviteResults) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *RunnerRegistrationRevokeInviteResults) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *RunnerRegistrationRevokeInviteResults) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type runnerRegistrationListRunnersArgsData struct{}

type RunnerRegistrationListRunnersArgs struct {
	call rpc.Call
	data runnerRegistrationListRunnersArgsData
}

func (v *RunnerRegistrationListRunnersArgs) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *RunnerRegistrationListRunnersArgs) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *RunnerRegistrationListRunnersArgs) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *RunnerRegistrationListRunnersArgs) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type runnerRegistrationListRunnersResultsData struct {
	Runners *[]*RunnerInfo `cbor:"0,keyasint,omitempty" json:"runners,omitempty"`
}

type RunnerRegistrationListRunnersResults struct {
	call rpc.Call
	data runnerRegistrationListRunnersResultsData
}

func (v *RunnerRegistrationListRunnersResults) SetRunners(runners []*RunnerInfo) {
	x := slices.Clone(runners)
	v.data.Runners = &x
}

func (v *RunnerRegistrationListRunnersResults) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.data)
}

func (v *RunnerRegistrationListRunnersResults) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, &v.data)
}

func (v *RunnerRegistrationListRunnersResults) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.data)
}

func (v *RunnerRegistrationListRunnersResults) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.data)
}

type RunnerRegistrationCreateInvite struct {
	rpc.Call
	args    RunnerRegistrationCreateInviteArgs
	results RunnerRegistrationCreateInviteResults
}

func (t *RunnerRegistrationCreateInvite) Args() *RunnerRegistrationCreateInviteArgs {
	args := &t.args
	if args.call != nil {
		return args
	}
	args.call = t.Call
	t.Call.Args(args)
	return args
}

func (t *RunnerRegistrationCreateInvite) Results() *RunnerRegistrationCreateInviteResults {
	results := &t.results
	if results.call != nil {
		return results
	}
	results.call = t.Call
	t.Call.Results(results)
	return results
}

type RunnerRegistrationJoin struct {
	rpc.Call
	args    RunnerRegistrationJoinArgs
	results RunnerRegistrationJoinResults
}

func (t *RunnerRegistrationJoin) Args() *RunnerRegistrationJoinArgs {
	args := &t.args
	if args.call != nil {
		return args
	}
	args.call = t.Call
	t.Call.Args(args)
	return args
}

func (t *RunnerRegistrationJoin) Results() *RunnerRegistrationJoinResults {
	results := &t.results
	if results.call != nil {
		return results
	}
	results.call = t.Call
	t.Call.Results(results)
	return results
}

type RunnerRegistrationListInvites struct {
	rpc.Call
	args    RunnerRegistrationListInvitesArgs
	results RunnerRegistrationListInvitesResults
}

func (t *RunnerRegistrationListInvites) Args() *RunnerRegistrationListInvitesArgs {
	args := &t.args
	if args.call != nil {
		return args
	}
	args.call = t.Call
	t.Call.Args(args)
	return args
}

func (t *RunnerRegistrationListInvites) Results() *RunnerRegistrationListInvitesResults {
	results := &t.results
	if results.call != nil {
		return results
	}
	results.call = t.Call
	t.Call.Results(results)
	return results
}

type RunnerRegistrationRevokeInvite struct {
	rpc.Call
	args    RunnerRegistrationRevokeInviteArgs
	results RunnerRegistrationRevokeInviteResults
}

func (t *RunnerRegistrationRevokeInvite) Args() *RunnerRegistrationRevokeInviteArgs {
	args := &t.args
	if args.call != nil {
		return args
	}
	args.call = t.Call
	t.Call.Args(args)
	return args
}

func (t *RunnerRegistrationRevokeInvite) Results() *RunnerRegistrationRevokeInviteResults {
	results := &t.results
	if results.call != nil {
		return results
	}
	results.call = t.Call
	t.Call.Results(results)
	return results
}

type RunnerRegistrationListRunners struct {
	rpc.Call
	args    RunnerRegistrationListRunnersArgs
	results RunnerRegistrationListRunnersResults
}

func (t *RunnerRegistrationListRunners) Args() *RunnerRegistrationListRunnersArgs {
	args := &t.args
	if args.call != nil {
		return args
	}
	args.call = t.Call
	t.Call.Args(args)
	return args
}

func (t *RunnerRegistrationListRunners) Results() *RunnerRegistrationListRunnersResults {
	results := &t.results
	if results.call != nil {
		return results
	}
	results.call = t.Call
	t.Call.Results(results)
	return results
}

type RunnerRegistration interface {
	CreateInvite(ctx context.Context, state *RunnerRegistrationCreateInvite) error
	Join(ctx context.Context, state *RunnerRegistrationJoin) error
	ListInvites(ctx context.Context, state *RunnerRegistrationListInvites) error
	RevokeInvite(ctx context.Context, state *RunnerRegistrationRevokeInvite) error
	ListRunners(ctx context.Context, state *RunnerRegistrationListRunners) error
}

type reexportRunnerRegistration struct {
	client rpc.Client
}

func (reexportRunnerRegistration) CreateInvite(ctx context.Context, state *RunnerRegistrationCreateInvite) error {
	panic("not implemented")
}

func (reexportRunnerRegistration) Join(ctx context.Context, state *RunnerRegistrationJoin) error {
	panic("not implemented")
}

func (reexportRunnerRegistration) ListInvites(ctx context.Context, state *RunnerRegistrationListInvites) error {
	panic("not implemented")
}

func (reexportRunnerRegistration) RevokeInvite(ctx context.Context, state *RunnerRegistrationRevokeInvite) error {
	panic("not implemented")
}

func (reexportRunnerRegistration) ListRunners(ctx context.Context, state *RunnerRegistrationListRunners) error {
	panic("not implemented")
}

func (t reexportRunnerRegistration) CapabilityClient() rpc.Client {
	return t.client
}

func AdaptRunnerRegistration(t RunnerRegistration) *rpc.Interface {
	methods := []rpc.Method{
		{
			Name:          "CreateInvite",
			InterfaceName: "RunnerRegistration",
			Index:         0,
			Public:        false,
			Handler: func(ctx context.Context, call rpc.Call) error {
				return t.CreateInvite(ctx, &RunnerRegistrationCreateInvite{Call: call})
			},
		},
		{
			Name:          "Join",
			InterfaceName: "RunnerRegistration",
			Index:         1,
			Public:        true,
			Handler: func(ctx context.Context, call rpc.Call) error {
				return t.Join(ctx, &RunnerRegistrationJoin{Call: call})
			},
		},
		{
			Name:          "ListInvites",
			InterfaceName: "RunnerRegistration",
			Index:         2,
			Public:        false,
			Handler: func(ctx context.Context, call rpc.Call) error {
				return t.ListInvites(ctx, &RunnerRegistrationListInvites{Call: call})
			},
		},
		{
			Name:          "RevokeInvite",
			InterfaceName: "RunnerRegistration",
			Index:         3,
			Public:        false,
			Handler: func(ctx context.Context, call rpc.Call) error {
				return t.RevokeInvite(ctx, &RunnerRegistrationRevokeInvite{Call: call})
			},
		},
		{
			Name:          "ListRunners",
			InterfaceName: "RunnerRegistration",
			Index:         4,
			Public:        false,
			Handler: func(ctx context.Context, call rpc.Call) error {
				return t.ListRunners(ctx, &RunnerRegistrationListRunners{Call: call})
			},
		},
	}

	return rpc.NewInterface(methods, t)
}

type RunnerRegistrationClient struct {
	rpc.Client
}

func NewRunnerRegistrationClient(client rpc.Client) *RunnerRegistrationClient {
	return &RunnerRegistrationClient{Client: client}
}

func (c RunnerRegistrationClient) Export() RunnerRegistration {
	return reexportRunnerRegistration{client: c.Client}
}

type RunnerRegistrationClientCreateInviteResults struct {
	client rpc.Client
	data   runnerRegistrationCreateInviteResultsData
}

func (v *RunnerRegistrationClientCreateInviteResults) HasCode() bool {
	return v.data.Code != nil
}

func (v *RunnerRegistrationClientCreateInviteResults) Code() string {
	if v.data.Code == nil {
		return ""
	}
	return *v.data.Code
}

func (v *RunnerRegistrationClientCreateInviteResults) HasExpiresAt() bool {
	return v.data.ExpiresAt != nil
}

func (v *RunnerRegistrationClientCreateInviteResults) ExpiresAt() *standard.Timestamp {
	return v.data.ExpiresAt
}

func (v RunnerRegistrationClient) CreateInvite(ctx context.Context, labels []string, expires_in_hours int32) (*RunnerRegistrationClientCreateInviteResults, error) {
	args := RunnerRegistrationCreateInviteArgs{}
	x := slices.Clone(labels)
	args.data.Labels = &x
	args.data.ExpiresInHours = &expires_in_hours

	var ret runnerRegistrationCreateInviteResultsData

	err := v.Call(ctx, "CreateInvite", &args, &ret)
	if err != nil {
		return nil, err
	}

	return &RunnerRegistrationClientCreateInviteResults{client: v.Client, data: ret}, nil
}

type RunnerRegistrationClientJoinResults struct {
	client rpc.Client
	data   runnerRegistrationJoinResultsData
}

func (v *RunnerRegistrationClientJoinResults) HasCertPem() bool {
	return v.data.CertPem != nil
}

func (v *RunnerRegistrationClientJoinResults) CertPem() []byte {
	if v.data.CertPem == nil {
		return nil
	}
	return *v.data.CertPem
}

func (v *RunnerRegistrationClientJoinResults) HasKeyPem() bool {
	return v.data.KeyPem != nil
}

func (v *RunnerRegistrationClientJoinResults) KeyPem() []byte {
	if v.data.KeyPem == nil {
		return nil
	}
	return *v.data.KeyPem
}

func (v *RunnerRegistrationClientJoinResults) HasCaPem() bool {
	return v.data.CaPem != nil
}

func (v *RunnerRegistrationClientJoinResults) CaPem() []byte {
	if v.data.CaPem == nil {
		return nil
	}
	return *v.data.CaPem
}

func (v *RunnerRegistrationClientJoinResults) HasCoordinatorAddr() bool {
	return v.data.CoordinatorAddr != nil
}

func (v *RunnerRegistrationClientJoinResults) CoordinatorAddr() string {
	if v.data.CoordinatorAddr == nil {
		return ""
	}
	return *v.data.CoordinatorAddr
}

func (v *RunnerRegistrationClientJoinResults) HasRunnerId() bool {
	return v.data.RunnerId != nil
}

func (v *RunnerRegistrationClientJoinResults) RunnerId() string {
	if v.data.RunnerId == nil {
		return ""
	}
	return *v.data.RunnerId
}

func (v *RunnerRegistrationClientJoinResults) HasError() bool {
	return v.data.Error != nil
}

func (v *RunnerRegistrationClientJoinResults) Error() string {
	if v.data.Error == nil {
		return ""
	}
	return *v.data.Error
}

func (v RunnerRegistrationClient) Join(ctx context.Context, code string, runner_id string, listen_addr string, version string, labels []string) (*RunnerRegistrationClientJoinResults, error) {
	args := RunnerRegistrationJoinArgs{}
	args.data.Code = &code
	args.data.RunnerId = &runner_id
	args.data.ListenAddr = &listen_addr
	args.data.Version = &version
	x := slices.Clone(labels)
	args.data.Labels = &x

	var ret runnerRegistrationJoinResultsData

	err := v.Call(ctx, "Join", &args, &ret)
	if err != nil {
		return nil, err
	}

	return &RunnerRegistrationClientJoinResults{client: v.Client, data: ret}, nil
}

type RunnerRegistrationClientListInvitesResults struct {
	client rpc.Client
	data   runnerRegistrationListInvitesResultsData
}

func (v *RunnerRegistrationClientListInvitesResults) HasInvites() bool {
	return v.data.Invites != nil
}

func (v *RunnerRegistrationClientListInvitesResults) Invites() []*InviteInfo {
	if v.data.Invites == nil {
		return nil
	}
	return *v.data.Invites
}

func (v RunnerRegistrationClient) ListInvites(ctx context.Context) (*RunnerRegistrationClientListInvitesResults, error) {
	args := RunnerRegistrationListInvitesArgs{}

	var ret runnerRegistrationListInvitesResultsData

	err := v.Call(ctx, "ListInvites", &args, &ret)
	if err != nil {
		return nil, err
	}

	return &RunnerRegistrationClientListInvitesResults{client: v.Client, data: ret}, nil
}

type RunnerRegistrationClientRevokeInviteResults struct {
	client rpc.Client
	data   runnerRegistrationRevokeInviteResultsData
}

func (v *RunnerRegistrationClientRevokeInviteResults) HasSuccess() bool {
	return v.data.Success != nil
}

func (v *RunnerRegistrationClientRevokeInviteResults) Success() bool {
	if v.data.Success == nil {
		return false
	}
	return *v.data.Success
}

func (v *RunnerRegistrationClientRevokeInviteResults) HasError() bool {
	return v.data.Error != nil
}

func (v *RunnerRegistrationClientRevokeInviteResults) Error() string {
	if v.data.Error == nil {
		return ""
	}
	return *v.data.Error
}

func (v RunnerRegistrationClient) RevokeInvite(ctx context.Context, invite_id string) (*RunnerRegistrationClientRevokeInviteResults, error) {
	args := RunnerRegistrationRevokeInviteArgs{}
	args.data.InviteId = &invite_id

	var ret runnerRegistrationRevokeInviteResultsData

	err := v.Call(ctx, "RevokeInvite", &args, &ret)
	if err != nil {
		return nil, err
	}

	return &RunnerRegistrationClientRevokeInviteResults{client: v.Client, data: ret}, nil
}

type RunnerRegistrationClientListRunnersResults struct {
	client rpc.Client
	data   runnerRegistrationListRunnersResultsData
}

func (v *RunnerRegistrationClientListRunnersResults) HasRunners() bool {
	return v.data.Runners != nil
}

func (v *RunnerRegistrationClientListRunnersResults) Runners() []*RunnerInfo {
	if v.data.Runners == nil {
		return nil
	}
	return *v.data.Runners
}

func (v RunnerRegistrationClient) ListRunners(ctx context.Context) (*RunnerRegistrationClientListRunnersResults, error) {
	args := RunnerRegistrationListRunnersArgs{}

	var ret runnerRegistrationListRunnersResultsData

	err := v.Call(ctx, "ListRunners", &args, &ret)
	if err != nil {
		return nil, err
	}

	return &RunnerRegistrationClientListRunnersResults{client: v.Client, data: ret}, nil
}

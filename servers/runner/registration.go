package runner

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"miren.dev/runtime/api/compute/compute_v1alpha"
	"miren.dev/runtime/api/entityserver/entityserver_v1alpha"
	"miren.dev/runtime/api/runner/runner_v1alpha"
	"miren.dev/runtime/pkg/caauth"
	"miren.dev/runtime/pkg/cond"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/entity/types"
	"miren.dev/runtime/pkg/joincode"
	"miren.dev/runtime/pkg/rpc/standard"
)

const (
	DefaultInviteExpiryHours = 1
	MaxInviteExpiryHours     = 168 // 7 days
)

type RegistrationServer struct {
	Log             *slog.Logger
	Authority       *caauth.Authority
	EAC             *entityserver_v1alpha.EntityAccessClient
	CoordinatorAddr string
	EtcdEndpoints   []string
	EtcdPrefix      string
	NetworkBackend  string
}

var _ runner_v1alpha.RunnerRegistration = (*RegistrationServer)(nil)

func NewRegistrationServer(log *slog.Logger, authority *caauth.Authority, eac *entityserver_v1alpha.EntityAccessClient, coordinatorAddr string, etcdEndpoints []string, etcdPrefix string, networkBackend string) *RegistrationServer {
	return &RegistrationServer{
		Log:             log.With("module", "runner-registration"),
		Authority:       authority,
		EAC:             eac,
		CoordinatorAddr: coordinatorAddr,
		EtcdEndpoints:   etcdEndpoints,
		EtcdPrefix:      etcdPrefix,
		NetworkBackend:  networkBackend,
	}
}

func (s *RegistrationServer) CreateInvite(ctx context.Context, req *runner_v1alpha.RunnerRegistrationCreateInvite) error {
	args := req.Args()
	results := req.Results()

	expiryHours := int32(DefaultInviteExpiryHours)
	if args.HasExpiresInHours() && args.ExpiresInHours() > 0 {
		expiryHours = args.ExpiresInHours()
	}
	if expiryHours > MaxInviteExpiryHours {
		return cond.ValidationFailure("invalid-expiry", fmt.Sprintf("expiry cannot exceed %d hours", MaxInviteExpiryHours))
	}

	code, err := joincode.Generate()
	if err != nil {
		s.Log.Error("Failed to generate join code", "error", err)
		return cond.Error("failed to generate join code")
	}

	codeHash := joincode.Hash(code)
	now := time.Now()
	expiresAt := now.Add(time.Duration(expiryHours) * time.Hour)

	invite := &runner_v1alpha.RunnerInvite{
		CodeHash:  codeHash,
		Status:    runner_v1alpha.PENDING,
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}

	if args.HasLabels() {
		for _, labelStr := range args.Labels() {
			parts := strings.SplitN(labelStr, "=", 2)
			if len(parts) == 2 {
				invite.Labels = append(invite.Labels, types.Label{Key: parts[0], Value: parts[1]})
			}
		}
	}

	attrs := invite.Encode()
	rpcEntity := &entityserver_v1alpha.Entity{}
	rpcEntity.SetAttrs(attrs)

	putResp, err := s.EAC.Put(ctx, rpcEntity)
	if err != nil {
		s.Log.Error("Failed to create invite entity", "error", err)
		return cond.Error("failed to create invite")
	}

	s.Log.Info("Created runner invite",
		"invite_id", putResp.Id(),
		"expires_at", expiresAt.Format(time.RFC3339),
		"label_count", len(invite.Labels))

	results.SetCode(code)
	results.SetExpiresAt(standard.ToTimestamp(expiresAt))

	return nil
}

func (s *RegistrationServer) Join(ctx context.Context, req *runner_v1alpha.RunnerRegistrationJoin) error {
	args := req.Args()
	results := req.Results()

	if !args.HasCode() || args.Code() == "" {
		results.SetError("join code is required")
		return nil
	}

	code := args.Code()
	if !joincode.Validate(code) {
		results.SetError("invalid join code format")
		return nil
	}

	codeHash := joincode.Hash(code)

	invite, inviteRevision, err := s.findInviteByHash(ctx, codeHash)
	if err != nil {
		s.Log.Error("Failed to find invite", "error", err)
		results.SetError("failed to validate invite")
		return nil
	}
	if invite == nil {
		results.SetError("invalid or expired join code")
		return nil
	}

	if invite.Status != runner_v1alpha.PENDING {
		results.SetError("join code has already been used or revoked")
		return nil
	}

	if time.Now().After(invite.ExpiresAt) {
		results.SetError("join code has expired")
		return nil
	}

	runnerID := args.RunnerId()
	if runnerID == "" {
		runnerID = uuid.New().String()
	} else if _, err := uuid.Parse(runnerID); err != nil {
		results.SetError("runner_id must be a valid UUID")
		return nil
	}

	listenAddr := ""
	if args.HasListenAddr() {
		listenAddr = args.ListenAddr()
	}

	version := ""
	if args.HasVersion() {
		version = args.Version()
	}

	// Claim the invite first (with CAS via revision) to prevent concurrent joins
	// from minting multiple valid certificates for the same invite
	invite.Status = runner_v1alpha.CLAIMED
	invite.ClaimedBy = runnerID
	invite.ClaimedAt = time.Now()

	updateAttrs := invite.Encode()
	updateEntity := &entityserver_v1alpha.Entity{}
	updateEntity.SetId(string(invite.ID))
	updateEntity.SetAttrs(updateAttrs)
	updateEntity.SetRevision(inviteRevision)

	_, err = s.EAC.Put(ctx, updateEntity)
	if err != nil {
		s.Log.Error("Failed to update invite status", "error", err, "invite_id", invite.ID)
		results.SetError("failed to complete registration")
		return nil
	}

	// Now that invite is claimed, issue the certificate
	runnerIDPrefix := runnerID
	if len(runnerIDPrefix) > 8 {
		runnerIDPrefix = runnerIDPrefix[:8]
	}
	certName := fmt.Sprintf("runner-%s", runnerIDPrefix)
	cc, err := s.Authority.IssueCertificate(caauth.Options{
		CommonName:   certName,
		Organization: "miren",
		ValidFor:     365 * 24 * time.Hour,
	})
	if err != nil {
		s.Log.Error("Failed to issue certificate", "error", err, "runner_id", runnerID)
		results.SetError("failed to issue certificate")
		return nil
	}

	labels := make(types.Labels, 0, len(invite.Labels))
	labels = append(labels, invite.Labels...)
	if args.HasLabels() {
		for _, labelStr := range args.Labels() {
			parts := strings.SplitN(labelStr, "=", 2)
			if len(parts) == 2 {
				labels = append(labels, types.Label{Key: parts[0], Value: parts[1]})
			}
		}
	}

	node := &compute_v1alpha.Node{
		RunnerId:     runnerID,
		Status:       compute_v1alpha.READY,
		ApiAddress:   listenAddr,
		Version:      version,
		RegisteredAt: time.Now(),
		Constraints:  labels,
	}

	nodeAttrs := node.Encode()
	nodeEntity := &entityserver_v1alpha.Entity{}
	nodeEntity.SetAttrs(nodeAttrs)

	nodePutResp, err := s.EAC.Put(ctx, nodeEntity)
	if err != nil {
		s.Log.Error("Failed to create node entity", "error", err, "runner_id", runnerID)
		results.SetError("failed to register runner")
		return nil
	}

	s.Log.Info("Runner joined successfully",
		"runner_id", runnerID,
		"node_id", nodePutResp.Id(),
		"listen_addr", listenAddr,
		"version", version,
		"label_count", len(labels))

	results.SetCertPem(cc.CertPEM)
	results.SetKeyPem(cc.KeyPEM)
	results.SetCaPem(cc.CACert)
	results.SetCoordinatorAddr(s.CoordinatorAddr)
	results.SetRunnerId(runnerID)

	// Provide network configuration for distributed runners
	if len(s.EtcdEndpoints) > 0 {
		results.SetEtcdEndpoints(s.EtcdEndpoints)
	}
	if s.EtcdPrefix != "" {
		results.SetEtcdPrefix(s.EtcdPrefix + "/sub/flannel")
	}
	if s.NetworkBackend != "" {
		results.SetNetworkBackend(s.NetworkBackend)
	}

	return nil
}

func (s *RegistrationServer) ListInvites(ctx context.Context, req *runner_v1alpha.RunnerRegistrationListInvites) error {
	results := req.Results()

	listResp, err := s.EAC.List(ctx, entity.Ref(entity.EntityKind, runner_v1alpha.KindRunnerInvite))
	if err != nil {
		s.Log.Error("Failed to list invites", "error", err)
		return cond.Error("failed to list invites")
	}

	now := time.Now()
	invites := make([]*runner_v1alpha.InviteInfo, 0)

	for _, e := range listResp.Values() {
		var invite runner_v1alpha.RunnerInvite
		decodeEntity(e, &invite)

		if invite.Status == runner_v1alpha.PENDING && now.After(invite.ExpiresAt) {
			continue
		}

		info := &runner_v1alpha.InviteInfo{}
		info.SetId(string(invite.ID))
		info.SetStatus(string(invite.Status))

		labelStrs := make([]string, 0, len(invite.Labels))
		for _, l := range invite.Labels {
			labelStrs = append(labelStrs, fmt.Sprintf("%s=%s", l.Key, l.Value))
		}
		info.SetLabels(labelStrs)

		info.SetExpiresAt(standard.ToTimestamp(invite.ExpiresAt))
		info.SetCreatedAt(standard.ToTimestamp(invite.CreatedAt))

		if invite.ClaimedBy != "" {
			info.SetClaimedBy(invite.ClaimedBy)
			info.SetClaimedAt(standard.ToTimestamp(invite.ClaimedAt))
		}

		invites = append(invites, info)
	}

	results.SetInvites(invites)
	return nil
}

func (s *RegistrationServer) RevokeInvite(ctx context.Context, req *runner_v1alpha.RunnerRegistrationRevokeInvite) error {
	args := req.Args()
	results := req.Results()

	if !args.HasInviteId() || args.InviteId() == "" {
		results.SetError("invite_id is required")
		return nil
	}

	inviteID := args.InviteId()

	inviteResp, err := s.EAC.Get(ctx, inviteID)
	if err != nil {
		s.Log.Error("Failed to get invite", "invite_id", inviteID, "error", err)
		results.SetError("invite not found")
		return nil
	}

	var invite runner_v1alpha.RunnerInvite
	decodeEntity(inviteResp.Entity(), &invite)

	if invite.Status != runner_v1alpha.PENDING {
		results.SetError(fmt.Sprintf("cannot revoke invite in %s state", invite.Status))
		return nil
	}

	invite.Status = runner_v1alpha.REVOKED

	updateAttrs := invite.Encode()
	updateEntity := &entityserver_v1alpha.Entity{}
	updateEntity.SetId(inviteID)
	updateEntity.SetAttrs(updateAttrs)
	updateEntity.SetRevision(inviteResp.Entity().Revision())

	_, err = s.EAC.Put(ctx, updateEntity)
	if err != nil {
		s.Log.Error("Failed to revoke invite", "invite_id", inviteID, "error", err)
		results.SetError("failed to revoke invite")
		return nil
	}

	s.Log.Info("Revoked runner invite", "invite_id", inviteID)
	results.SetSuccess(true)
	return nil
}

func (s *RegistrationServer) ListRunners(ctx context.Context, req *runner_v1alpha.RunnerRegistrationListRunners) error {
	results := req.Results()

	listResp, err := s.EAC.List(ctx, entity.Ref(entity.EntityKind, compute_v1alpha.KindNode))
	if err != nil {
		s.Log.Error("Failed to list nodes", "error", err)
		return cond.Error("failed to list runners")
	}

	runners := make([]*runner_v1alpha.RunnerInfo, 0)

	for _, e := range listResp.Values() {
		var node compute_v1alpha.Node
		decodeEntity(e, &node)

		if node.RunnerId == "" {
			continue
		}

		info := &runner_v1alpha.RunnerInfo{}
		info.SetId(string(node.ID))
		info.SetRunnerId(node.RunnerId)
		info.SetName(string(node.ID))
		info.SetStatus(string(node.Status))
		info.SetVersion(node.Version)
		info.SetApiAddress(node.ApiAddress)

		labelStrs := make([]string, 0, len(node.Constraints))
		for _, l := range node.Constraints {
			labelStrs = append(labelStrs, fmt.Sprintf("%s=%s", l.Key, l.Value))
		}
		info.SetLabels(labelStrs)

		if !node.RegisteredAt.IsZero() {
			info.SetRegisteredAt(standard.ToTimestamp(node.RegisteredAt))
		}

		runners = append(runners, info)
	}

	results.SetRunners(runners)
	return nil
}

func (s *RegistrationServer) findInviteByHash(ctx context.Context, codeHash string) (*runner_v1alpha.RunnerInvite, int64, error) {
	listResp, err := s.EAC.List(ctx, entity.Ref(entity.EntityKind, runner_v1alpha.KindRunnerInvite))
	if err != nil {
		return nil, 0, err
	}

	for _, e := range listResp.Values() {
		var invite runner_v1alpha.RunnerInvite
		decodeEntity(e, &invite)
		if invite.CodeHash == codeHash {
			return &invite, e.Revision(), nil
		}
	}

	return nil, 0, nil
}

func decodeEntity(rpcEntity *entityserver_v1alpha.Entity, target interface{}) {
	type decoder interface {
		Decode(entity.AttrGetter)
	}

	if d, ok := target.(decoder); ok {
		d.Decode(&rpcEntityWrapper{entity: rpcEntity})
	}
}

type rpcEntityWrapper struct {
	entity *entityserver_v1alpha.Entity
}

func (w *rpcEntityWrapper) Get(id entity.Id) (entity.Attr, bool) {
	if id == entity.DBId {
		return entity.Ref(entity.DBId, entity.Id(w.entity.Id())), true
	}

	attrs := w.entity.Attrs()
	for _, attr := range attrs {
		if entity.Id(attr.ID) == id {
			return attr, true
		}
	}
	return entity.Attr{}, false
}

func (w *rpcEntityWrapper) GetAll(name entity.Id) []entity.Attr {
	var result []entity.Attr
	attrs := w.entity.Attrs()
	for _, attr := range attrs {
		if entity.Id(attr.ID) == name {
			result = append(result, attr)
		}
	}
	return result
}

func (w *rpcEntityWrapper) Attrs() []entity.Attr {
	return w.entity.Attrs()
}

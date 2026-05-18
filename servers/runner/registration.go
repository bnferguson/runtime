package runner

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
	"miren.dev/runtime/api/compute/compute_v1alpha"
	"miren.dev/runtime/api/core/core_v1alpha"
	"miren.dev/runtime/api/entityserver/entityserver_v1alpha"
	"miren.dev/runtime/api/runner/runner_v1alpha"
	"miren.dev/runtime/api/storage/storage_v1alpha"
	"miren.dev/runtime/pkg/caauth"
	"miren.dev/runtime/pkg/cond"
	"miren.dev/runtime/pkg/enrolltoken"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/entity/types"
	"miren.dev/runtime/pkg/joincode"
	"miren.dev/runtime/pkg/rpc/standard"
)

const (
	DefaultInviteExpiryHours = 1
	MaxInviteExpiryHours     = 168 // 7 days

	enrollmentCountRetries = 3
)

type RegistrationServerConfig struct {
	Log             *slog.Logger
	Authority       *caauth.Authority
	EAC             *entityserver_v1alpha.EntityAccessClient
	CoordinatorAddr string
	EtcdEndpoints   []string
	EtcdPrefix      string
	NetworkBackend  string

	// Observability endpoints provided to runners at join time
	VictoriametricsAddress string
	VictorialogsAddress    string
}

type RegistrationServer struct {
	RegistrationServerConfig
}

var _ runner_v1alpha.RunnerRegistration = (*RegistrationServer)(nil)

func NewRegistrationServer(cfg RegistrationServerConfig) *RegistrationServer {
	cfg.Log = cfg.Log.With("module", "runner-registration")
	return &RegistrationServer{RegistrationServerConfig: cfg}
}

func (s *RegistrationServer) CreateInvite(ctx context.Context, req *runner_v1alpha.RunnerRegistrationCreateInvite) error {
	args := req.Args()
	results := req.Results()

	reusable := args.HasReusable() && args.Reusable()

	// Determine expiry
	now := time.Now()
	var expiresAt time.Time

	// The generated client always sends ttl_seconds, so we use a negative
	// sentinel (-1) to mean "not specified." The CLI sends -1 when --ttl
	// is omitted, 0 for --ttl 0 (no expiry), and >0 for an explicit TTL.
	ttl := int64(-1)
	if args.HasTtlSeconds() {
		ttl = args.TtlSeconds()
	}

	switch {
	case ttl < -1:
		return cond.ValidationFailure("invalid-ttl", "TTL must be non-negative (use 0 for no expiry)")
	case ttl == 0 && reusable:
		// --ttl 0 on a reusable token means no expiry
		expiresAt = time.Time{}
	case ttl > 0:
		if !reusable && ttl > int64(MaxInviteExpiryHours)*3600 {
			return cond.ValidationFailure("invalid-ttl", fmt.Sprintf("TTL cannot exceed %d hours for one-time tokens", MaxInviteExpiryHours))
		}
		expiresAt = now.Add(time.Duration(ttl) * time.Second)
	default:
		// ttl == -1 (not specified) or ttl == 0 on a non-reusable token:
		// fall through to expires_in_hours
		expiryHours := int32(DefaultInviteExpiryHours)
		if args.HasExpiresInHours() && args.ExpiresInHours() > 0 {
			expiryHours = args.ExpiresInHours()
		}
		if !reusable && expiryHours > int32(MaxInviteExpiryHours) {
			return cond.ValidationFailure("invalid-expiry", fmt.Sprintf("expiry cannot exceed %d hours", MaxInviteExpiryHours))
		}
		expiresAt = now.Add(time.Duration(expiryHours) * time.Hour)
	}

	secret, err := enrolltoken.GenerateSecret()
	if err != nil {
		s.Log.Error("Failed to generate secret", "error", err)
		return cond.Error("failed to generate invite secret")
	}

	codeHash := joincode.Hash(secret)

	invite := &runner_v1alpha.RunnerInvite{
		CodeHash:  codeHash,
		Status:    runner_v1alpha.PENDING,
		CreatedAt: now,
		ExpiresAt: expiresAt,
		Reusable:  reusable,
	}

	if args.HasName() {
		invite.Name = args.Name()
	}

	if args.HasLabels() {
		for _, labelStr := range args.Labels() {
			parts := strings.SplitN(labelStr, "=", 2)
			if len(parts) == 2 {
				invite.Labels = append(invite.Labels, types.Label{Key: parts[0], Value: parts[1]})
			}
		}
	}

	// Build entity with an ident so it gets a stable, unique key
	inviteIdent := "runner_invite/" + codeHash[:16]
	rpcEntity := &entityserver_v1alpha.Entity{}
	rpcEntity.SetAttrs(
		entity.New(
			invite.Encode,
			entity.Ident, types.Keyword(inviteIdent),
		).Attrs())

	putResp, err := s.EAC.Put(ctx, rpcEntity)
	if err != nil {
		s.Log.Error("Failed to create invite entity", "error", err)
		return cond.Error("failed to create invite")
	}

	// Build the token with the coordinator address baked in
	addr := s.CoordinatorAddr
	if args.HasCoordinatorAddr() && args.CoordinatorAddr() != "" {
		addr = args.CoordinatorAddr()
	}
	token := enrolltoken.Encode(addr, secret)

	s.Log.Info("Created runner invite",
		"invite_id", putResp.Id(),
		"reusable", reusable,
		"name", invite.Name,
		"expires_at", expiresAt.Format(time.RFC3339),
		"label_count", len(invite.Labels))

	results.SetCode(token)
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
	if !enrolltoken.IsHexSecret(code) {
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

	if !invite.ExpiresAt.IsZero() && time.Now().After(invite.ExpiresAt) {
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

	if !invite.Reusable {
		// One-time invite: claim it (PENDING->CLAIMED) with CAS to prevent
		// concurrent joins from minting multiple certificates
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
	}

	// Now that invite is claimed, issue the certificate with proper SANs
	// so the coordinator can connect to the runner's API by IP.
	runnerIDPrefix := runnerID
	if len(runnerIDPrefix) > 8 {
		runnerIDPrefix = runnerIDPrefix[:8]
	}
	certName := fmt.Sprintf("runner-%s", runnerIDPrefix)

	ips := []net.IP{
		net.ParseIP("127.0.0.1"),
		net.ParseIP("::1"),
	}
	dnsNames := []string{"localhost"}

	if listenAddr != "" {
		host, _, err := net.SplitHostPort(listenAddr)
		if err == nil && host != "" {
			if ip := net.ParseIP(host); ip != nil {
				ips = append(ips, ip)
			} else if host != "localhost" {
				dnsNames = append(dnsNames, host)
			}
		}
	}

	cc, err := s.Authority.IssueCertificate(caauth.Options{
		CommonName:   certName,
		Organization: "miren",
		ValidFor:     365 * 24 * time.Hour,
		IPs:          ips,
		DNSNames:     dnsNames,
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

	name := ""
	if args.HasName() {
		name = args.Name()
	}

	node := &compute_v1alpha.Node{
		RunnerId:     runnerID,
		Name:         name,
		ApiAddress:   listenAddr,
		Version:      version,
		RegisteredAt: time.Now(),
		Constraints:  labels,
	}

	// Create node entity with an ident so setupEntity can find it via CreateOrUpdate
	nodeEntity := &entityserver_v1alpha.Entity{}
	nodeEntity.SetAttrs(
		entity.New(
			(&core_v1alpha.Metadata{Name: runnerID}).Encode,
			node.Encode,
			entity.Ident, types.Keyword(node.ShortKind()+"/"+runnerID),
		).Attrs())

	nodePutResp, err := s.EAC.Put(ctx, nodeEntity)
	if err != nil {
		s.Log.Error("Failed to create node entity", "error", err, "runner_id", runnerID)
		results.SetError("failed to register runner")
		return nil
	}

	// Increment enrollment count after everything succeeded, so the count
	// only reflects runners that actually completed the join.
	if invite.Reusable {
		if err := s.incrementEnrollmentCount(ctx, invite, inviteRevision); err != nil {
			s.Log.Warn("Failed to increment enrollment count (runner joined successfully)",
				"error", err, "invite_id", invite.ID, "runner_id", runnerID)
		}
	}

	s.Log.Info("Runner joined successfully",
		"runner_id", runnerID,
		"name", name,
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
	if s.VictoriametricsAddress != "" {
		results.SetVictoriametricsAddress(s.VictoriametricsAddress)
	}
	if s.VictorialogsAddress != "" {
		results.SetVictorialogsAddress(s.VictorialogsAddress)
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

		if invite.Status == runner_v1alpha.PENDING && !invite.ExpiresAt.IsZero() && now.After(invite.ExpiresAt) {
			continue
		}

		info := &runner_v1alpha.InviteInfo{}
		info.SetId(e.Id())
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

		if invite.Name != "" {
			info.SetName(invite.Name)
		}
		info.SetReusable(invite.Reusable)
		if invite.EnrollmentCount > 0 {
			info.SetEnrollmentCount(invite.EnrollmentCount)
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
		name := node.Name
		if name == "" {
			name = string(node.ID)
		}
		info.SetName(name)

		if sid := e.Entity().ShortId(); sid != "" {
			info.SetShortId(sid)
		}
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

func (s *RegistrationServer) RemoveRunner(ctx context.Context, req *runner_v1alpha.RunnerRegistrationRemoveRunner) error {
	args := req.Args()
	results := req.Results()

	if !args.HasQuery() || args.Query() == "" {
		results.SetError("runner name or ID is required")
		return nil
	}

	query := args.Query()
	force := args.HasForce() && args.Force()

	// Find the node entity matching the query
	node, nodeID, err := s.findNodeByQuery(ctx, query)
	if err != nil {
		s.Log.Error("Failed to find runner", "query", query, "error", err)
		results.SetError(err.Error())
		return nil
	}
	if node == nil {
		results.SetError(fmt.Sprintf("runner %q not found", query))
		return nil
	}

	// Check for active schedules (sandboxes assigned to this node).
	// Skip the check entirely when --force is set so that a query error
	// (e.g. missing index) can't block a forced removal.
	if !force {
		scheduleCount, err := s.countNodeSchedules(ctx, nodeID)
		if err != nil {
			s.Log.Error("Failed to check schedules", "node_id", nodeID, "error", err)
			results.SetError("failed to check for active sandboxes")
			return nil
		}

		if scheduleCount > 0 {
			results.SetError(fmt.Sprintf("runner has %d active sandbox schedule(s); use --force to remove anyway", scheduleCount))
			return nil
		}
	}

	// Clean up associated resources
	removedResources := int32(0)

	// Delete schedules for this node (only needed on --force; the non-force
	// path already rejected if any schedules existed).
	if force {
		deleted, err := s.deleteNodeSchedules(ctx, nodeID)
		if err != nil {
			s.Log.Warn("Failed to delete schedules (continuing with --force)", "node_id", nodeID, "error", err)
		} else {
			removedResources += int32(deleted)
		}
	}

	// Delete disk mounts, volumes, and leases for this node
	for _, ref := range []entity.Attr{
		entity.Ref(storage_v1alpha.DiskMountNodeIdId, nodeID),
		entity.Ref(storage_v1alpha.DiskVolumeNodeIdId, nodeID),
		entity.Ref(storage_v1alpha.DiskLeaseNodeIdId, nodeID),
	} {
		deleted, err := s.deleteEntitiesByIndex(ctx, ref)
		if err != nil {
			s.Log.Warn("Failed to clean up some resources", "index", ref.ID, "error", err)
		}
		removedResources += int32(deleted)
	}

	// Delete the node entity
	_, err = s.EAC.Delete(ctx, string(nodeID))
	if err != nil {
		s.Log.Error("Failed to delete node entity", "node_id", nodeID, "error", err)
		results.SetError("failed to delete runner")
		return nil
	}

	name := node.Name
	if name == "" {
		name = string(nodeID)
	}

	s.Log.Info("Removed runner",
		"name", name,
		"runner_id", node.RunnerId,
		"node_id", nodeID,
		"removed_resources", removedResources)

	results.SetName(name)
	results.SetRunnerId(node.RunnerId)
	results.SetRemovedResources(removedResources)
	return nil
}

// findNodeByQuery looks up a node entity by name, runner ID, entity ID, or short ID prefix.
// Exact matches (name, runner ID, entity ID) are returned immediately. Prefix
// matches are collected and only returned when unambiguous.
func (s *RegistrationServer) findNodeByQuery(ctx context.Context, query string) (*compute_v1alpha.Node, entity.Id, error) {
	listResp, err := s.EAC.List(ctx, entity.Ref(entity.EntityKind, compute_v1alpha.KindNode))
	if err != nil {
		return nil, "", err
	}

	query = strings.TrimSpace(query)

	type candidate struct {
		node compute_v1alpha.Node
		id   entity.Id
	}
	var prefixMatches []candidate

	for _, e := range listResp.Values() {
		var node compute_v1alpha.Node
		decodeEntity(e, &node)

		if node.RunnerId == "" {
			continue
		}

		id := entity.Id(e.Id())

		// Exact match by entity ID, runner ID, name, or short ID
		if string(id) == query ||
			node.RunnerId == query ||
			(node.Name != "" && node.Name == query) ||
			e.Entity().ShortId() == query {
			return &node, id, nil
		}

		// Prefix match by entity ID
		if strings.HasPrefix(string(id), query) {
			prefixMatches = append(prefixMatches, candidate{node, id})
		}
	}

	switch len(prefixMatches) {
	case 0:
		return nil, "", nil
	case 1:
		return &prefixMatches[0].node, prefixMatches[0].id, nil
	default:
		return nil, "", fmt.Errorf("ambiguous query %q matches %d runners", query, len(prefixMatches))
	}
}

func (s *RegistrationServer) countNodeSchedules(ctx context.Context, nodeID entity.Id) (int, error) {
	listResp, err := s.EAC.List(ctx, compute_v1alpha.Index(compute_v1alpha.KindSandbox, nodeID))
	if err != nil {
		return 0, err
	}
	return len(listResp.Values()), nil
}

func (s *RegistrationServer) deleteNodeSchedules(ctx context.Context, nodeID entity.Id) (int, error) {
	listResp, err := s.EAC.List(ctx, compute_v1alpha.Index(compute_v1alpha.KindSandbox, nodeID))
	if err != nil {
		return 0, err
	}

	deleted := 0
	for _, e := range listResp.Values() {
		if _, err := s.EAC.Delete(ctx, e.Id()); err != nil {
			s.Log.Warn("Failed to delete schedule", "id", e.Id(), "error", err)
			continue
		}
		deleted++
	}
	return deleted, nil
}

func (s *RegistrationServer) deleteEntitiesByIndex(ctx context.Context, ref entity.Attr) (int, error) {
	listResp, err := s.EAC.List(ctx, ref)
	if err != nil {
		return 0, err
	}

	deleted := 0
	for _, e := range listResp.Values() {
		if _, err := s.EAC.Delete(ctx, e.Id()); err != nil {
			s.Log.Warn("Failed to delete entity", "id", e.Id(), "error", err)
			continue
		}
		deleted++
	}
	return deleted, nil
}

// incrementEnrollmentCount atomically increments the enrollment count on a
// reusable invite. It retries on CAS contention.
func (s *RegistrationServer) incrementEnrollmentCount(ctx context.Context, invite *runner_v1alpha.RunnerInvite, revision int64) error {
	for attempt := 0; attempt < enrollmentCountRetries; attempt++ {
		if attempt > 0 {
			// Re-read the invite to get the latest revision and count
			refreshed, rev, err := s.findInviteByHash(ctx, invite.CodeHash)
			if err != nil {
				return fmt.Errorf("re-reading invite: %w", err)
			}
			if refreshed == nil {
				return fmt.Errorf("invite no longer exists")
			}
			invite = refreshed
			revision = rev
		}

		// Re-check state in case the invite was revoked or expired
		// between the initial check and this attempt
		if invite.Status != runner_v1alpha.PENDING {
			return fmt.Errorf("invite is no longer pending")
		}
		if !invite.ExpiresAt.IsZero() && time.Now().After(invite.ExpiresAt) {
			return fmt.Errorf("invite has expired")
		}

		invite.EnrollmentCount++

		updateAttrs := invite.Encode()
		updateEntity := &entityserver_v1alpha.Entity{}
		updateEntity.SetId(string(invite.ID))
		updateEntity.SetAttrs(updateAttrs)
		updateEntity.SetRevision(revision)

		_, err := s.EAC.Put(ctx, updateEntity)
		if err == nil {
			return nil
		}

		s.Log.Warn("CAS contention incrementing enrollment count, retrying",
			"attempt", attempt+1,
			"invite_id", invite.ID,
			"error", err)
	}
	return fmt.Errorf("failed to increment enrollment count after %d retries", enrollmentCountRetries)
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

package sandbox

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"time"

	"github.com/containerd/containerd/namespaces"
	containerd "github.com/containerd/containerd/v2/client"

	compute "miren.dev/runtime/api/compute/compute_v1alpha"
	"miren.dev/runtime/network"
	"miren.dev/runtime/pkg/controller"
	"miren.dev/runtime/pkg/entity"
	"miren.dev/runtime/pkg/saga"
)

// Saga action names
const (
	sagaCreateSandbox  = "create-sandbox"
	actionAllocNetwork = "alloc-network"
	actionPatchNetwork = "patch-network"
	actionCreateCtr    = "create-container"
	actionBootTask     = "boot-task"
	actionBootCtrs     = "boot-containers"
	actionAddMetrics   = "add-metrics"
	actionWaitPorts    = "wait-ports"
	actionSetRunning   = "set-running"
	actionUpdateSvcs   = "update-services"
)

// createSandboxDeps holds the dependencies injected into the saga context.
// These are retrieved via saga.Get[*createSandboxDeps](ctx) within actions.
type createSandboxDeps struct {
	ctrl         *SandboxController
	writeTracker controller.WriteTracker
}

// --- Action input/output types ---

type allocNetworkIn struct {
	SandboxID string `json:"sandbox_id" saga:"sandbox_id"`
}

type allocNetworkOut struct {
	Addresses []string `json:"addresses" saga:"addresses"`
}

func allocNetwork(ctx context.Context, in allocNetworkIn) (allocNetworkOut, error) {
	deps := saga.Get[*createSandboxDeps](ctx)
	log := saga.Get[*slog.Logger](ctx)

	// Fetch sandbox entity to pass to allocateNetwork
	resp, err := deps.ctrl.EAC.Get(ctx, in.SandboxID)
	if err != nil {
		return allocNetworkOut{}, fmt.Errorf("fetching sandbox: %w", err)
	}
	var co compute.Sandbox
	co.Decode(resp.Entity().Entity())

	ep, err := deps.ctrl.allocateNetwork(ctx, &co)
	if err != nil {
		return allocNetworkOut{}, fmt.Errorf("allocating network: %w", err)
	}

	addrs := make([]string, len(ep.Addresses))
	for i, a := range ep.Addresses {
		addrs[i] = a.String()
	}

	log.Debug("saga: allocated network", "sandbox", in.SandboxID, "addresses", addrs)
	return allocNetworkOut{Addresses: addrs}, nil
}

func undoAllocNetwork(ctx context.Context, in allocNetworkIn, out allocNetworkOut) error {
	deps := saga.Get[*createSandboxDeps](ctx)
	log := saga.Get[*slog.Logger](ctx)

	for _, addrStr := range out.Addresses {
		prefix, err := netip.ParsePrefix(addrStr)
		if err != nil {
			log.Error("saga undo: failed to parse address", "addr", addrStr, "error", err)
			continue
		}
		if err := deps.ctrl.Subnet.ReleaseAddr(prefix.Addr()); err != nil {
			log.Error("saga undo: failed to release IP", "addr", prefix.Addr(), "error", err)
		} else {
			log.Debug("saga undo: released IP", "addr", prefix.Addr())
		}
	}
	return nil
}

// --- Patch network entity ---

type patchNetworkIn struct {
	SandboxID string   `json:"sandbox_id" saga:"sandbox_id"`
	Addresses []string `json:"addresses" saga:"addresses"`
}

type patchNetworkOut struct {
	Revision int64 `json:"revision" saga:"revision"`
}

func patchNetwork(ctx context.Context, in patchNetworkIn) (patchNetworkOut, error) {
	deps := saga.Get[*createSandboxDeps](ctx)

	// Build network components from addresses
	var networkAttrs []any
	networkAttrs = append(networkAttrs, entity.Ref(entity.DBId, entity.Id(in.SandboxID)))

	for _, addrStr := range in.Addresses {
		net := compute.Network{
			Address: addrStr,
			Subnet:  deps.ctrl.Bridge,
		}
		networkAttrs = append(networkAttrs, entity.Component(compute.SandboxNetworkId, net.Encode()))
	}

	patchEnt := entity.New(networkAttrs...)
	res, err := deps.ctrl.EAC.Patch(ctx, patchEnt.Attrs(), 0)
	if err != nil {
		return patchNetworkOut{}, fmt.Errorf("patching sandbox with network: %w", err)
	}

	if deps.writeTracker != nil && res.HasRevision() {
		deps.writeTracker.RecordWrite(res.Revision())
	}

	return patchNetworkOut{Revision: res.Revision()}, nil
}

func undoPatchNetwork(_ context.Context, _ patchNetworkIn, _ patchNetworkOut) error {
	// Entity patches don't need explicit undo; the sandbox will be marked DEAD
	return nil
}

// --- Create container (includes build spec) ---

type createContainerIn struct {
	SandboxID string   `json:"sandbox_id" saga:"sandbox_id"`
	Addresses []string `json:"addresses" saga:"addresses"`
	Revision  int64    `json:"revision" saga:"revision"`
}

type createContainerOut struct {
	ContainerID string `json:"container_id" saga:"container_id"`
}

func createContainer(ctx context.Context, in createContainerIn) (createContainerOut, error) {
	deps := saga.Get[*createSandboxDeps](ctx)
	log := saga.Get[*slog.Logger](ctx)

	ctx = namespaces.WithNamespace(ctx, deps.ctrl.Namespace)

	// Fetch current sandbox state
	resp, err := deps.ctrl.EAC.Get(ctx, in.SandboxID)
	if err != nil {
		return createContainerOut{}, fmt.Errorf("fetching sandbox: %w", err)
	}
	var co compute.Sandbox
	co.Decode(resp.Entity().Entity())

	// Rebuild endpoint config from stored addresses
	ep, err := rebuildEndpointConfig(deps.ctrl, in.Addresses)
	if err != nil {
		return createContainerOut{}, fmt.Errorf("rebuilding endpoint config: %w", err)
	}

	// Build entity meta for buildSpec
	meta := &entity.Meta{
		Entity:   resp.Entity().Entity(),
		Revision: in.Revision,
	}

	// Build OCI spec
	opts, err := deps.ctrl.buildSpec(ctx, &co, ep, meta)
	if err != nil {
		return createContainerOut{}, fmt.Errorf("building spec: %w", err)
	}

	// Create pause container
	cid := pauseContainerId(co.ID)
	_, err = deps.ctrl.CC.NewContainer(ctx, cid, opts...)
	if err != nil {
		return createContainerOut{}, fmt.Errorf("creating container %s: %w", co.ID, err)
	}

	log.Debug("saga: created container", "sandbox", in.SandboxID, "container", cid)
	return createContainerOut{ContainerID: cid}, nil
}

func undoCreateContainer(ctx context.Context, _ createContainerIn, out createContainerOut) error {
	deps := saga.Get[*createSandboxDeps](ctx)
	log := saga.Get[*slog.Logger](ctx)

	if out.ContainerID == "" {
		return nil
	}

	ctx = namespaces.WithNamespace(ctx, deps.ctrl.Namespace)
	container, err := deps.ctrl.CC.LoadContainer(ctx, out.ContainerID)
	if err != nil {
		log.Debug("saga undo: container not found, already cleaned up", "id", out.ContainerID)
		return nil
	}
	deps.ctrl.cleanupContainer(ctx, container)
	log.Debug("saga undo: deleted container", "id", out.ContainerID)
	return nil
}

// --- Boot task ---

type bootTaskIn struct {
	SandboxID   string   `json:"sandbox_id" saga:"sandbox_id"`
	ContainerID string   `json:"container_id" saga:"container_id"`
	Addresses   []string `json:"addresses" saga:"addresses"`
}

type bootTaskOut struct {
	TaskPID int    `json:"task_pid" saga:"task_pid"`
	Cgroups string `json:"cgroups" saga:"cgroups"`
}

func bootTask(ctx context.Context, in bootTaskIn) (bootTaskOut, error) {
	deps := saga.Get[*createSandboxDeps](ctx)
	log := saga.Get[*slog.Logger](ctx)

	ctx = namespaces.WithNamespace(ctx, deps.ctrl.Namespace)

	// Fetch sandbox
	resp, err := deps.ctrl.EAC.Get(ctx, in.SandboxID)
	if err != nil {
		return bootTaskOut{}, fmt.Errorf("fetching sandbox: %w", err)
	}
	var co compute.Sandbox
	co.Decode(resp.Entity().Entity())

	// Rebuild endpoint config
	ep, err := rebuildEndpointConfig(deps.ctrl, in.Addresses)
	if err != nil {
		return bootTaskOut{}, fmt.Errorf("rebuilding endpoint config: %w", err)
	}

	// Load container
	container, err := deps.ctrl.CC.LoadContainer(ctx, in.ContainerID)
	if err != nil {
		return bootTaskOut{}, fmt.Errorf("loading container: %w", err)
	}

	// Boot initial task
	task, err := deps.ctrl.bootInitialTask(ctx, &co, ep, container)
	if err != nil {
		return bootTaskOut{}, fmt.Errorf("booting task: %w", err)
	}

	// Get cgroups path from container spec
	rootSpec, err := container.Spec(ctx)
	if err != nil {
		return bootTaskOut{}, fmt.Errorf("getting container spec: %w", err)
	}

	cgroups := rootSpec.Linux.CgroupsPath
	log.Debug("saga: booted task", "sandbox", in.SandboxID, "pid", task.Pid(), "cgroups", cgroups)
	return bootTaskOut{TaskPID: int(task.Pid()), Cgroups: cgroups}, nil
}

func undoBootTask(ctx context.Context, in bootTaskIn, _ bootTaskOut) error {
	deps := saga.Get[*createSandboxDeps](ctx)
	log := saga.Get[*slog.Logger](ctx)

	ctx = namespaces.WithNamespace(ctx, deps.ctrl.Namespace)

	// Unconfigure firewall for this sandbox
	resp, err := deps.ctrl.EAC.Get(ctx, in.SandboxID)
	if err == nil {
		var co compute.Sandbox
		co.Decode(resp.Entity().Entity())
		deps.ctrl.unconfigureFirewall(&co)
	}

	// Kill task via container
	container, err := deps.ctrl.CC.LoadContainer(ctx, in.ContainerID)
	if err != nil {
		log.Debug("saga undo: container not found for task kill", "id", in.ContainerID)
		return nil
	}

	task, err := container.Task(ctx, nil)
	if err != nil {
		log.Debug("saga undo: task not found", "id", in.ContainerID)
		return nil
	}

	_, err = task.Delete(ctx, containerd.WithProcessKill)
	if err != nil {
		log.Error("saga undo: failed to kill task", "id", in.ContainerID, "error", err)
	}
	log.Debug("saga undo: killed task", "id", in.ContainerID)
	return nil
}

// --- Boot containers ---

type bootContainersIn struct {
	SandboxID   string   `json:"sandbox_id" saga:"sandbox_id"`
	ContainerID string   `json:"container_id" saga:"container_id"`
	Addresses   []string `json:"addresses" saga:"addresses"`
	TaskPID     int      `json:"task_pid" saga:"task_pid"`
	Cgroups     string   `json:"cgroups" saga:"cgroups"`
	Revision    int64    `json:"revision" saga:"revision"`
}

type bootContainersOut struct {
	WaitPortIDs   []string `json:"wait_port_ids" saga:"wait_port_ids"`
	WaitPortPorts []int    `json:"wait_port_ports" saga:"wait_port_ports"`
}

func bootContainers(ctx context.Context, in bootContainersIn) (bootContainersOut, error) {
	deps := saga.Get[*createSandboxDeps](ctx)
	log := saga.Get[*slog.Logger](ctx)

	ctx = namespaces.WithNamespace(ctx, deps.ctrl.Namespace)

	// Fetch sandbox
	resp, err := deps.ctrl.EAC.Get(ctx, in.SandboxID)
	if err != nil {
		return bootContainersOut{}, fmt.Errorf("fetching sandbox: %w", err)
	}
	var co compute.Sandbox
	co.Decode(resp.Entity().Entity())

	// Rebuild endpoint config
	ep, err := rebuildEndpointConfig(deps.ctrl, in.Addresses)
	if err != nil {
		return bootContainersOut{}, fmt.Errorf("rebuilding endpoint config: %w", err)
	}

	// Build cgroups map
	cgroups := map[string]string{"": in.Cgroups}

	// Build meta
	meta := &entity.Meta{
		Entity:   resp.Entity().Entity(),
		Revision: in.Revision,
	}

	// Configure volumes
	volumeMounts, err := deps.ctrl.configureVolumes(ctx, &co, meta)
	if err != nil {
		return bootContainersOut{}, fmt.Errorf("configuring volumes: %w", err)
	}

	// Boot app containers
	waitPorts, err := deps.ctrl.bootContainers(ctx, &co, ep, in.TaskPID, cgroups, meta, volumeMounts)
	if err != nil {
		return bootContainersOut{}, fmt.Errorf("booting containers: %w", err)
	}

	var wpIDs []string
	var wpPorts []int
	for _, wp := range waitPorts {
		wpIDs = append(wpIDs, wp.id)
		wpPorts = append(wpPorts, wp.port)
	}

	log.Debug("saga: booted containers", "sandbox", in.SandboxID, "ports", len(waitPorts))
	return bootContainersOut{WaitPortIDs: wpIDs, WaitPortPorts: wpPorts}, nil
}

func undoBootContainers(ctx context.Context, in bootContainersIn, _ bootContainersOut) error {
	deps := saga.Get[*createSandboxDeps](ctx)
	log := saga.Get[*slog.Logger](ctx)

	ctx = namespaces.WithNamespace(ctx, deps.ctrl.Namespace)
	deps.ctrl.destroySubContainers(ctx, entity.Id(in.SandboxID))
	log.Debug("saga undo: destroyed subcontainers", "sandbox", in.SandboxID)

	// Release disk leases
	if err := deps.ctrl.releaseDiskLeases(ctx, entity.Id(in.SandboxID)); err != nil {
		log.Error("saga undo: failed to release disk leases", "sandbox", in.SandboxID, "error", err)
	}
	return nil
}

// --- Add metrics ---

type addMetricsIn struct {
	SandboxID string `json:"sandbox_id" saga:"sandbox_id"`
	Cgroups   string `json:"cgroups" saga:"cgroups"`
}

type addMetricsOut struct {
	LogEntity string `json:"log_entity" saga:"log_entity"`
}

func addMetrics(ctx context.Context, in addMetricsIn) (addMetricsOut, error) {
	deps := saga.Get[*createSandboxDeps](ctx)

	// Fetch sandbox for spec info
	resp, err := deps.ctrl.EAC.Get(ctx, in.SandboxID)
	if err != nil {
		return addMetricsOut{}, fmt.Errorf("fetching sandbox: %w", err)
	}
	var co compute.Sandbox
	co.Decode(resp.Entity().Entity())

	le := co.Spec.LogEntity
	if le == "" {
		le = co.ID.String()
	}

	attrs := map[string]string{"sandbox": co.ID.String()}
	if co.Spec.Version != "" {
		attrs["version"] = co.Spec.Version.String()
	}
	for _, lbl := range co.Spec.LogAttribute {
		attrs[lbl.Key] = lbl.Value
	}

	cgroups := map[string]string{"": in.Cgroups}
	if err := deps.ctrl.Metrics.Add(le, cgroups, attrs); err != nil {
		return addMetricsOut{}, fmt.Errorf("adding metrics: %w", err)
	}

	return addMetricsOut{LogEntity: le}, nil
}

func undoAddMetrics(ctx context.Context, _ addMetricsIn, out addMetricsOut) error {
	deps := saga.Get[*createSandboxDeps](ctx)
	if out.LogEntity != "" {
		deps.ctrl.Metrics.Remove(out.LogEntity)
	}
	return nil
}

// --- Wait ports ---

type waitPortsIn struct {
	SandboxID     string   `json:"sandbox_id" saga:"sandbox_id"`
	WaitPortIDs   []string `json:"wait_port_ids" saga:"wait_port_ids"`
	WaitPortPorts []int    `json:"wait_port_ports" saga:"wait_port_ports"`
}

type waitPortsOut struct{}

func waitPorts(ctx context.Context, in waitPortsIn) (waitPortsOut, error) {
	deps := saga.Get[*createSandboxDeps](ctx)
	log := saga.Get[*slog.Logger](ctx)

	portTimeout := portWaitTimeout
	for i := range in.WaitPortIDs {
		if i >= len(in.WaitPortPorts) {
			break
		}
		log.Info("saga: waiting for port", "sandbox", in.SandboxID, "port", in.WaitPortPorts[i])
		if err := deps.ctrl.waitForPort(ctx, in.WaitPortIDs[i], in.WaitPortPorts[i], portTimeout); err != nil {
			return waitPortsOut{}, fmt.Errorf("port %d not reachable: %w", in.WaitPortPorts[i], err)
		}
	}
	return waitPortsOut{}, nil
}

func undoWaitPorts(_ context.Context, _ waitPortsIn, _ waitPortsOut) error {
	return nil
}

// --- Set running ---

type setRunningIn struct {
	SandboxID string `json:"sandbox_id" saga:"sandbox_id"`
}

type setRunningOut struct{}

func setRunning(ctx context.Context, in setRunningIn) (setRunningOut, error) {
	deps := saga.Get[*createSandboxDeps](ctx)
	log := saga.Get[*slog.Logger](ctx)

	// Fetch current status to avoid race condition
	resp, err := deps.ctrl.EAC.Get(ctx, in.SandboxID)
	if err != nil {
		log.Warn("saga: failed to fetch sandbox status", "id", in.SandboxID, "error", err)
	} else {
		var currentSandbox compute.Sandbox
		currentSandbox.Decode(resp.Entity().Entity())
		if currentSandbox.Status == compute.DEAD || currentSandbox.Status == compute.STOPPED {
			log.Info("saga: sandbox already in terminal state",
				"id", in.SandboxID, "status", currentSandbox.Status)
			return setRunningOut{}, nil
		}
	}

	// Update status to RUNNING
	result, err := deps.ctrl.EAC.Patch(ctx, entity.New(
		entity.Ref(entity.DBId, entity.Id(in.SandboxID)),
		(&compute.Sandbox{Status: compute.RUNNING}).Encode,
	).Attrs(), 0)
	if err != nil {
		return setRunningOut{}, fmt.Errorf("setting status to RUNNING: %w", err)
	}

	if deps.writeTracker != nil && result.HasRevision() {
		deps.writeTracker.RecordWrite(result.Revision())
	}

	log.Info("saga: sandbox set to RUNNING", "id", in.SandboxID)
	return setRunningOut{}, nil
}

func undoSetRunning(ctx context.Context, in setRunningIn, _ setRunningOut) error {
	deps := saga.Get[*createSandboxDeps](ctx)
	log := saga.Get[*slog.Logger](ctx)

	result, err := deps.ctrl.EAC.Patch(ctx, entity.New(
		entity.Ref(entity.DBId, entity.Id(in.SandboxID)),
		(&compute.Sandbox{Status: compute.DEAD}).Encode,
	).Attrs(), 0)
	if err != nil {
		log.Error("saga undo: failed to set status to DEAD", "id", in.SandboxID, "error", err)
		return nil // best-effort
	}

	if deps.writeTracker != nil && result.HasRevision() {
		deps.writeTracker.RecordWrite(result.Revision())
	}
	return nil
}

// --- Update services ---

type updateServicesIn struct {
	SandboxID string   `json:"sandbox_id" saga:"sandbox_id"`
	Addresses []string `json:"addresses" saga:"addresses"`
	Revision  int64    `json:"revision" saga:"revision"`
}

type updateServicesOut struct{}

func updateServices(ctx context.Context, in updateServicesIn) (updateServicesOut, error) {
	deps := saga.Get[*createSandboxDeps](ctx)

	// Fetch sandbox
	resp, err := deps.ctrl.EAC.Get(ctx, in.SandboxID)
	if err != nil {
		return updateServicesOut{}, fmt.Errorf("fetching sandbox: %w", err)
	}
	var co compute.Sandbox
	co.Decode(resp.Entity().Entity())

	// Rebuild endpoint config
	ep, err := rebuildEndpointConfig(deps.ctrl, in.Addresses)
	if err != nil {
		return updateServicesOut{}, fmt.Errorf("rebuilding endpoint config: %w", err)
	}

	meta := &entity.Meta{
		Entity:   resp.Entity().Entity(),
		Revision: in.Revision,
	}

	if err := deps.ctrl.updateServices(ctx, &co, meta, ep); err != nil {
		return updateServicesOut{}, fmt.Errorf("updating services: %w", err)
	}

	return updateServicesOut{}, nil
}

func undoUpdateServices(_ context.Context, _ updateServicesIn, _ updateServicesOut) error {
	// Services reconcile themselves; no explicit undo needed
	return nil
}

// --- Helpers ---

// rebuildEndpointConfig reconstructs a network.EndpointConfig from stored address strings.
func rebuildEndpointConfig(ctrl *SandboxController, addresses []string) (*network.EndpointConfig, error) {
	ep := &network.EndpointConfig{
		Bridge: &network.BridgeConfig{
			Name:      ctrl.Bridge,
			Addresses: []netip.Prefix{ctrl.Subnet.Router()},
		},
	}

	for _, addrStr := range addresses {
		prefix, err := netip.ParsePrefix(addrStr)
		if err != nil {
			return nil, fmt.Errorf("parsing address %q: %w", addrStr, err)
		}
		ep.Addresses = append(ep.Addresses, prefix)
	}

	if err := ep.DeriveDefaultGateway(); err != nil {
		return nil, fmt.Errorf("deriving default gateway: %w", err)
	}

	return ep, nil
}

// portWaitTimeout is the timeout for waiting for ports to bind.
var portWaitTimeout = 15 * time.Second

// registerCreateSandboxSaga registers the saga definition for sandbox creation.
func registerCreateSandboxSaga(
	registry *saga.Registry,
	ctrl *SandboxController,
	writeTracker controller.WriteTracker,
	log *slog.Logger,
) error {
	deps := &createSandboxDeps{
		ctrl:         ctrl,
		writeTracker: writeTracker,
	}

	// Build and register the saga definition
	return saga.Define(sagaCreateSandbox).
		Using(deps).
		Using(log).
		Action(actionAllocNetwork, allocNetwork).Undo(undoAllocNetwork).
		Action(actionPatchNetwork, patchNetwork).Undo(undoPatchNetwork).
		Action(actionCreateCtr, createContainer).Undo(undoCreateContainer).
		Action(actionBootTask, bootTask).Undo(undoBootTask).
		Action(actionBootCtrs, bootContainers).Undo(undoBootContainers).
		Action(actionAddMetrics, addMetrics).Undo(undoAddMetrics).
		Action(actionWaitPorts, waitPorts).Undo(undoWaitPorts).
		Action(actionSetRunning, setRunning).Undo(undoSetRunning).
		Action(actionUpdateSvcs, updateServices).Undo(undoUpdateServices).
		RegisterTo(registry)
}

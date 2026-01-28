// Example: Sandbox Creation
//
// This example demonstrates saga patterns for the sandbox creation flow from
// controllers/sandbox/sandbox.go. It models the full 8-step process of bringing
// up an isolated execution environment, showing how sagas handle complex
// orchestration with multiple collaborators (containerd, networking, metrics).
//
// Key concepts demonstrated:
//
//   - Multi-collaborator orchestration: Network allocation, container runtime,
//     metrics collection, and entity storage all coordinated in sequence
//
//   - Handle-based resources: Containerd returns handles (Container, Task) that
//     aren't directly serializable. We store IDs in saga outputs and look up
//     handles via the collaborator on recovery.
//
//   - Implicit vs explicit undo: Some actions (BuildSpec) have no side effects
//     and need no undo. Others (CreateContainer) have explicit cleanup.
//
//   - Graceful degradation: BootContainers attempts SIGTERM before SIGKILL,
//     showing that undos can have their own error handling.
//
// The saga models these steps from controllers/sandbox/sandbox.go:
//
//	AllocateNetwork  → Reserve IP from subnet
//	BuildSpec        → Construct OCI container specification
//	ConfigureVolumes → Prepare volume mounts, acquire disk leases
//	CreateContainer  → Create pause container in containerd
//	BootTask         → Start pause container, configure network namespace
//	BootContainers   → Start application containers joined to pause
//	AddMetrics       → Register with metrics collection system
//	UpdateStatus     → Mark sandbox RUNNING in entity store
//
// Run the tests:
//
//	go test -v ./pkg/saga/... -run TestSandboxSaga
//
// See example_sandwich_test.go for a simpler introduction to saga mechanics,
// and example_build_test.go for entity system integration patterns.
package saga_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"miren.dev/runtime/pkg/saga"
)

// =============================================================================
// Stubbed Collaborators
// =============================================================================

// Subnet manages IP address allocation for sandbox networking.
type Subnet struct {
	mu        sync.Mutex
	available []string
	allocated map[string]string // sandboxID -> IP
	events    []string
}

func NewSubnet(ips ...string) *Subnet {
	return &Subnet{
		available: ips,
		allocated: make(map[string]string),
	}
}

func (s *Subnet) Allocate(sandboxID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.available) == 0 {
		s.events = append(s.events, fmt.Sprintf("Allocation failed for %s: no IPs available", sandboxID))
		return "", errors.New("no IP addresses available")
	}

	ip := s.available[0]
	s.available = s.available[1:]
	s.allocated[sandboxID] = ip
	s.events = append(s.events, fmt.Sprintf("Allocated %s to %s", ip, sandboxID))
	return ip, nil
}

func (s *Subnet) Release(sandboxID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ip, ok := s.allocated[sandboxID]
	if !ok {
		return nil // Already released, idempotent
	}

	delete(s.allocated, sandboxID)
	s.available = append(s.available, ip)
	s.events = append(s.events, fmt.Sprintf("Released %s from %s", ip, sandboxID))
	return nil
}

// Containerd is a stubbed container runtime.
type Containerd struct {
	mu         sync.Mutex
	containers map[string]*FakeContainer
	tasks      map[string]*FakeTask
	events     []string

	// Failure injection
	CreateContainerErr error
	BootTaskErr        error
	BootContainersErr  error
}

func NewContainerd() *Containerd {
	return &Containerd{
		containers: make(map[string]*FakeContainer),
		tasks:      make(map[string]*FakeTask),
	}
}

type FakeContainer struct {
	ID      string
	Spec    string
	Deleted bool
}

type FakeTask struct {
	ContainerID string
	PID         int
	Running     bool
}

func (c *Containerd) CreateContainer(id, spec string) (*FakeContainer, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.CreateContainerErr != nil {
		c.events = append(c.events, fmt.Sprintf("CreateContainer failed for %s: %v", id, c.CreateContainerErr))
		return nil, c.CreateContainerErr
	}

	container := &FakeContainer{ID: id, Spec: spec}
	c.containers[id] = container
	c.events = append(c.events, fmt.Sprintf("Created container %s", id))
	return container, nil
}

func (c *Containerd) DeleteContainer(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	container, ok := c.containers[id]
	if !ok {
		return nil // Already deleted, idempotent
	}

	container.Deleted = true
	delete(c.containers, id)
	c.events = append(c.events, fmt.Sprintf("Deleted container %s", id))
	return nil
}

func (c *Containerd) StartTask(containerID string) (*FakeTask, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.BootTaskErr != nil {
		c.events = append(c.events, fmt.Sprintf("StartTask failed for %s: %v", containerID, c.BootTaskErr))
		return nil, c.BootTaskErr
	}

	task := &FakeTask{
		ContainerID: containerID,
		PID:         1000 + len(c.tasks),
		Running:     true,
	}
	c.tasks[containerID] = task
	c.events = append(c.events, fmt.Sprintf("Started task for %s (PID %d)", containerID, task.PID))
	return task, nil
}

func (c *Containerd) KillTask(containerID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	task, ok := c.tasks[containerID]
	if !ok {
		return nil // Already killed, idempotent
	}

	task.Running = false
	delete(c.tasks, containerID)
	c.events = append(c.events, fmt.Sprintf("Killed task for %s", containerID))
	return nil
}

func (c *Containerd) StartSubcontainers(pauseID string, names []string) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.BootContainersErr != nil {
		c.events = append(c.events, fmt.Sprintf("StartSubcontainers failed: %v", c.BootContainersErr))
		return nil, c.BootContainersErr
	}

	var ids []string
	for _, name := range names {
		id := fmt.Sprintf("%s-%s", pauseID, name)
		c.containers[id] = &FakeContainer{ID: id, Spec: "subcontainer"}
		c.tasks[id] = &FakeTask{ContainerID: id, PID: 2000 + len(c.tasks), Running: true}
		ids = append(ids, id)
		c.events = append(c.events, fmt.Sprintf("Started subcontainer %s", id))
	}
	return ids, nil
}

func (c *Containerd) StopSubcontainers(ids []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, id := range ids {
		if task, ok := c.tasks[id]; ok {
			task.Running = false
			delete(c.tasks, id)
		}
		if container, ok := c.containers[id]; ok {
			container.Deleted = true
			delete(c.containers, id)
		}
		c.events = append(c.events, fmt.Sprintf("Stopped subcontainer %s", id))
	}
	return nil
}

// VolumeManager handles volume mounts and disk leases.
type VolumeManager struct {
	mu     sync.Mutex
	leases map[string][]string // sandboxID -> disk names
	events []string

	ConfigureErr error
}

func NewVolumeManager() *VolumeManager {
	return &VolumeManager{
		leases: make(map[string][]string),
	}
}

func (v *VolumeManager) ConfigureVolumes(sandboxID string, volumes []string) (map[string]string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.ConfigureErr != nil {
		v.events = append(v.events, fmt.Sprintf("ConfigureVolumes failed for %s: %v", sandboxID, v.ConfigureErr))
		return nil, v.ConfigureErr
	}

	mounts := make(map[string]string)
	for _, vol := range volumes {
		mounts[vol] = fmt.Sprintf("/mnt/%s", vol)
	}
	v.leases[sandboxID] = volumes
	v.events = append(v.events, fmt.Sprintf("Configured volumes for %s: %v", sandboxID, volumes))
	return mounts, nil
}

func (v *VolumeManager) ReleaseLeases(sandboxID string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if leases, ok := v.leases[sandboxID]; ok {
		v.events = append(v.events, fmt.Sprintf("Released leases for %s: %v", sandboxID, leases))
		delete(v.leases, sandboxID)
	}
	return nil
}

// MetricsCollector manages metrics registration.
type MetricsCollector struct {
	mu         sync.Mutex
	registered map[string]bool
	events     []string

	AddErr error
}

func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		registered: make(map[string]bool),
	}
}

func (m *MetricsCollector) Add(sandboxID string, cgroups string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.AddErr != nil {
		m.events = append(m.events, fmt.Sprintf("Metrics registration failed for %s: %v", sandboxID, m.AddErr))
		return m.AddErr
	}

	m.registered[sandboxID] = true
	m.events = append(m.events, fmt.Sprintf("Registered metrics for %s (cgroups: %s)", sandboxID, cgroups))
	return nil
}

func (m *MetricsCollector) Remove(sandboxID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.registered[sandboxID] {
		delete(m.registered, sandboxID)
		m.events = append(m.events, fmt.Sprintf("Deregistered metrics for %s", sandboxID))
	}
	return nil
}

// SandboxStore is a simple in-memory store for sandbox status.
// In production this would be the entity system.
type SandboxStore struct {
	mu       sync.Mutex
	statuses map[string]string
	events   []string

	UpdateErr error
}

func NewSandboxStore() *SandboxStore {
	return &SandboxStore{
		statuses: make(map[string]string),
	}
}

func (s *SandboxStore) Create(sandboxID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statuses[sandboxID] = "pending"
	s.events = append(s.events, fmt.Sprintf("Created sandbox %s with status pending", sandboxID))
}

func (s *SandboxStore) GetStatus(sandboxID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.statuses[sandboxID]
}

func (s *SandboxStore) UpdateStatus(sandboxID, status string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.UpdateErr != nil {
		s.events = append(s.events, fmt.Sprintf("Status update failed for %s: %v", sandboxID, s.UpdateErr))
		return "", s.UpdateErr
	}

	previous := s.statuses[sandboxID]
	s.statuses[sandboxID] = status
	s.events = append(s.events, fmt.Sprintf("Updated %s: %s -> %s", sandboxID, previous, status))
	return previous, nil
}

// SandboxLog collects all events for test verification.
type SandboxLog struct {
	events []string
}

func (l *SandboxLog) record(event string) {
	l.events = append(l.events, event)
}

// =============================================================================
// Saga Actions
// =============================================================================

// --- AllocateNetwork ---

type AllocateNetworkIn struct {
	SandboxID string
}

type AllocateNetworkOut struct {
	IP        string
	NetworkNS string // Would be real netns path in production
}

func AllocateNetwork(ctx context.Context, in AllocateNetworkIn) (AllocateNetworkOut, error) {
	subnet := saga.Get[*Subnet](ctx)
	log := saga.Get[*SandboxLog](ctx)

	ip, err := subnet.Allocate(in.SandboxID)
	if err != nil {
		log.record(fmt.Sprintf("AllocateNetwork failed: %v", err))
		return AllocateNetworkOut{}, err
	}

	netns := fmt.Sprintf("/var/run/netns/%s", in.SandboxID)
	log.record(fmt.Sprintf("Allocated network: IP=%s, netns=%s", ip, netns))
	return AllocateNetworkOut{IP: ip, NetworkNS: netns}, nil
}

func UndoAllocateNetwork(ctx context.Context, in AllocateNetworkIn, out AllocateNetworkOut) error {
	subnet := saga.Get[*Subnet](ctx)
	log := saga.Get[*SandboxLog](ctx)

	if err := subnet.Release(in.SandboxID); err != nil {
		return err
	}
	log.record(fmt.Sprintf("Released network: IP=%s", out.IP))
	return nil
}

// --- BuildSpec ---

type BuildSpecIn struct {
	SandboxID string
	IP        string
	NetworkNS string
	Image     string
}

type BuildSpecOut struct {
	Spec string // JSON OCI spec in production
}

func BuildSpec(ctx context.Context, in BuildSpecIn) (BuildSpecOut, error) {
	log := saga.Get[*SandboxLog](ctx)

	// In production: pull image, construct full OCI spec with mounts, env, etc.
	spec := fmt.Sprintf(`{"image":"%s","network":{"ip":"%s","netns":"%s"}}`, in.Image, in.IP, in.NetworkNS)
	log.record(fmt.Sprintf("Built container spec for %s", in.SandboxID))
	return BuildSpecOut{Spec: spec}, nil
}

func UndoBuildSpec(ctx context.Context, in BuildSpecIn, out BuildSpecOut) error {
	// BuildSpec is read-only (constructs ephemeral data), nothing to undo
	log := saga.Get[*SandboxLog](ctx)
	log.record("BuildSpec undo: no-op (ephemeral)")
	return nil
}

// --- ConfigureVolumes ---

type ConfigureVolumesIn struct {
	SandboxID string
	Volumes   []string `saga:",optional"`
}

type ConfigureVolumesOut struct {
	Mounts map[string]string // volume name -> mount path
}

func ConfigureVolumes(ctx context.Context, in ConfigureVolumesIn) (ConfigureVolumesOut, error) {
	volumes := saga.Get[*VolumeManager](ctx)
	log := saga.Get[*SandboxLog](ctx)

	if len(in.Volumes) == 0 {
		log.record("No volumes to configure")
		return ConfigureVolumesOut{Mounts: map[string]string{}}, nil
	}

	mounts, err := volumes.ConfigureVolumes(in.SandboxID, in.Volumes)
	if err != nil {
		log.record(fmt.Sprintf("ConfigureVolumes failed: %v", err))
		return ConfigureVolumesOut{}, err
	}

	log.record(fmt.Sprintf("Configured %d volumes", len(mounts)))
	return ConfigureVolumesOut{Mounts: mounts}, nil
}

func UndoConfigureVolumes(ctx context.Context, in ConfigureVolumesIn, out ConfigureVolumesOut) error {
	volumes := saga.Get[*VolumeManager](ctx)
	log := saga.Get[*SandboxLog](ctx)

	if err := volumes.ReleaseLeases(in.SandboxID); err != nil {
		return err
	}
	log.record("Released volume leases")
	return nil
}

// --- CreateContainer ---

type CreateContainerIn struct {
	SandboxID string
	Spec      string
}

type CreateContainerOut struct {
	ContainerID string
}

func CreateContainer(ctx context.Context, in CreateContainerIn) (CreateContainerOut, error) {
	containerd := saga.Get[*Containerd](ctx)
	log := saga.Get[*SandboxLog](ctx)

	containerID := fmt.Sprintf("pause-%s", in.SandboxID)
	_, err := containerd.CreateContainer(containerID, in.Spec)
	if err != nil {
		log.record(fmt.Sprintf("CreateContainer failed: %v", err))
		return CreateContainerOut{}, err
	}

	log.record(fmt.Sprintf("Created pause container %s", containerID))
	return CreateContainerOut{ContainerID: containerID}, nil
}

func UndoCreateContainer(ctx context.Context, in CreateContainerIn, out CreateContainerOut) error {
	containerd := saga.Get[*Containerd](ctx)
	log := saga.Get[*SandboxLog](ctx)

	// Kill any running task first
	_ = containerd.KillTask(out.ContainerID)

	if err := containerd.DeleteContainer(out.ContainerID); err != nil {
		return err
	}
	log.record(fmt.Sprintf("Deleted pause container %s", out.ContainerID))
	return nil
}

// --- BootTask ---

type BootTaskIn struct {
	ContainerID string
	NetworkNS   string
}

type BootTaskOut struct {
	TaskPID int
	Cgroups string
}

func BootTask(ctx context.Context, in BootTaskIn) (BootTaskOut, error) {
	containerd := saga.Get[*Containerd](ctx)
	log := saga.Get[*SandboxLog](ctx)

	task, err := containerd.StartTask(in.ContainerID)
	if err != nil {
		log.record(fmt.Sprintf("BootTask failed: %v", err))
		return BootTaskOut{}, err
	}

	cgroups := fmt.Sprintf("/sys/fs/cgroup/sandbox/%s", in.ContainerID)
	log.record(fmt.Sprintf("Started pause task PID=%d, cgroups=%s", task.PID, cgroups))
	return BootTaskOut{TaskPID: task.PID, Cgroups: cgroups}, nil
}

func UndoBootTask(ctx context.Context, in BootTaskIn, out BootTaskOut) error {
	containerd := saga.Get[*Containerd](ctx)
	log := saga.Get[*SandboxLog](ctx)

	if err := containerd.KillTask(in.ContainerID); err != nil {
		return err
	}
	log.record(fmt.Sprintf("Killed pause task PID=%d", out.TaskPID))
	return nil
}

// --- BootContainers ---

type BootContainersIn struct {
	SandboxID   string
	ContainerID string // Pause container to join
	TaskPID     int
	Mounts      map[string]string `saga:",optional"`
	Containers  []string          `saga:",optional"` // Container names to start
}

type BootContainersOut struct {
	SubcontainerIDs []string
}

func BootContainers(ctx context.Context, in BootContainersIn) (BootContainersOut, error) {
	containerd := saga.Get[*Containerd](ctx)
	log := saga.Get[*SandboxLog](ctx)

	if len(in.Containers) == 0 {
		log.record("No subcontainers to boot")
		return BootContainersOut{}, nil
	}

	ids, err := containerd.StartSubcontainers(in.ContainerID, in.Containers)
	if err != nil {
		log.record(fmt.Sprintf("BootContainers failed: %v", err))
		return BootContainersOut{}, err
	}

	log.record(fmt.Sprintf("Started %d subcontainers: %v", len(ids), ids))
	return BootContainersOut{SubcontainerIDs: ids}, nil
}

func UndoBootContainers(ctx context.Context, in BootContainersIn, out BootContainersOut) error {
	containerd := saga.Get[*Containerd](ctx)
	log := saga.Get[*SandboxLog](ctx)

	if len(out.SubcontainerIDs) == 0 {
		return nil
	}

	// Graceful shutdown: SIGTERM then SIGKILL (simulated here)
	if err := containerd.StopSubcontainers(out.SubcontainerIDs); err != nil {
		return err
	}
	log.record(fmt.Sprintf("Stopped %d subcontainers", len(out.SubcontainerIDs)))
	return nil
}

// --- AddMetrics ---

type AddMetricsIn struct {
	SandboxID string
	Cgroups   string
}

type AddMetricsOut struct {
	MetricsReady bool // Signal that metrics step is complete
}

func AddMetrics(ctx context.Context, in AddMetricsIn) (AddMetricsOut, error) {
	metrics := saga.Get[*MetricsCollector](ctx)
	log := saga.Get[*SandboxLog](ctx)

	if err := metrics.Add(in.SandboxID, in.Cgroups); err != nil {
		log.record(fmt.Sprintf("AddMetrics failed: %v", err))
		return AddMetricsOut{}, err
	}

	log.record("Registered metrics collection")
	return AddMetricsOut{MetricsReady: true}, nil
}

func UndoAddMetrics(ctx context.Context, in AddMetricsIn, out AddMetricsOut) error {
	metrics := saga.Get[*MetricsCollector](ctx)
	log := saga.Get[*SandboxLog](ctx)

	if err := metrics.Remove(in.SandboxID); err != nil {
		return err
	}
	log.record("Deregistered metrics collection")
	return nil
}

// --- UpdateStatus ---

type UpdateStatusIn struct {
	SandboxID    string
	MetricsReady bool // Creates dependency on AddMetrics
}

type UpdateStatusOut struct {
	PreviousStatus string
}

func UpdateStatus(ctx context.Context, in UpdateStatusIn) (UpdateStatusOut, error) {
	store := saga.Get[*SandboxStore](ctx)
	log := saga.Get[*SandboxLog](ctx)

	previous, err := store.UpdateStatus(in.SandboxID, "running")
	if err != nil {
		log.record(fmt.Sprintf("UpdateStatus failed: %v", err))
		return UpdateStatusOut{}, err
	}

	log.record(fmt.Sprintf("Updated status: %s -> running", previous))
	return UpdateStatusOut{PreviousStatus: previous}, nil
}

func UndoUpdateStatus(ctx context.Context, in UpdateStatusIn, out UpdateStatusOut) error {
	store := saga.Get[*SandboxStore](ctx)
	log := saga.Get[*SandboxLog](ctx)

	// Mark as dead on rollback, not restore to previous
	// (consistent with production behavior)
	_, err := store.UpdateStatus(in.SandboxID, "dead")
	if err != nil {
		return err
	}
	log.record("Marked sandbox as dead")
	return nil
}

// =============================================================================
// Helper to define the sandbox saga
// =============================================================================

func defineSandboxSaga(
	registry *saga.Registry,
	subnet *Subnet,
	containerd *Containerd,
	volumes *VolumeManager,
	metrics *MetricsCollector,
	store *SandboxStore,
	log *SandboxLog,
) {
	saga.Define("create-sandbox").
		Using(subnet).
		Using(containerd).
		Using(volumes).
		Using(metrics).
		Using(store).
		Using(log).
		Action(AllocateNetwork).Undo(UndoAllocateNetwork).
		Action(BuildSpec).Undo(UndoBuildSpec).
		Action(ConfigureVolumes).Undo(UndoConfigureVolumes).
		Action(CreateContainer).Undo(UndoCreateContainer).
		Action(BootTask).Undo(UndoBootTask).
		Action(BootContainers).Undo(UndoBootContainers).
		Action(AddMetrics).Undo(UndoAddMetrics).
		Action(UpdateStatus).Undo(UndoUpdateStatus).
		RegisterTo(registry)
}

// =============================================================================
// Tests
// =============================================================================

func TestSandboxSaga_Success(t *testing.T) {
	ctx := context.Background()

	// Set up collaborators
	subnet := NewSubnet("10.0.0.1", "10.0.0.2", "10.0.0.3")
	containerd := NewContainerd()
	volumes := NewVolumeManager()
	metrics := NewMetricsCollector()
	store := NewSandboxStore()
	log := &SandboxLog{}

	// Pre-create sandbox entity
	store.Create("sandbox-1")

	registry := saga.NewRegistry()
	defineSandboxSaga(registry, subnet, containerd, volumes, metrics, store, log)

	storage := saga.NewMemoryStorage()
	executor := saga.NewExecutor(storage, saga.WithRegistry(registry))

	err := executor.Start("create-sandbox").
		Input("sandboxid", "sandbox-1").
		Input("image", "pause:latest").
		Input("volumes", []string{"data", "logs"}).
		Input("containers", []string{"web", "worker"}).
		WithID("create-sandbox-1").
		Execute(ctx)

	if err != nil {
		t.Fatalf("saga failed: %v", err)
	}

	// Verify final state
	if store.GetStatus("sandbox-1") != "running" {
		t.Errorf("expected status running, got %s", store.GetStatus("sandbox-1"))
	}

	if len(subnet.allocated) != 1 {
		t.Errorf("expected 1 IP allocated, got %d", len(subnet.allocated))
	}

	if len(containerd.containers) != 3 { // pause + 2 subcontainers
		t.Errorf("expected 3 containers, got %d", len(containerd.containers))
	}

	if !metrics.registered["sandbox-1"] {
		t.Error("expected metrics to be registered")
	}

	t.Log("Saga events:")
	for _, event := range log.events {
		t.Logf("  - %s", event)
	}
}

func TestSandboxSaga_NetworkExhaustion(t *testing.T) {
	ctx := context.Background()

	// No IPs available!
	subnet := NewSubnet()
	containerd := NewContainerd()
	volumes := NewVolumeManager()
	metrics := NewMetricsCollector()
	store := NewSandboxStore()
	log := &SandboxLog{}

	store.Create("sandbox-1")

	registry := saga.NewRegistry()
	defineSandboxSaga(registry, subnet, containerd, volumes, metrics, store, log)

	storage := saga.NewMemoryStorage()
	executor := saga.NewExecutor(storage, saga.WithRegistry(registry))

	err := executor.Start("create-sandbox").
		Input("sandboxid", "sandbox-1").
		Input("image", "pause:latest").
		Input("containers", []string{"web"}).
		Execute(ctx)

	if err == nil {
		t.Fatal("saga should have failed")
	}

	t.Logf("Expected failure: %v", err)

	// Nothing should be allocated - failure at early step
	if len(subnet.allocated) != 0 {
		t.Errorf("expected no IPs allocated, got %d", len(subnet.allocated))
	}

	if len(containerd.containers) != 0 {
		t.Errorf("expected no containers, got %d", len(containerd.containers))
	}

	// Status depends on what ran before failure
	// ConfigureVolumes may have run (no dependencies), but UpdateStatus requires MetricsReady
	// which never happened, so status should be pending
	status := store.GetStatus("sandbox-1")
	t.Logf("Final status: %s", status)

	t.Log("Saga events:")
	for _, event := range log.events {
		t.Logf("  - %s", event)
	}
}

func TestSandboxSaga_ContainerCreationFails(t *testing.T) {
	ctx := context.Background()

	subnet := NewSubnet("10.0.0.1")
	containerd := NewContainerd()
	containerd.CreateContainerErr = errors.New("image not found")
	volumes := NewVolumeManager()
	metrics := NewMetricsCollector()
	store := NewSandboxStore()
	log := &SandboxLog{}

	store.Create("sandbox-1")

	registry := saga.NewRegistry()
	defineSandboxSaga(registry, subnet, containerd, volumes, metrics, store, log)

	storage := saga.NewMemoryStorage()
	executor := saga.NewExecutor(storage, saga.WithRegistry(registry))

	err := executor.Start("create-sandbox").
		Input("sandboxid", "sandbox-1").
		Input("image", "pause:latest").
		Input("volumes", []string{"data"}).
		Input("containers", []string{"web"}).
		Execute(ctx)

	if err == nil {
		t.Fatal("saga should have failed")
	}

	t.Logf("Expected failure: %v", err)

	// Network should be released (compensation ran)
	if len(subnet.allocated) != 0 {
		t.Errorf("expected IP released, got %d allocated", len(subnet.allocated))
	}

	// Volume leases should be released
	if len(volumes.leases) != 0 {
		t.Errorf("expected leases released, got %d", len(volumes.leases))
	}

	t.Log("Saga events:")
	for _, event := range log.events {
		t.Logf("  - %s", event)
	}
}

func TestSandboxSaga_BootTaskFails(t *testing.T) {
	ctx := context.Background()

	subnet := NewSubnet("10.0.0.1")
	containerd := NewContainerd()
	containerd.BootTaskErr = errors.New("cgroup limit exceeded")
	volumes := NewVolumeManager()
	metrics := NewMetricsCollector()
	store := NewSandboxStore()
	log := &SandboxLog{}

	store.Create("sandbox-1")

	registry := saga.NewRegistry()
	defineSandboxSaga(registry, subnet, containerd, volumes, metrics, store, log)

	storage := saga.NewMemoryStorage()
	executor := saga.NewExecutor(storage, saga.WithRegistry(registry))

	err := executor.Start("create-sandbox").
		Input("sandboxid", "sandbox-1").
		Input("image", "pause:latest").
		Input("containers", []string{"web"}).
		Execute(ctx)

	if err == nil {
		t.Fatal("saga should have failed")
	}

	// Container should be deleted (compensation ran)
	if len(containerd.containers) != 0 {
		t.Errorf("expected containers deleted, got %d", len(containerd.containers))
	}

	// Network should be released
	if len(subnet.allocated) != 0 {
		t.Errorf("expected IP released, got %d", len(subnet.allocated))
	}

	t.Log("Saga events:")
	for _, event := range log.events {
		t.Logf("  - %s", event)
	}
}

func TestSandboxSaga_SubcontainersFail(t *testing.T) {
	ctx := context.Background()

	subnet := NewSubnet("10.0.0.1")
	containerd := NewContainerd()
	containerd.BootContainersErr = errors.New("port already in use")
	volumes := NewVolumeManager()
	metrics := NewMetricsCollector()
	store := NewSandboxStore()
	log := &SandboxLog{}

	store.Create("sandbox-1")

	registry := saga.NewRegistry()
	defineSandboxSaga(registry, subnet, containerd, volumes, metrics, store, log)

	storage := saga.NewMemoryStorage()
	executor := saga.NewExecutor(storage, saga.WithRegistry(registry))

	err := executor.Start("create-sandbox").
		Input("sandboxid", "sandbox-1").
		Input("image", "pause:latest").
		Input("containers", []string{"web", "worker"}).
		Execute(ctx)

	if err == nil {
		t.Fatal("saga should have failed")
	}

	// Pause container and task should be cleaned up
	if len(containerd.containers) != 0 {
		t.Errorf("expected all containers deleted, got %d", len(containerd.containers))
	}

	if len(containerd.tasks) != 0 {
		t.Errorf("expected all tasks killed, got %d", len(containerd.tasks))
	}

	t.Log("Saga events:")
	for _, event := range log.events {
		t.Logf("  - %s", event)
	}
}

func TestSandboxSaga_MetricsFails_FullRollback(t *testing.T) {
	ctx := context.Background()

	subnet := NewSubnet("10.0.0.1")
	containerd := NewContainerd()
	volumes := NewVolumeManager()
	metrics := NewMetricsCollector()
	metrics.AddErr = errors.New("metrics service unavailable")
	store := NewSandboxStore()
	log := &SandboxLog{}

	store.Create("sandbox-1")

	registry := saga.NewRegistry()
	defineSandboxSaga(registry, subnet, containerd, volumes, metrics, store, log)

	storage := saga.NewMemoryStorage()
	executor := saga.NewExecutor(storage, saga.WithRegistry(registry))

	err := executor.Start("create-sandbox").
		Input("sandboxid", "sandbox-1").
		Input("image", "pause:latest").
		Input("containers", []string{"web"}).
		Execute(ctx)

	if err == nil {
		t.Fatal("saga should have failed")
	}

	// Everything should be rolled back
	if len(containerd.containers) != 0 {
		t.Errorf("expected containers deleted, got %d", len(containerd.containers))
	}

	if len(subnet.allocated) != 0 {
		t.Errorf("expected IP released, got %d", len(subnet.allocated))
	}

	// Verify the sequence: lots of actions succeeded before metrics failed
	t.Log("Saga events:")
	for _, event := range log.events {
		t.Logf("  - %s", event)
	}

	// Should see the full forward path then reverse compensation
	if len(log.events) < 10 {
		t.Errorf("expected many events showing forward+backward, got %d", len(log.events))
	}
}

func TestSandboxSaga_NoVolumes_NoContainers(t *testing.T) {
	ctx := context.Background()

	// Minimal sandbox - just pause container, no app containers or volumes
	subnet := NewSubnet("10.0.0.1")
	containerd := NewContainerd()
	volumes := NewVolumeManager()
	metrics := NewMetricsCollector()
	store := NewSandboxStore()
	log := &SandboxLog{}

	store.Create("sandbox-1")

	registry := saga.NewRegistry()
	defineSandboxSaga(registry, subnet, containerd, volumes, metrics, store, log)

	storage := saga.NewMemoryStorage()
	executor := saga.NewExecutor(storage, saga.WithRegistry(registry))

	// No volumes, no containers inputs
	err := executor.Start("create-sandbox").
		Input("sandboxid", "sandbox-1").
		Input("image", "pause:latest").
		Execute(ctx)

	if err != nil {
		t.Fatalf("saga failed: %v", err)
	}

	// Should still succeed with just pause container
	if store.GetStatus("sandbox-1") != "running" {
		t.Errorf("expected status running, got %s", store.GetStatus("sandbox-1"))
	}

	// Only pause container
	if len(containerd.containers) != 1 {
		t.Errorf("expected 1 container (pause only), got %d", len(containerd.containers))
	}

	t.Log("Saga events:")
	for _, event := range log.events {
		t.Logf("  - %s", event)
	}
}

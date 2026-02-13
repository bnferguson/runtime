//go:build linux

package commands

import (
	"log/slog"
	"net/netip"
	"os"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"miren.dev/runtime/api/entityserver/entityserver_v1alpha"
	"miren.dev/runtime/controllers/sandbox"
	"miren.dev/runtime/metrics"
	"miren.dev/runtime/network"
	"miren.dev/runtime/observability"
	"miren.dev/runtime/pkg/containerdx"
	"miren.dev/runtime/pkg/netdb"
)

// ServerState holds all server dependencies explicitly, replacing the asm.Registry.
type ServerState struct {
	// Containerd
	ContainerdSocket string
	CC               *containerd.Client
	Namespace        string

	// Network
	Bridge         string
	Subnet         *netdb.Subnet
	TargetPrefixes []netip.Prefix
	NetServ        *network.ServiceManager
	IPv4Routable   netip.Prefix

	// Paths
	DataPath string
	Tempdir  string

	// VictoriaLogs
	VictorialogsAddress string
	VictorialogsTimeout time.Duration

	// VictoriaMetrics
	VictoriametricsAddress string
	VictoriametricsTimeout time.Duration

	// Metrics
	Writer      *metrics.VictoriaMetricsWriter
	Reader      *metrics.VictoriaMetricsReader
	CPU         *metrics.CPUUsage
	Mem         *metrics.MemoryUsage
	HTTPMetrics *metrics.HTTPMetrics

	// Observability
	LogsMaintainer *observability.LogsMaintainer
	LogWriter      observability.LogWriter
	Logs           *observability.LogReader
	StatusMon      *observability.StatusMonitor

	// Sandbox
	SandboxMetrics *sandbox.Metrics
}

// NewServerState creates a new ServerState with default values.
func NewServerState() *ServerState {
	return &ServerState{
		ContainerdSocket:       containerdx.DefaultSocket,
		Namespace:              "miren",
		Bridge:                 "rt0",
		DataPath:               "/var/lib/miren",
		Tempdir:                os.TempDir(),
		VictorialogsTimeout:    30 * time.Second,
		VictoriametricsTimeout: 30 * time.Second,
	}
}

// InitContainerd creates the containerd client if not already set.
func (s *ServerState) InitContainerd() error {
	if s.CC != nil {
		return nil
	}
	cc, err := containerd.New(s.ContainerdSocket,
		containerd.WithDefaultNamespace(s.Namespace))
	if err != nil {
		return err
	}
	s.CC = cc
	return nil
}

// InitMetricsWriter creates the VictoriaMetrics writer if not already set.
func (s *ServerState) InitMetricsWriter(log *slog.Logger) {
	if s.Writer != nil {
		return
	}
	s.Writer = metrics.NewVictoriaMetricsWriter(log, s.VictoriametricsAddress, s.VictoriametricsTimeout)
	s.Writer.Start()
}

// InitMetricsReader creates the VictoriaMetrics reader if not already set.
func (s *ServerState) InitMetricsReader(log *slog.Logger) {
	if s.Reader != nil {
		return
	}
	s.Reader = metrics.NewVictoriaMetricsReader(log, s.VictoriametricsAddress, s.VictoriametricsTimeout)
}

// InitStatusMonitor creates the status monitor if not already set.
func (s *ServerState) InitStatusMonitor(log *slog.Logger) {
	if s.StatusMon != nil {
		return
	}
	s.StatusMon = observability.NewStatusMonitor(log)
}

// InitLogsMaintainer creates the logs maintainer if not already set.
func (s *ServerState) InitLogsMaintainer() {
	if s.LogsMaintainer != nil {
		return
	}
	s.LogsMaintainer = observability.NewLogsMaintainer()
}

// InitNetServ creates the network service manager if not already set.
func (s *ServerState) InitNetServ(log *slog.Logger, eac *entityserver_v1alpha.EntityAccessClient) {
	if s.NetServ != nil {
		return
	}
	s.NetServ = network.NewServiceManager(log, eac)
}

// InitSandboxMetrics creates the sandbox metrics if not already set.
// Note: CPU and Mem should be initialized before calling this method.
func (s *ServerState) InitSandboxMetrics(log *slog.Logger) {
	if s.SandboxMetrics != nil {
		return
	}
	s.SandboxMetrics = sandbox.NewMetrics()
	s.SandboxMetrics.Log = log
	if s.CPU != nil {
		s.SandboxMetrics.CPUUsage = s.CPU
	}
	if s.Mem != nil {
		s.SandboxMetrics.MemUsage = s.Mem
	}
}

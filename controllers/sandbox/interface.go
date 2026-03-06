package sandbox

import (
	"context"
	"time"

	compute "miren.dev/runtime/api/compute/compute_v1alpha"
	"miren.dev/runtime/observability"
	"miren.dev/runtime/pkg/controller"
	"miren.dev/runtime/pkg/entity"
)

// SandboxLifecycle defines the interface for sandbox lifecycle management.
// Both SandboxController and SagaSandboxController implement this interface.
type SandboxLifecycle interface {
	Init(ctx context.Context) error
	Create(ctx context.Context, co *compute.Sandbox, meta *entity.Meta) error
	Delete(ctx context.Context, id entity.Id, sb *compute.Sandbox) error
	Close() error
	Periodic(ctx context.Context, timeHorizon time.Duration) error
	SetWriteTracker(wt controller.WriteTracker)
	SetPortStatus(id string, port observability.BoundPort, status observability.PortStatus)
}

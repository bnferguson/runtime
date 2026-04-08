package addon

import (
	"context"
	"fmt"

	"github.com/containerd/containerd/v2/core/remotes/docker"

	"miren.dev/runtime/pkg/containerdx"
)

// RegistryImageChecker checks image accessibility by resolving the manifest
// against the remote registry.
type RegistryImageChecker struct{}

func NewRegistryImageChecker() *RegistryImageChecker {
	return &RegistryImageChecker{}
}

func (c *RegistryImageChecker) CheckImage(ctx context.Context, image string) error {
	resolver := docker.NewResolver(docker.ResolverOptions{
		Hosts: func(host string) ([]docker.RegistryHost, error) {
			config := containerdx.DefaultRegistryHost(host)
			return []docker.RegistryHost{config}, nil
		},
	})

	_, _, err := resolver.Resolve(ctx, image)
	if err != nil {
		return fmt.Errorf("image %q is not accessible: %w", image, err)
	}

	return nil
}

package containerdx

import (
	"net/http"

	"github.com/containerd/containerd/v2/core/remotes/docker"
)

// DefaultRegistryHost returns the docker.RegistryHost config for a given
// host using HTTPS, anonymous Docker auth, and pull+resolve capabilities.
// Docker Hub is rewritten to registry-1.docker.io.
func DefaultRegistryHost(host string) docker.RegistryHost {
	headers := make(http.Header)
	headers.Set("User-Agent", "containerd/2")

	config := docker.RegistryHost{
		Client: http.DefaultClient,
		Authorizer: docker.NewDockerAuthorizer(
			docker.WithAuthHeader(headers)),
		Host:         host,
		Scheme:       "https",
		Path:         "/v2",
		Capabilities: docker.HostCapabilityPull | docker.HostCapabilityResolve | docker.HostCapabilityPush,
	}

	if host == "docker.io" {
		config.Host = "registry-1.docker.io"
	}

	return config
}

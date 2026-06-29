// Package imagerefs centralizes all Docker image references used throughout the project.
// This provides a single source of truth for image versions and makes updates easier.
package imagerefs

// Infrastructure images. These reference exact upstream version tags served
// through the oci.miren.cloud pull-through cache (RFD-87): the proxy aliases
// each name to its real upstream registry (gcr.io, registry.k8s.io, ghcr.io,
// quay.io, Docker Hub). They were previously pinned to opaque "v1" tags from
// the hand-maintained mirror; that mirror is retired, though the old v1 tags
// stay frozen in the registry so already-deployed clusters keep resolving them
// through the proxy's legacy-tag bypass.
const (
	// etcd distributed key-value store (gcr.io/etcd-development/etcd)
	Etcd = "oci.miren.cloud/etcd:v3.5.15"

	// Kubernetes pause container for sandboxes (registry.k8s.io/pause)
	Pause = "oci.miren.cloud/pause:3.8"

	// BuildKit daemon for building containers (ghcr.io/mirendev/buildkit). Our
	// fork only publishes a rolling "latest", so we pin its digest for
	// reproducible builds; bump this when rolling out a new BuildKit.
	BuildKit = "oci.miren.cloud/buildkit@sha256:1263587b78162302359fec3485c153d44872114b8e944ef94be053cc2218679f"

	// Minio object storage server (quay.io/minio/minio)
	Minio = "oci.miren.cloud/minio:RELEASE.2025-04-03T14-56-28Z"

	// VictoriaLogs log storage server (docker.io/victoriametrics/victoria-logs)
	VictoriaLogs = "oci.miren.cloud/victoria-logs:v1.0.0-victorialogs"

	// VictoriaMetrics metrics storage server (docker.io/victoriametrics/victoria-metrics)
	VictoriaMetrics = "oci.miren.cloud/victoria-metrics:v1.106.1"

	// Miren runtime server
	Miren = "oci.miren.cloud/miren:latest"
)

// Base images for language stacks
const (
	// Default Alpine Linux base image
	AlpineDefault = "oci.miren.cloud/alpine:3.21"

	// Default Busybox image
	BusyboxDefault = "oci.miren.cloud/busybox:1.37-musl"
)

// GetPythonImage returns a Python image reference with the specified version
func GetPythonImage(version string) string {
	return "oci.miren.cloud/python:" + version + "-slim"
}

// GetRubyImage returns a Ruby image reference with the specified version
func GetRubyImage(version string) string {
	return "oci.miren.cloud/ruby:" + version + "-slim"
}

// GetGolangImage returns a Golang image reference with the specified version
func GetGolangImage(version string) string {
	return "oci.miren.cloud/golang:" + version + "-alpine"
}

// GetBunImage returns a Bun runtime image reference with the specified version
func GetBunImage(version string) string {
	return "oci.miren.cloud/bun:" + version
}

// GetNodeImage returns a Node.js image reference with the specified version
func GetNodeImage(version string) string {
	return "oci.miren.cloud/node:" + version + "-slim"
}

// GetRustImage returns a Rust image reference with the specified version
func GetRustImage(version string) string {
	return "oci.miren.cloud/rust:" + version
}

//go:build !linux

package commands

func gatherContainers(ctx *Context, socket, namespace string) ([]byte, error) {
	return []byte("containerd container listing is only supported on Linux\n"), nil
}

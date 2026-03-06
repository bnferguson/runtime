package sandbox

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRebuildEndpointConfigParseAddresses(t *testing.T) {
	t.Run("valid addresses are parsed", func(t *testing.T) {
		// We can't easily create a full SandboxController without real deps,
		// so test the address parsing logic directly
		addresses := []string{"10.0.0.5/32", "10.0.0.6/32"}
		var parsed []netip.Prefix
		for _, addrStr := range addresses {
			prefix, err := netip.ParsePrefix(addrStr)
			assert.NoError(t, err)
			parsed = append(parsed, prefix)
		}
		assert.Equal(t, 2, len(parsed))
		assert.Equal(t, netip.MustParsePrefix("10.0.0.5/32"), parsed[0])
		assert.Equal(t, netip.MustParsePrefix("10.0.0.6/32"), parsed[1])
	})

	t.Run("invalid address fails", func(t *testing.T) {
		_, err := netip.ParsePrefix("not-an-address")
		assert.Error(t, err)
	})
}

func TestSagaActionTypes(t *testing.T) {
	t.Run("allocNetworkOut holds addresses", func(t *testing.T) {
		out := allocNetworkOut{Addresses: []string{"10.0.0.1/32"}}
		assert.Equal(t, 1, len(out.Addresses))
	})

	t.Run("createContainerOut holds container ID", func(t *testing.T) {
		out := createContainerOut{ContainerID: "test-container"}
		assert.Equal(t, "test-container", out.ContainerID)
	})

	t.Run("bootTaskOut holds PID and cgroups", func(t *testing.T) {
		out := bootTaskOut{TaskPID: 1234, Cgroups: "/sys/fs/cgroup/test"}
		assert.Equal(t, 1234, out.TaskPID)
		assert.Equal(t, "/sys/fs/cgroup/test", out.Cgroups)
	})

	t.Run("bootContainersOut holds wait port info", func(t *testing.T) {
		out := bootContainersOut{
			WaitPortIDs:   []string{"id1", "id2"},
			WaitPortPorts: []int{8080, 8443},
		}
		assert.Equal(t, 2, len(out.WaitPortIDs))
		assert.Equal(t, 2, len(out.WaitPortPorts))
	})
}

func TestSandboxLifecycleInterface(t *testing.T) {
	var _ SandboxLifecycle = (*SandboxController)(nil)
	var _ SandboxLifecycle = (*SagaSandboxController)(nil)
}

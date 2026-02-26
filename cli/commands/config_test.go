package commands

import (
	"testing"

	"github.com/stretchr/testify/require"
	"miren.dev/runtime/clientconfig"
)

func TestCheckFingerprint(t *testing.T) {
	t.Run("prefix stripped", func(t *testing.T) {
		r := require.New(t)
		r.NoError(checkFingerprint("sha1:abcdef", "abcdef"))
	})

	t.Run("case insensitive", func(t *testing.T) {
		r := require.New(t)
		r.NoError(checkFingerprint("ABCDEF", "abcdef"))
	})

	t.Run("prefix and case insensitive", func(t *testing.T) {
		r := require.New(t)
		r.NoError(checkFingerprint("sha1:ABCDEF", "abcdef"))
	})

	t.Run("mismatch error", func(t *testing.T) {
		r := require.New(t)
		err := checkFingerprint("wrong", "abcdef")
		r.Error(err)
		r.Contains(err.Error(), "abcdef")
		r.Contains(err.Error(), "wrong")
	})

	t.Run("empty expected skips verification", func(t *testing.T) {
		r := require.New(t)
		r.NoError(checkFingerprint("", "abcdef"))
	})
}

func TestLoadClusterEnvVar(t *testing.T) {
	t.Run("known cluster name", func(t *testing.T) {
		r := require.New(t)

		cfg := clientconfig.NewConfig()
		cfg.SetCluster("known-name", &clientconfig.ClusterConfig{
			Hostname: "10.0.0.1:8443",
		})

		cc := ConfigCentric{cfg: cfg}
		t.Setenv("MIREN_CLUSTER", "known-name")

		cluster, name, err := cc.LoadCluster()
		r.NoError(err)
		r.Equal("known-name", name)
		r.NotNil(cluster)
		r.Equal("10.0.0.1:8443", cluster.Hostname)
	})

	t.Run("known name with fingerprint suffix still matches", func(t *testing.T) {
		r := require.New(t)

		cfg := clientconfig.NewConfig()
		cfg.SetCluster("known-name;sha1:extra", &clientconfig.ClusterConfig{
			Hostname: "10.0.0.1:8443",
		})

		cc := ConfigCentric{cfg: cfg}
		t.Setenv("MIREN_CLUSTER", "known-name;sha1:extra")

		cluster, name, err := cc.LoadCluster()
		r.NoError(err)
		r.Equal("known-name;sha1:extra", name)
		r.NotNil(cluster)
	})

	t.Run("unknown address falls through", func(t *testing.T) {
		r := require.New(t)

		cfg := clientconfig.NewConfig()
		cc := ConfigCentric{cfg: cfg}
		t.Setenv("MIREN_CLUSTER", "unknown-addr:8443")

		_, _, err := cc.LoadCluster()
		r.Error(err)
		r.Contains(err.Error(), "unknown-addr:8443")
	})

	t.Run("unknown address with fingerprint uses only address", func(t *testing.T) {
		r := require.New(t)

		cfg := clientconfig.NewConfig()
		cc := ConfigCentric{cfg: cfg}
		t.Setenv("MIREN_CLUSTER", "unknown-addr:8443;sha1:abc")

		_, _, err := cc.LoadCluster()
		r.Error(err)
		r.Contains(err.Error(), "unknown-addr:8443")
		// The fingerprint portion should not appear in the address part of the error
		r.NotContains(err.Error(), `"unknown-addr:8443;sha1:abc"`)
	})
}

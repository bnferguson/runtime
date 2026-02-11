package compute

import (
	"testing"

	"miren.dev/runtime/api/core/core_v1alpha"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigRoundTrip(t *testing.T) {
	original := core_v1alpha.Config{
		Entrypoint:     "/bin/app",
		StartDirectory: "/app",
		Port:           3000,
		Commands: []core_v1alpha.Commands{
			{Service: "web", Command: "serve"},
			{Service: "worker", Command: "work"},
		},
		Variable: []core_v1alpha.Variable{
			{Key: "FOO", Value: "bar", Source: "config"},
			{Key: "SECRET", Value: "hidden", Sensitive: true, Source: "manual"},
		},
		Services: []core_v1alpha.Services{
			{
				Name:     "web",
				Port:     8080,
				PortName: "http",
				PortType: "http",
				ServiceConcurrency: core_v1alpha.ServiceConcurrency{
					Mode:                "auto",
					RequestsPerInstance: 10,
					ScaleDownDelay:      "5m",
				},
				Env: []core_v1alpha.Env{
					{Key: "WEB_ONLY", Value: "yes", Source: "config"},
				},
				Disks: []core_v1alpha.Disks{
					{Name: "data", MountPath: "/data", SizeGb: 10, Filesystem: "ext4"},
				},
			},
			{
				Name:  "worker",
				Image: "postgres:16",
				ServiceConcurrency: core_v1alpha.ServiceConcurrency{
					Mode:         "fixed",
					NumInstances: 3,
				},
			},
		},
	}

	spec := ConfigSpecFromConfig(&original)

	// Check top-level fields
	assert.Equal(t, original.Entrypoint, spec.Entrypoint)
	assert.Equal(t, original.StartDirectory, spec.StartDirectory)

	// Check commands merged into services
	require.Len(t, spec.Services, 2)
	svcMap := make(map[string]core_v1alpha.ConfigSpecServices)
	for _, s := range spec.Services {
		svcMap[s.Name] = s
	}
	assert.Equal(t, "serve", svcMap["web"].Command)
	assert.Equal(t, "work", svcMap["worker"].Command)

	// Check variables
	require.Len(t, spec.Variables, 2)
	varMap := make(map[string]core_v1alpha.ConfigSpecVariables)
	for _, v := range spec.Variables {
		varMap[v.Key] = v
	}
	assert.Equal(t, "bar", varMap["FOO"].Value)
	assert.Equal(t, "config", varMap["FOO"].Source)
	assert.Equal(t, "hidden", varMap["SECRET"].Value)
	assert.True(t, varMap["SECRET"].Sensitive)
	assert.Equal(t, "manual", varMap["SECRET"].Source)

	// Check services
	web := svcMap["web"]
	assert.Equal(t, int64(8080), web.Port)
	assert.Equal(t, "http", web.PortName)
	assert.Equal(t, "http", web.PortType)
	assert.Equal(t, "auto", web.Concurrency.Mode)
	assert.Equal(t, int64(10), web.Concurrency.RequestsPerInstance)
	assert.Equal(t, "5m", web.Concurrency.ScaleDownDelay)
	require.Len(t, web.Env, 1)
	assert.Equal(t, "WEB_ONLY", web.Env[0].Key)
	require.Len(t, web.Disks, 1)
	assert.Equal(t, "data", web.Disks[0].Name)
	assert.Equal(t, "/data", web.Disks[0].MountPath)
	assert.Equal(t, int64(10), web.Disks[0].SizeGb)

	worker := svcMap["worker"]
	assert.Equal(t, "postgres:16", worker.Image)
	assert.Equal(t, "fixed", worker.Concurrency.Mode)
	assert.Equal(t, int64(3), worker.Concurrency.NumInstances)
}

func TestConfigVersionFromConfigEmpty(t *testing.T) {
	cfg := &core_v1alpha.Config{}
	spec := ConfigSpecFromConfig(cfg)

	assert.Empty(t, spec.Entrypoint)
	assert.Empty(t, spec.StartDirectory)
	assert.Empty(t, spec.Variables)
	assert.Empty(t, spec.Services)
}

func TestGetServiceConcurrency(t *testing.T) {
	spec := &core_v1alpha.ConfigSpec{
		Services: []core_v1alpha.ConfigSpecServices{
			{
				Name: "web",
				Concurrency: core_v1alpha.ConfigSpecServicesConcurrency{
					Mode:                "auto",
					RequestsPerInstance: 10,
				},
			},
			{
				Name: "worker",
				Concurrency: core_v1alpha.ConfigSpecServicesConcurrency{
					Mode:         "fixed",
					NumInstances: 5,
				},
			},
		},
	}

	sc, err := GetServiceConcurrency(spec, "web")
	require.NoError(t, err)
	assert.Equal(t, "auto", sc.Mode)
	assert.Equal(t, int64(10), sc.RequestsPerInstance)

	sc, err = GetServiceConcurrency(spec, "worker")
	require.NoError(t, err)
	assert.Equal(t, "fixed", sc.Mode)
	assert.Equal(t, int64(5), sc.NumInstances)

	_, err = GetServiceConcurrency(spec, "missing")
	assert.Error(t, err)
}

func TestServiceCommandMerging(t *testing.T) {
	cfg := &core_v1alpha.Config{
		Commands: []core_v1alpha.Commands{
			{Service: "web", Command: "serve --port 8080"},
			{Service: "worker", Command: "run-worker"},
		},
		Services: []core_v1alpha.Services{
			{Name: "web", Port: 8080},
			{Name: "worker"},
		},
	}

	spec := ConfigSpecFromConfig(cfg)

	require.Len(t, spec.Services, 2)
	svcMap := make(map[string]core_v1alpha.ConfigSpecServices)
	for _, s := range spec.Services {
		svcMap[s.Name] = s
	}

	assert.Equal(t, "serve --port 8080", svcMap["web"].Command)
	assert.Equal(t, "run-worker", svcMap["worker"].Command)
}

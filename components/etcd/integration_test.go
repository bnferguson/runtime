package etcd_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clientv3 "go.etcd.io/etcd/client/v3"
	"golang.org/x/sys/unix"
	"miren.dev/runtime/components/etcd"
	"miren.dev/runtime/pkg/testutils"
)

func TestEtcdComponentIntegration(t *testing.T) {
	testDeps, cleanup := testutils.NewTestDeps()
	defer cleanup()

	cc := testDeps.CC

	// Create temporary directory for test data
	tmpDir, err := os.MkdirTemp("", "etcd-test")
	require.NoError(t, err, "failed to create temp dir")
	defer os.RemoveAll(tmpDir)

	// Create logger
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Use dynamic namespace to avoid conflicts with parallel tests
	testNamespace := fmt.Sprintf("miren-etcd-test-%d", time.Now().UnixNano())

	// Create etcd component
	component := etcd.NewEtcdComponent(log, cc, testNamespace, tmpDir)

	// Use dynamic ports to avoid conflicts with parallel tests
	clientPort := testutils.GetFreePort(t)
	peerPort := testutils.GetFreePort(t)

	config := etcd.EtcdConfig{
		Name:         "test-etcd",
		ClientPort:   clientPort,
		PeerPort:     peerPort,
		InitialToken: "test-cluster",
		ClusterState: "new",
	}

	// Ensure cleanup
	defer func() {
		if component.IsRunning() {
			err := component.Stop(context.Background())
			if err != nil {
				t.Logf("failed to stop component: %v", err)
			}
		}

		// Clean up any remaining containers
		cleanupContainer(t, cc, testNamespace)
	}()

	// Start the etcd component
	t.Log("Starting etcd component...")
	err = component.Start(context.Background(), config)
	if err != nil {
		if strings.Contains(err.Error(), "permission denied") {
			t.Skip("permission denied error, skipping test")
		}
		require.NoError(t, err, "failed to start etcd component")
	}

	// Verify component reports as running
	assert.True(t, component.IsRunning(), "component should report as running")

	// Get client endpoint
	endpoint := component.ClientEndpoint()
	assert.NotEmpty(t, endpoint, "client endpoint should not be empty")

	expectedEndpoint := fmt.Sprintf("http://localhost:%d", clientPort)
	assert.Equal(t, expectedEndpoint, endpoint, "client endpoint should match expected")

	// Wait for etcd to be fully ready by polling
	t.Log("Waiting for etcd to be ready...")
	var etcdClient *clientv3.Client
	require.Eventually(t, func() bool {
		var err error
		etcdClient, err = clientv3.New(clientv3.Config{
			Endpoints:   []string{endpoint},
			DialTimeout: 1 * time.Second,
		})
		if err != nil {
			return false
		}
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_, err = etcdClient.Get(ctx, "health-check")
		if err != nil {
			etcdClient.Close()
			etcdClient = nil
			return false
		}
		return true
	}, 30*time.Second, 500*time.Millisecond, "etcd failed to become ready")
	defer etcdClient.Close()

	t.Log("Testing etcd functionality...")

	// Test basic key-value operations
	testKey := "test-key"
	testValue := "test-value"

	// Put operation
	t.Log("Testing Put operation...")
	ctx3, cancel3 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel3()

	_, err = etcdClient.Put(ctx3, testKey, testValue)
	require.NoError(t, err, "failed to put key-value")

	// Get operation
	t.Log("Testing Get operation...")
	ctx4, cancel4 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel4()

	resp, err := etcdClient.Get(ctx4, testKey)
	require.NoError(t, err, "failed to get key")
	require.Len(t, resp.Kvs, 1, "expected 1 key-value pair")
	assert.Equal(t, testValue, string(resp.Kvs[0].Value), "value should match")

	// Delete operation
	t.Log("Testing Delete operation...")
	ctx5, cancel5 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel5()

	_, err = etcdClient.Delete(ctx5, testKey)
	require.NoError(t, err, "failed to delete key")

	// Verify deletion
	ctx6, cancel6 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel6()

	resp, err = etcdClient.Get(ctx6, testKey)
	require.NoError(t, err, "failed to get key after deletion")
	assert.Len(t, resp.Kvs, 0, "expected 0 key-value pairs after deletion")

	// Test watch functionality
	t.Log("Testing Watch operation...")
	watchKey := "watch-test"
	watchValue := "watch-value"

	ctx7, cancel7 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel7()

	watchCh := etcdClient.Watch(ctx7, watchKey)

	// Put a value in a goroutine to trigger the watch
	go func() {
		time.Sleep(1 * time.Second)
		ctx8, cancel8 := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel8()
		etcdClient.Put(ctx8, watchKey, watchValue)
	}()

	// Wait for watch event
	select {
	case watchResp := <-watchCh:
		require.NoError(t, watchResp.Err(), "watch should not error")
		require.Len(t, watchResp.Events, 1, "expected 1 watch event")
		assert.Equal(t, watchValue, string(watchResp.Events[0].Kv.Value), "watch value should match")
	case <-ctx7.Done():
		t.Fatal("watch operation timed out")
	}

	t.Log("All etcd operations completed successfully!")

	// Test restart functionality
	t.Log("Testing restart functionality...")
	err = component.Stop(context.Background())
	require.NoError(t, err, "failed to stop component")

	assert.False(t, component.IsRunning(), "component should not report as running after stop")

	// Start again - this should use the restart logic
	err = component.Start(context.Background(), config)
	require.NoError(t, err, "failed to restart etcd component")

	assert.True(t, component.IsRunning(), "component should report as running after restart")

	// Wait for etcd to be ready after restart by polling
	var etcdClient2 *clientv3.Client
	require.Eventually(t, func() bool {
		var err error
		etcdClient2, err = clientv3.New(clientv3.Config{
			Endpoints:   []string{component.ClientEndpoint()},
			DialTimeout: 1 * time.Second,
		})
		if err != nil {
			return false
		}
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_, err = etcdClient2.Get(ctx, "restart-test")
		if err != nil {
			etcdClient2.Close()
			etcdClient2 = nil
			return false
		}
		return true
	}, 30*time.Second, 500*time.Millisecond, "etcd failed to become ready after restart")
	defer etcdClient2.Close()

	t.Log("Restart test completed successfully!")
}

func TestEtcdComponentAutoRestart(t *testing.T) {
	testDeps, cleanup := testutils.NewTestDeps()
	defer cleanup()

	cc := testDeps.CC

	// Create temporary directory for test data
	tmpDir, err := os.MkdirTemp("", "etcd-restart-test")
	require.NoError(t, err, "failed to create temp dir")
	defer os.RemoveAll(tmpDir)

	// Create logger
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Use dynamic namespace to avoid conflicts with parallel tests
	testNamespace := fmt.Sprintf("miren-etcd-restart-test-%d", time.Now().UnixNano())

	// Create etcd component
	component := etcd.NewEtcdComponent(log, cc, testNamespace, tmpDir)

	// Use dynamic ports to avoid conflicts
	clientPort := testutils.GetFreePort(t)
	peerPort := testutils.GetFreePort(t)
	httpClientPort := testutils.GetFreePort(t)

	config := etcd.EtcdConfig{
		Name:           "test-etcd-restart",
		ClientPort:     clientPort,
		HTTPClientPort: httpClientPort,
		PeerPort:       peerPort,
		InitialToken:   "restart-test-cluster",
		ClusterState:   "new",
	}

	// Ensure cleanup
	defer func() {
		if component.IsRunning() {
			err := component.Stop(context.Background())
			if err != nil {
				t.Logf("failed to stop component: %v", err)
			}
		}
		cleanupContainer(t, cc, testNamespace)
	}()

	// Start the etcd component
	t.Log("Starting etcd component...")
	err = component.Start(context.Background(), config)
	if err != nil {
		if strings.Contains(err.Error(), "permission denied") {
			t.Skip("permission denied error, skipping test")
		}
		require.NoError(t, err, "failed to start etcd component")
	}

	assert.True(t, component.IsRunning(), "component should report as running")

	// Wait for etcd to be fully ready
	endpoint := component.ClientEndpoint()
	var etcdClient *clientv3.Client
	require.Eventually(t, func() bool {
		var err error
		etcdClient, err = clientv3.New(clientv3.Config{
			Endpoints:   []string{endpoint},
			DialTimeout: 1 * time.Second,
		})
		if err != nil {
			return false
		}
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_, err = etcdClient.Get(ctx, "health-check")
		if err != nil {
			etcdClient.Close()
			etcdClient = nil
			return false
		}
		return true
	}, 30*time.Second, 500*time.Millisecond, "etcd failed to become ready")
	etcdClient.Close()

	t.Log("etcd is ready, now killing the task to simulate crash...")

	// Kill the etcd task directly using containerd to simulate a crash
	ctx := namespaces.WithNamespace(context.Background(), testNamespace)
	containers, err := cc.Containers(ctx)
	require.NoError(t, err, "failed to list containers")

	var etcdContainer containerd.Container
	for _, c := range containers {
		if strings.Contains(c.ID(), "etcd") {
			etcdContainer = c
			break
		}
	}
	require.NotNil(t, etcdContainer, "should find etcd container")

	task, err := etcdContainer.Task(ctx, nil)
	require.NoError(t, err, "should get task")

	// Kill the task with SIGKILL to simulate crash
	err = task.Kill(ctx, unix.SIGKILL)
	require.NoError(t, err, "should be able to kill task")

	t.Log("Task killed, waiting for auto-restart...")

	// Wait for the component to auto-restart and become ready again
	// The restart has a 2 second backoff for the first restart
	require.Eventually(t, func() bool {
		// Try to connect to etcd
		client, err := clientv3.New(clientv3.Config{
			Endpoints:   []string{endpoint},
			DialTimeout: 1 * time.Second,
		})
		if err != nil {
			return false
		}
		defer client.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_, err = client.Get(ctx, "restart-health-check")
		return err == nil
	}, 30*time.Second, 1*time.Second, "etcd should auto-restart and become ready")

	t.Log("etcd auto-restarted successfully!")

	// Verify the component still reports as running
	assert.True(t, component.IsRunning(), "component should still report as running after auto-restart")

	// Test that we can still perform operations
	etcdClient2, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{endpoint},
		DialTimeout: 5 * time.Second,
	})
	require.NoError(t, err, "should be able to create client after restart")
	defer etcdClient2.Close()

	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()

	_, err = etcdClient2.Put(ctx2, "post-restart-key", "post-restart-value")
	require.NoError(t, err, "should be able to write after auto-restart")

	resp, err := etcdClient2.Get(ctx2, "post-restart-key")
	require.NoError(t, err, "should be able to read after auto-restart")
	require.Len(t, resp.Kvs, 1, "should have one key")
	assert.Equal(t, "post-restart-value", string(resp.Kvs[0].Value), "value should match")

	t.Log("Auto-restart test completed successfully!")
}

func cleanupContainer(t *testing.T, cc *containerd.Client, namespace string) {
	ctx := context.Background()
	ctx = namespaces.WithNamespace(ctx, namespace)

	// Try to find and delete any test containers
	containers, err := cc.Containers(ctx)
	if err != nil {
		t.Logf("failed to list containers for cleanup: %v", err)
		return
	}

	for _, container := range containers {
		// Stop and delete task if it exists
		task, err := container.Task(ctx, nil)
		if err == nil {
			task.Kill(ctx, unix.SIGTERM)
			task.Wait(ctx)
			task.Delete(ctx)
		}

		// Delete container
		err = container.Delete(ctx, containerd.WithSnapshotCleanup)
		if err != nil {
			t.Logf("failed to delete container %s: %v", container.ID(), err)
		} else {
			t.Logf("cleaned up container %s", container.ID())
		}
	}
}

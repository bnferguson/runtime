package etcdtest

import (
	"context"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"miren.dev/runtime/pkg/idgen"
)

// TestEtcdClient connects to the shared etcd instance at etcd:2379 and returns
// a client along with a random prefix for test isolation. It verifies etcd is
// reachable with a short timeout so tests fail fast with a clear message when
// run outside the dev environment.
//
// Cleanup is registered automatically: all keys under the prefix are deleted
// and the client is closed when the test finishes.
func TestEtcdClient(t testing.TB) (*clientv3.Client, string) {
	t.Helper()

	client, err := clientv3.New(clientv3.Config{
		Endpoints:       []string{"etcd:2379"},
		DialTimeout:     2 * time.Second,
		MaxUnaryRetries: 2,
	})
	if err != nil {
		t.Fatalf("failed to create etcd client: %v", err)
	}

	// gRPC connections are lazy, so clientv3.New succeeds even when etcd is
	// unreachable. Do a quick probe to surface that early instead of letting
	// the test hang on its first real operation.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := client.Get(ctx, "/ping"); err != nil {
		client.Close()
		t.Fatalf("cannot reach etcd: %v\nThese tests require the dev environment. Use: make dev, then hack/it or hack/run.", err)
	}

	prefix := "/" + idgen.Gen("test")

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := client.Delete(ctx, prefix, clientv3.WithPrefix())
		if err != nil {
			t.Logf("warning: failed to cleanup etcd prefix %s: %v", prefix, err)
		}
		client.Close()
	})

	return client, prefix
}

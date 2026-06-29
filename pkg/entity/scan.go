package entity

import (
	"context"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// defaultScanPageSize bounds how many keys are fetched per etcd request when
// scanning a keyspace.
const defaultScanPageSize = 500

type scanConfig struct {
	pageSize int64
	keysOnly bool
}

type scanOption func(*scanConfig)

// withKeysOnly fetches keys without their values. Use it when the caller only
// needs the key set (e.g. listing IDs or checking for collisions), so etcd
// doesn't ship values it won't read.
func withKeysOnly() scanOption {
	return func(c *scanConfig) { c.keysOnly = true }
}

// withPageSize overrides the default per-request page size.
func withPageSize(n int64) scanOption {
	return func(c *scanConfig) { c.pageSize = n }
}

// scanPaged reads every key/value under prefix in ascending key order, fetching
// them in bounded pages rather than a single Get(WithPrefix()).
//
// A single unbounded prefix read loads the entire keyspace in one RPC. On a
// large or not-recently-compacted store that request can stall, which is a
// problem when it runs early in coordinator startup. Paging keeps each request
// bounded and predictable regardless of store size, so every full-keyspace read
// should route through here instead of open-coding Get(WithPrefix()).
func scanPaged(ctx context.Context, client *clientv3.Client, prefix string, opts ...scanOption) ([]*mvccpb.KeyValue, error) {
	cfg := scanConfig{pageSize: defaultScanPageSize}
	for _, opt := range opts {
		opt(&cfg)
	}

	getOpts := []clientv3.OpOption{
		clientv3.WithRange(clientv3.GetPrefixRangeEnd(prefix)),
		clientv3.WithLimit(cfg.pageSize),
		clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend),
	}
	if cfg.keysOnly {
		getOpts = append(getOpts, clientv3.WithKeysOnly())
	}

	next := prefix

	var kvs []*mvccpb.KeyValue
	first := true
	for {
		resp, err := client.Get(ctx, next, getOpts...)
		if err != nil {
			return nil, err
		}

		// Pin every page after the first to the revision the scan opened at, so
		// concurrent writes can't make us miss or duplicate keys partway
		// through. A scan that outruns the compaction window will then fail
		// loudly (ErrCompacted) rather than return a torn result.
		if first {
			getOpts = append(getOpts, clientv3.WithRev(resp.Header.Revision))
			first = false
		}

		kvs = append(kvs, resp.Kvs...)

		// An empty page can't advance the cursor; stop rather than index
		// Kvs[-1]. Unreachable while WithLimit > 0, but keeps the loop safe if
		// that ever changes.
		if !resp.More || len(resp.Kvs) == 0 {
			break
		}

		// Resume strictly after the last key returned in this page.
		next = string(resp.Kvs[len(resp.Kvs)-1].Key) + "\x00"
	}

	return kvs, nil
}

// scanPaged reads every key/value under prefix from the store's etcd client in
// bounded pages. See the package-level scanPaged for why.
func (s *EtcdStore) scanPaged(ctx context.Context, prefix string, opts ...scanOption) ([]*mvccpb.KeyValue, error) {
	return scanPaged(ctx, s.client, prefix, opts...)
}

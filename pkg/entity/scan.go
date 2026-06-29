package entity

import (
	"context"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// defaultScanPageSize bounds how many entities are fetched per etcd request
// when scanning the entity keyspace.
const defaultScanPageSize = 500

// listEntitiesPaged reads every key/value under prefix in ascending key order,
// fetching them in bounded pages rather than a single Get(WithPrefix()).
//
// A single unbounded prefix read loads the entire keyspace (keys and values) in
// one RPC. On a large or not-recently-compacted store that request can stall,
// which is a problem when it runs early in coordinator startup. Paging keeps
// each request bounded and predictable regardless of store size.
func listEntitiesPaged(ctx context.Context, client *clientv3.Client, prefix string) ([]*mvccpb.KeyValue, error) {
	rangeEnd := clientv3.GetPrefixRangeEnd(prefix)
	next := prefix

	var kvs []*mvccpb.KeyValue
	for {
		resp, err := client.Get(ctx, next,
			clientv3.WithRange(rangeEnd),
			clientv3.WithLimit(defaultScanPageSize),
			clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend),
		)
		if err != nil {
			return nil, err
		}

		kvs = append(kvs, resp.Kvs...)

		if !resp.More {
			break
		}

		// Resume strictly after the last key returned in this page.
		next = string(resp.Kvs[len(resp.Kvs)-1].Key) + "\x00"
	}

	return kvs, nil
}

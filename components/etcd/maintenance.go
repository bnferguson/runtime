package etcd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	maintenanceInterval = 5 * time.Minute
	defragBloatRatio    = 2.0
	warnBloatRatio      = 1.5
	errorBloatRatio     = 3.0
)

// StartMaintenanceLoop runs a background goroutine that periodically checks
// etcd database health and triggers defragmentation when the BoltDB file has
// grown significantly larger than its live data. Compaction (already configured
// as periodic/1h) marks old revisions as deleted, but BoltDB never releases
// pages without an explicit defrag.
func (e *EtcdComponent) StartMaintenanceLoop(ctx context.Context) {
	go e.maintenanceLoop(ctx)
}

func (e *EtcdComponent) maintenanceLoop(ctx context.Context) {
	endpoint := e.ClientEndpoint()
	if endpoint == "" {
		e.Log.Warn("etcd maintenance loop: no endpoint available, skipping")
		return
	}

	client, err := e.newMaintenanceClient(endpoint)
	if err != nil {
		e.Log.Error("etcd maintenance loop: failed to create client", "error", err)
		return
	}
	defer client.Close()

	ticker := time.NewTicker(maintenanceInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			e.runMaintenanceCheck(ctx, client, endpoint)
		case <-ctx.Done():
			return
		}
	}
}

func (e *EtcdComponent) newMaintenanceClient(endpoint string) (*clientv3.Client, error) {
	cfg := clientv3.Config{
		Endpoints:   []string{endpoint},
		DialTimeout: 5 * time.Second,
	}

	if e.config.TLS != nil {
		tlsCfg, err := buildMaintenanceTLSConfig(e.config.TLS.CertsDir)
		if err != nil {
			return nil, fmt.Errorf("building TLS config: %w", err)
		}
		cfg.TLS = tlsCfg
	}

	return clientv3.New(cfg)
}

func buildMaintenanceTLSConfig(certsDir string) (*tls.Config, error) {
	certFile := filepath.Join(certsDir, "server.crt")
	keyFile := filepath.Join(certsDir, "server.key")
	caFile := filepath.Join(certsDir, "ca.crt")

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("loading key pair: %w", err)
	}

	caCert, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("reading CA cert: %w", err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
	}, nil
}

func (e *EtcdComponent) runMaintenanceCheck(ctx context.Context, client *clientv3.Client, endpoint string) {
	resp, err := client.Status(ctx, endpoint)
	if err != nil {
		e.Log.Warn("etcd maintenance: failed to get status", "error", err)
		return
	}

	dbSize := resp.DbSize
	dbSizeInUse := resp.DbSizeInUse
	var bloatRatio float64
	if dbSizeInUse > 0 {
		bloatRatio = float64(dbSize) / float64(dbSizeInUse)
	}

	attrs := []any{
		"db_size_bytes", dbSize,
		"db_size_in_use_bytes", dbSizeInUse,
		"bloat_ratio", bloatRatio,
	}

	switch {
	case bloatRatio >= errorBloatRatio:
		e.Log.Error("etcd database bloat is critically high", attrs...)
	case bloatRatio >= warnBloatRatio:
		e.Log.Warn("etcd database bloat is elevated", attrs...)
	default:
		e.Log.Info("etcd database status", attrs...)
	}

	if dbSizeInUse > 0 && dbSize > int64(defragBloatRatio*float64(dbSizeInUse)) {
		e.defragment(ctx, client, endpoint, dbSize)
	}
}

func (e *EtcdComponent) defragment(ctx context.Context, client *clientv3.Client, endpoint string, dbSizeBefore int64) {
	e.Log.Info("etcd defrag starting", "db_size_before_bytes", dbSizeBefore)

	_, err := client.Defragment(ctx, endpoint)
	if err != nil {
		e.Log.Error("etcd defrag failed", "error", err)
		return
	}

	resp, err := client.Status(ctx, endpoint)
	if err != nil {
		e.Log.Warn("etcd defrag completed but failed to get post-defrag status", "error", err)
		return
	}

	reclaimed := dbSizeBefore - resp.DbSize
	e.Log.Info("etcd defrag completed",
		"db_size_before_bytes", dbSizeBefore,
		"db_size_after_bytes", resp.DbSize,
		"reclaimed_bytes", reclaimed,
	)
}

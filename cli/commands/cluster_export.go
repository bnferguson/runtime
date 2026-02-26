package commands

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/pem"
	"fmt"
)

func ClusterExportAddress(ctx *Context, opts struct {
	ConfigCentric
}) error {
	cfg, err := opts.LoadConfig()
	if err != nil {
		return err
	}

	name := opts.Cluster
	if name == "" {
		name = cfg.ActiveCluster()
	}
	if name == "" {
		return fmt.Errorf("no cluster specified and no active cluster set; use -C to specify one")
	}

	cc, err := cfg.GetCluster(name)
	if err != nil {
		return fmt.Errorf("cluster %q not found: %w", name, err)
	}
	if cc == nil {
		return fmt.Errorf("cluster %q not found", name)
	}

	block, _ := pem.Decode([]byte(cc.CACert))
	if block == nil {
		return fmt.Errorf("cluster %q has no valid CA certificate", name)
	}

	sum := sha1.Sum(block.Bytes)
	fingerprint := hex.EncodeToString(sum[:])

	ctx.Printf("%s;sha1:%s\n", cc.Hostname, fingerprint)
	return nil
}

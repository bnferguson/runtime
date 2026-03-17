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
	cc, name, err := opts.LoadCluster()
	if err != nil {
		return err
	}
	if cc == nil || name == "" {
		return fmt.Errorf("no cluster specified and no active cluster set; use -C to specify one")
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

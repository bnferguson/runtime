package sandbox

import (
	"context"
	"io"
	"log/slog"
	"net/netip"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"miren.dev/runtime/pkg/netdb"
)

func TestIPReconciler(t *testing.T) {
	newSubnet := func(t *testing.T) *netdb.Subnet {
		n, err := netdb.New(filepath.Join(t.TempDir(), "net.db"))
		require.NoError(t, err)
		s, err := n.Subnet("10.8.64.0/24")
		require.NoError(t, err)
		return s
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	reservedSet := func(t *testing.T, s *netdb.Subnet) map[string]bool {
		reserved, err := s.ReservedAddrs()
		require.NoError(t, err)
		got := make(map[string]bool, len(reserved))
		for _, a := range reserved {
			got[a.String()] = true
		}
		return got
	}

	t.Run("re-reserves a live address netdb lost", func(t *testing.T) {
		r := require.New(t)
		subnet := newSubnet(t)

		live := map[netip.Addr]bool{netip.MustParseAddr("10.8.64.5"): true}
		rec := &IPReconciler{
			Log:                log,
			Subnet:             subnet,
			LiveIPs:            func(context.Context) (map[netip.Addr]bool, error) { return live, nil },
			ReleaseAfterMisses: 2,
			misses:             make(map[netip.Addr]int),
		}

		r.NoError(rec.reconcile(context.Background()))

		r.True(reservedSet(t, subnet)["10.8.64.5"], "live-but-unreserved address should be re-reserved")
	})

	t.Run("reaps a leaked reservation only after the miss threshold", func(t *testing.T) {
		r := require.New(t)
		subnet := newSubnet(t)

		_, err := subnet.Reserve() // .2 reserved, but never live
		r.NoError(err)

		rec := &IPReconciler{
			Log:    log,
			Subnet: subnet,
			LiveIPs: func(context.Context) (map[netip.Addr]bool, error) {
				return map[netip.Addr]bool{}, nil // nothing live
			},
			ReleaseAfterMisses: 2,
			misses:             make(map[netip.Addr]int),
		}

		// First cycle: miss = 1 (< threshold), still reserved.
		r.NoError(rec.reconcile(context.Background()))
		r.Len(reservedSet(t, subnet), 1, "should not reap on the first miss")

		// Second cycle: miss = 2 (== threshold), released.
		r.NoError(rec.reconcile(context.Background()))
		r.Empty(reservedSet(t, subnet), "should reap after reaching the miss threshold")
	})

	t.Run("never reaps an address that is live", func(t *testing.T) {
		r := require.New(t)
		subnet := newSubnet(t)

		ip, err := subnet.Reserve() // .2
		r.NoError(err)

		live := map[netip.Addr]bool{ip.Addr(): true}
		rec := &IPReconciler{
			Log:                log,
			Subnet:             subnet,
			LiveIPs:            func(context.Context) (map[netip.Addr]bool, error) { return live, nil },
			ReleaseAfterMisses: 1,
			misses:             make(map[netip.Addr]int),
		}

		r.NoError(rec.reconcile(context.Background()))
		r.NoError(rec.reconcile(context.Background()))
		r.Len(reservedSet(t, subnet), 1, "a live address must never be reaped")
	})
}

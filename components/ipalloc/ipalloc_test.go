package ipalloc

import (
	"context"
	"io"
	"log/slog"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
	"miren.dev/runtime/pkg/entity"
)

func TestAllocator_random(t *testing.T) {
	r := require.New(t)

	t.Run("ipv4", func(t *testing.T) {
		prefix := netip.MustParsePrefix("10.10.0.0/16")

		seen := map[string]struct{}{}

		rr := newRandReader()

		for range 1000 {
			ra, err := generateRandomIPInSubnet(rr, prefix)
			r.NoError(err)

			seen[ra.String()] = struct{}{}

			r.True(prefix.Contains(ra))
		}

		r.Greater(len(seen), 800)
	})

	t.Run("ipv6", func(t *testing.T) {
		prefix := netip.MustParsePrefix("fdaa::/64")

		seen := map[string]struct{}{}

		rr := newRandReader()

		for range 1000 {
			ra, err := generateRandomIPInSubnet(rr, prefix)
			r.NoError(err)

			seen[ra.String()] = struct{}{}

			r.True(prefix.Contains(ra))
		}

		r.Greater(len(seen), 900)
	})
}

func TestAllocator_releaseServiceAllocations(t *testing.T) {
	r := require.New(t)

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	a := NewAllocator(log, []netip.Prefix{
		netip.MustParsePrefix("10.10.0.0/16"),
	})

	svcA := entity.Id("svc/a")
	svcB := entity.Id("svc/b")

	ipsA, err := a.Allocate(context.Background(), svcA)
	r.NoError(err)
	r.NotEmpty(ipsA)

	ipsB, err := a.Allocate(context.Background(), svcB)
	r.NoError(err)
	r.NotEmpty(ipsB)

	// Both services hold their reservations.
	r.Len(a.allocations, len(ipsA)+len(ipsB))

	// Releasing A returns only A's addresses to the pool.
	a.releaseServiceAllocations(svcA)

	for _, ip := range ipsA {
		_, ok := a.allocations[ip]
		r.Falsef(ok, "expected %s to be released", ip)
	}
	for _, ip := range ipsB {
		holder, ok := a.allocations[ip]
		r.Truef(ok, "expected %s to remain reserved", ip)
		r.Equal(svcB.String(), holder)
	}
	r.Len(a.allocations, len(ipsB))

	// A's freed addresses can be handed out again.
	ipsA2, err := a.Allocate(context.Background(), svcA)
	r.NoError(err)
	r.NotEmpty(ipsA2)
}

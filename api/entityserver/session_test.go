package entityserver

import (
	"errors"
	"testing"
	"time"
)

func TestIsLeaseGone(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"unrelated", errors.New("connection refused"), false},
		// The message as it arrives after crossing the RPC boundary from the
		// coordinator's PingSession -> etcd KeepAliveOnce.
		{"wrapped etcd", errors.New("remote error: generic unknown: failed to assert lease: etcdserver: requested lease not found"), true},
		{"bare etcd", errors.New("etcdserver: requested lease not found"), true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isLeaseGone(tc.err); got != tc.want {
				t.Fatalf("isLeaseGone(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestEvalPing(t *testing.T) {
	const ttl = 60 * time.Second
	base := time.Unix(1_700_000_000, 0)
	leaseGone := errors.New("etcdserver: requested lease not found")
	transient := errors.New("connection refused")

	t.Run("success resets failure clock", func(t *testing.T) {
		action, first := evalPing(nil, base.Add(-90*time.Second), base, ttl)
		if action != pingOK {
			t.Fatalf("action = %v, want pingOK", action)
		}
		if !first.IsZero() {
			t.Fatalf("firstFailure = %v, want zero", first)
		}
	})

	t.Run("lease gone is immediately fatal", func(t *testing.T) {
		// Fatal even on the very first failure, before any TTL window elapses.
		action, _ := evalPing(leaseGone, time.Time{}, base, ttl)
		if action != pingDead {
			t.Fatalf("action = %v, want pingDead", action)
		}
	})

	t.Run("first transient failure retries and stamps the clock", func(t *testing.T) {
		action, first := evalPing(transient, time.Time{}, base, ttl)
		if action != pingRetry {
			t.Fatalf("action = %v, want pingRetry", action)
		}
		if !first.Equal(base) {
			t.Fatalf("firstFailure = %v, want %v", first, base)
		}
	})

	t.Run("transient within TTL keeps retrying", func(t *testing.T) {
		firstFailure := base
		action, first := evalPing(transient, firstFailure, base.Add(ttl), ttl)
		if action != pingRetry {
			t.Fatalf("action = %v, want pingRetry", action)
		}
		if !first.Equal(firstFailure) {
			t.Fatalf("firstFailure = %v, want carried-forward %v", first, firstFailure)
		}
	})

	t.Run("transient past TTL is treated as lost lease", func(t *testing.T) {
		firstFailure := base
		action, _ := evalPing(transient, firstFailure, base.Add(ttl+time.Second), ttl)
		if action != pingDead {
			t.Fatalf("action = %v, want pingDead", action)
		}
	})
}

package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"miren.dev/runtime/api/compute/compute_v1alpha"
	"miren.dev/runtime/pkg/apphealth"
)

func TestPoolHealthClassify(t *testing.T) {
	cases := []struct {
		name string
		h    poolHealth
		want string
	}{
		{"in cooldown is crashed regardless of counts", poolHealth{ready: 1, desired: 1, inCooldown: true}, apphealth.Crashed},
		{"autoscale at zero is idle", poolHealth{ready: 0, desired: 0, isAutoscale: true}, apphealth.Idle},
		{"fixed at zero is starting, not idle", poolHealth{ready: 0, desired: 0, isAutoscale: false}, apphealth.Starting},
		{"all ready is healthy", poolHealth{ready: 3, desired: 3}, apphealth.Healthy},
		{"some ready is degraded", poolHealth{ready: 1, desired: 3}, apphealth.Degraded},
		{"none ready is starting", poolHealth{ready: 0, desired: 2}, apphealth.Starting},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.h.classify())
		})
	}
}

func TestPoolHealthAccumulate(t *testing.T) {
	now := time.Now()

	var h poolHealth

	p1 := &compute_v1alpha.SandboxPool{ReadyInstances: 1, DesiredInstances: 2}
	h.accumulate(p1, now)

	// A second pool in cooldown contributes its crash count and the longest
	// remaining cooldown.
	p2 := &compute_v1alpha.SandboxPool{
		ReadyInstances:        0,
		DesiredInstances:      1,
		CooldownUntil:         now.Add(30 * time.Second),
		ConsecutiveCrashCount: 4,
	}
	h.accumulate(p2, now)

	assert.Equal(t, 1, h.ready)
	assert.Equal(t, 3, h.desired)
	assert.True(t, h.inCooldown)
	assert.Equal(t, int64(4), h.crashCount)
	assert.Greater(t, h.cooldownLeft, 29*time.Second)
	assert.Equal(t, apphealth.Crashed, h.classify())
}

func TestPoolHealthAccumulate_ExpiredCooldownIgnored(t *testing.T) {
	now := time.Now()

	var h poolHealth
	p := &compute_v1alpha.SandboxPool{
		ReadyInstances:        2,
		DesiredInstances:      2,
		CooldownUntil:         now.Add(-time.Minute), // already passed
		ConsecutiveCrashCount: 9,
	}
	h.accumulate(p, now)

	assert.False(t, h.inCooldown)
	assert.Equal(t, apphealth.Healthy, h.classify())
}

package dbsaga

import "time"

// AddonConfig holds database-agnostic configuration that shared saga
// actions use instead of referencing provider-specific package constants.
// Inject via saga.Using() and retrieve with saga.Get[*AddonConfig](ctx).
type AddonConfig struct {
	AddonName    string
	Port         int64
	ReadyTimeout time.Duration
}

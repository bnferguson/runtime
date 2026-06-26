// Package apphealth defines the health classification strings that flow from
// app-status (ApplicationStatus / AppInfo) to consumers like `m app list` and
// the deploy poller. Keeping them in one place makes the server-derived value
// and the client-side interpretation a single shared contract.
package apphealth

const (
	// Healthy means every desired instance of the active version is RUNNING.
	Healthy = "healthy"
	// Degraded means at least one instance is serving but fewer than desired.
	Degraded = "degraded"
	// Starting means the version is active but no instance is serving yet.
	Starting = "starting"
	// Crashed means a pool is in crash cooldown.
	Crashed = "crashed"
	// Idle means the app is deliberately scaled to zero (no desired instances).
	Idle = "idle"
	// Unknown means there is no pool state to derive health from.
	Unknown = "unknown"
)

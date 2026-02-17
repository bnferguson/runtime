package labs

// EnableAll enables all known feature flags.
// This is used when generating help documentation to ensure
// all commands are registered regardless of feature gating.
func EnableAll() {
	mu.Lock()
	defer mu.Unlock()

	for _, name := range AllFeatures() {
		enabledFeatures[name] = true
	}
}

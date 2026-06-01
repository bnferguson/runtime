package sandbox

import (
	"context"
	"os"
	"sync"
	"time"
)

const tokenRefreshInterval = 45 * time.Minute

type tokenEntry struct {
	filePath  string
	appName   string
	sandboxID string
}

type tokenRefresher struct {
	mu      sync.Mutex
	entries map[string]tokenEntry // keyed by sandbox ID
}

func newTokenRefresher() *tokenRefresher {
	return &tokenRefresher{
		entries: make(map[string]tokenEntry),
	}
}

func (tr *tokenRefresher) register(sandboxID, filePath, appName string) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.entries[sandboxID] = tokenEntry{
		filePath:  filePath,
		appName:   appName,
		sandboxID: sandboxID,
	}
}

func (tr *tokenRefresher) unregister(sandboxID string) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	delete(tr.entries, sandboxID)
}

func (tr *tokenRefresher) snapshot() []tokenEntry {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	entries := make([]tokenEntry, 0, len(tr.entries))
	for _, e := range tr.entries {
		entries = append(entries, e)
	}
	return entries
}

func (c *SandboxController) runTokenRefresh(ctx context.Context) {
	ticker := time.NewTicker(tokenRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.refreshTokens()
		}
	}
}

func (c *SandboxController) refreshTokens() {
	if c.WorkloadIssuer == nil || c.tokenRefresher == nil {
		return
	}

	entries := c.tokenRefresher.snapshot()
	for _, e := range entries {
		token, err := c.WorkloadIssuer.IssueToken(e.appName, e.sandboxID)
		if err != nil {
			c.Log.Warn("failed to refresh workload identity token", "sandbox", e.sandboxID, "error", err)
			continue
		}
		if err := os.WriteFile(e.filePath, []byte(token), 0444); err != nil {
			c.Log.Warn("failed to write refreshed workload identity token", "sandbox", e.sandboxID, "error", err)
		}
	}

	if len(entries) > 0 {
		c.Log.Debug("refreshed workload identity tokens", "count", len(entries))
	}
}

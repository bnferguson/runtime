package release

import (
	"testing"
	"time"
)

func TestVersionInfo_Equivalent(t *testing.T) {
	now := time.Now()
	later := now.Add(5 * time.Minute)

	tests := []struct {
		name     string
		v        VersionInfo
		other    VersionInfo
		expected bool
	}{
		{
			name: "matching commits are equivalent regardless of build date",
			v: VersionInfo{
				Version:   "main:abc1234",
				Commit:    "abc1234def567",
				BuildDate: later,
			},
			other: VersionInfo{
				Version:   "main:abc1234",
				Commit:    "abc1234def567",
				BuildDate: now,
			},
			expected: true,
		},
		{
			name: "different commits are not equivalent",
			v: VersionInfo{
				Version: "main:abc1234",
				Commit:  "abc1234def567",
			},
			other: VersionInfo{
				Version: "main:def5678",
				Commit:  "def5678abc123",
			},
			expected: false,
		},
		{
			name: "unknown commits fall back to version + build date",
			v: VersionInfo{
				Version:   "v0.2.0",
				Commit:    "unknown",
				BuildDate: now,
			},
			other: VersionInfo{
				Version:   "v0.2.0",
				Commit:    "unknown",
				BuildDate: now,
			},
			expected: true,
		},
		{
			name: "unknown commits with different build dates not equivalent",
			v: VersionInfo{
				Version:   "v0.2.0",
				BuildDate: now,
			},
			other: VersionInfo{
				Version:   "v0.2.0",
				BuildDate: later,
			},
			expected: false,
		},
		{
			name: "different versions not equivalent even with no commits",
			v: VersionInfo{
				Version: "v0.2.0",
			},
			other: VersionInfo{
				Version: "v0.3.0",
			},
			expected: false,
		},
		{
			name: "same version, no commits, no build dates - equivalent",
			v: VersionInfo{
				Version: "main:abc1234",
			},
			other: VersionInfo{
				Version: "main:abc1234",
			},
			expected: true,
		},
		{
			name: "drift: same version string but different commits",
			v: VersionInfo{
				Version: "main",
				Commit:  "0a13763abcdef",
			},
			other: VersionInfo{
				Version: "main",
				Commit:  "9876543210fed",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.v.Equivalent(tt.other)
			if got != tt.expected {
				t.Errorf("Equivalent() = %v, expected %v", got, tt.expected)
			}
			// Equivalence is symmetric
			rev := tt.other.Equivalent(tt.v)
			if rev != tt.expected {
				t.Errorf("Equivalent() not symmetric: forward=%v reverse=%v", got, rev)
			}
		})
	}
}

func TestGetRunningServiceVersion_NotFound(t *testing.T) {
	// No real service named this should exist; ensure we get an error
	// rather than a panic. Mirrors the TestIsServerRunning style.
	_, err := GetRunningServiceVersion("definitely-not-a-real-miren-service-xyz")
	if err == nil {
		t.Error("expected error for non-existent service, got nil")
	}
}

func TestVersionInfo_IsNewer(t *testing.T) {
	now := time.Now()
	later := now.Add(5 * time.Minute)

	tests := []struct {
		name     string
		v        VersionInfo
		other    VersionInfo
		expected bool
	}{
		{
			name: "same commit different build times - not newer",
			v: VersionInfo{
				Version:   "main:abc123",
				Commit:    "abc123def456",
				BuildDate: later,
			},
			other: VersionInfo{
				Version:   "main:abc123",
				Commit:    "abc123def456",
				BuildDate: now,
			},
			expected: false,
		},
		{
			name: "different commits newer build time - newer",
			v: VersionInfo{
				Version:   "main:def456",
				Commit:    "def456abc123",
				BuildDate: later,
			},
			other: VersionInfo{
				Version:   "main:abc123",
				Commit:    "abc123def456",
				BuildDate: now,
			},
			expected: true,
		},
		{
			name: "different commits older build time - not newer",
			v: VersionInfo{
				Version:   "main:def456",
				Commit:    "def456abc123",
				BuildDate: now,
			},
			other: VersionInfo{
				Version:   "main:abc123",
				Commit:    "abc123def456",
				BuildDate: later,
			},
			expected: false,
		},
		{
			name: "no commits different build times - newer by build time",
			v: VersionInfo{
				Version:   "main:abc123",
				BuildDate: later,
			},
			other: VersionInfo{
				Version:   "main:abc123",
				BuildDate: now,
			},
			expected: true,
		},
		{
			name: "no commits same build time same version - not newer",
			v: VersionInfo{
				Version:   "main:abc123",
				BuildDate: now,
			},
			other: VersionInfo{
				Version:   "main:abc123",
				BuildDate: now,
			},
			expected: false,
		},
		{
			name: "unknown commits different build times - newer by build time",
			v: VersionInfo{
				Version:   "main:abc123",
				Commit:    "unknown",
				BuildDate: later,
			},
			other: VersionInfo{
				Version:   "main:abc123",
				Commit:    "unknown",
				BuildDate: now,
			},
			expected: true,
		},
		{
			name: "one has build date other doesn't - newer",
			v: VersionInfo{
				Version:   "main:abc123",
				Commit:    "abc123def456",
				BuildDate: now,
			},
			other: VersionInfo{
				Version: "main:abc123",
				Commit:  "def456abc123",
			},
			expected: true,
		},
		{
			name: "same version no commits no build dates - not newer",
			v: VersionInfo{
				Version: "main:abc123",
			},
			other: VersionInfo{
				Version: "main:abc123",
			},
			expected: false,
		},
		{
			name: "different versions no other info - newer",
			v: VersionInfo{
				Version: "main:def456",
			},
			other: VersionInfo{
				Version: "main:abc123",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.v.IsNewer(tt.other)
			if got != tt.expected {
				t.Errorf("IsNewer() = %v, expected %v", got, tt.expected)
			}
		})
	}
}

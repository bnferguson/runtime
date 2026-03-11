//go:build linux

package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrintSystemRequirementsGuidance(t *testing.T) {
	ctx := newTestContext()

	tests := []struct {
		name        string
		reqs        systemRequirements
		wantBlocked bool
	}{
		{
			name: "all requirements met",
			reqs: systemRequirements{
				totalMemoryBytes:  16 * 1024 * 1024 * 1024,  // 16 GB
				availStorageBytes: 200 * 1024 * 1024 * 1024, // 200 GB
				storagePath:       "/var/lib/miren",
			},
			wantBlocked: false,
		},
		{
			name: "memory below minimum blocks install",
			reqs: systemRequirements{
				totalMemoryBytes:  512 * 1024 * 1024, // 512 MB
				availStorageBytes: 200 * 1024 * 1024 * 1024,
				storagePath:       "/var/lib/miren",
			},
			wantBlocked: true,
		},
		{
			name: "storage below minimum blocks install",
			reqs: systemRequirements{
				totalMemoryBytes:  16 * 1024 * 1024 * 1024,
				availStorageBytes: 10 * 1024 * 1024 * 1024, // 10 GB
				storagePath:       "/var/lib/miren",
			},
			wantBlocked: true,
		},
		{
			name: "both below minimum blocks install",
			reqs: systemRequirements{
				totalMemoryBytes:  1 * 1024 * 1024 * 1024,  // 1 GB
				availStorageBytes: 10 * 1024 * 1024 * 1024, // 10 GB
				storagePath:       "/var/lib/miren",
			},
			wantBlocked: true,
		},
		{
			name: "memory below recommended does not block",
			reqs: systemRequirements{
				totalMemoryBytes:  6 * 1024 * 1024 * 1024, // 6 GB
				availStorageBytes: 200 * 1024 * 1024 * 1024,
				storagePath:       "/var/lib/miren",
			},
			wantBlocked: false,
		},
		{
			name: "storage below recommended does not block",
			reqs: systemRequirements{
				totalMemoryBytes:  16 * 1024 * 1024 * 1024,
				availStorageBytes: 75 * 1024 * 1024 * 1024, // 75 GB
				storagePath:       "/var/lib/miren",
			},
			wantBlocked: false,
		},
		{
			name: "memory check failed does not block",
			reqs: systemRequirements{
				memoryCheckFailed: true,
				availStorageBytes: 200 * 1024 * 1024 * 1024,
				storagePath:       "/var/lib/miren",
			},
			wantBlocked: false,
		},
		{
			name: "storage check failed does not block",
			reqs: systemRequirements{
				totalMemoryBytes:   16 * 1024 * 1024 * 1024,
				storageCheckFailed: true,
				storagePath:        "/var/lib/miren",
			},
			wantBlocked: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocked := printSystemRequirementsGuidance(ctx, tt.reqs)
			assert.Equal(t, tt.wantBlocked, blocked)
		})
	}
}

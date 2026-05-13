package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveVersionChannel(t *testing.T) {
	testCases := []struct {
		name    string
		version string
		channel string
		want    string
		wantErr bool
	}{
		{
			name: "neither set defaults to latest",
			want: "latest",
		},
		{
			name:    "channel main resolves to main",
			channel: "main",
			want:    "main",
		},
		{
			name:    "channel latest resolves to latest",
			channel: "latest",
			want:    "latest",
		},
		{
			name:    "version only is passed through",
			version: "v0.2.0",
			want:    "v0.2.0",
		},
		{
			name:    "both version and channel is an error",
			version: "v0.2.0",
			channel: "main",
			wantErr: true,
		},
		{
			name:    "version with channel=latest is still an error",
			version: "v0.2.0",
			channel: "latest",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveVersionChannel(tc.version, tc.channel)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

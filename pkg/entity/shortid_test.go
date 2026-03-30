package entity

import (
	"testing"
)

func TestExtractBase58Suffix(t *testing.T) {
	tests := []struct {
		name     string
		entityId string
		wantSfx  string
		wantOk   bool
	}{
		// Category 1: Named entities — no base58 portion
		{
			name:     "named app",
			entityId: "app/blog",
			wantOk:   false,
		},
		{
			name:     "named route",
			entityId: "http_route/example.com",
			wantOk:   false,
		},
		// Category 2: Name + prefix + base58 suffix
		{
			name:     "app version with v prefix",
			entityId: "blog-vCZ1eUgSgNd28ed6vt2DgY",
			wantSfx:  "CZ1eUgSgNd28ed6vt2DgY",
			wantOk:   true,
		},
		{
			name:     "artifact with a prefix",
			entityId: "blog-aCZ5PZNZ4MPzoPPGLishEM",
			wantSfx:  "CZ5PZNZ4MPzoPPGLishEM",
			wantOk:   true,
		},
		{
			name:     "oidc binding with ob prefix",
			entityId: "oidcb-obA1b2c3d4e5f6g7h8j9k",
			wantSfx:  "A1b2c3d4e5f6g7h8j9k",
			wantOk:   true,
		},
		// Category 3: Namespace + prefix-base58
		{
			name:     "sandbox with namespace",
			entityId: "sandbox/blog-web-CZAtBvhsMNbG38MceikkB",
			wantSfx:  "CZAtBvhsMNbG38MceikkB",
			wantOk:   true,
		},
		{
			name:     "disk volume",
			entityId: "disk_volume/disk-vol-CZAtBvhsMNbG38MceikkB",
			wantSfx:  "CZAtBvhsMNbG38MceikkB",
			wantOk:   true,
		},
		// Category 4: ForceID — kind-base58
		{
			name:     "deployment",
			entityId: "deployment-CZ1eUgSgNd28ed6vt2DgY",
			wantSfx:  "CZ1eUgSgNd28ed6vt2DgY",
			wantOk:   true,
		},
		{
			name:     "sandbox pool",
			entityId: "sandbox_pool-CZ1eUgSgNd28ed6vt2DgY",
			wantSfx:  "CZ1eUgSgNd28ed6vt2DgY",
			wantOk:   true,
		},
		// Edge cases
		{
			name:     "short segment not base58 enough",
			entityId: "app/my-app",
			wantOk:   false,
		},
		{
			name:     "empty string",
			entityId: "",
			wantOk:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSfx, gotOk := ExtractBase58Suffix(tt.entityId)
			if gotOk != tt.wantOk {
				t.Errorf("ExtractBase58Suffix(%q) ok = %v, want %v", tt.entityId, gotOk, tt.wantOk)
			}
			if gotSfx != tt.wantSfx {
				t.Errorf("ExtractBase58Suffix(%q) suffix = %q, want %q", tt.entityId, gotSfx, tt.wantSfx)
			}
		})
	}
}

func TestAllocateShortId(t *testing.T) {
	t.Run("derives from suffix", func(t *testing.T) {
		// Entity with base58 suffix ending in "DgY"
		sid, err := AllocateShortId("blog-vCZ1eUgSgNd28ed6vt2DgY", func(candidate string) (bool, error) {
			return false, nil // nothing exists
		})
		if err != nil {
			t.Fatal(err)
		}
		if sid != "DgY" {
			t.Errorf("got %q, want %q", sid, "DgY")
		}
	})

	t.Run("grows on collision", func(t *testing.T) {
		calls := 0
		sid, err := AllocateShortId("blog-vCZ1eUgSgNd28ed6vt2DgY", func(candidate string) (bool, error) {
			calls++
			// First candidate "DgY" collides, second "2DgY" is free
			return calls == 1, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if sid != "2DgY" {
			t.Errorf("got %q, want %q", sid, "2DgY")
		}
	})

	t.Run("falls back to random for named entities", func(t *testing.T) {
		sid, err := AllocateShortId("app/blog", func(candidate string) (bool, error) {
			return false, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(sid) < defaultShortIdLen {
			t.Errorf("random short-id too short: %q", sid)
		}
		if !isBase58(sid) {
			t.Errorf("random short-id contains non-base58 chars: %q", sid)
		}
	})

	t.Run("falls back to random when all suffix candidates collide", func(t *testing.T) {
		suffixCalls := 0
		sid, err := AllocateShortId("blog-vCZ1eUgSgNd28ed6vt2DgY", func(candidate string) (bool, error) {
			suffixCalls++
			// All suffix-derived candidates collide (there are 21 chars in the suffix)
			if suffixCalls <= 21-defaultShortIdLen+1 {
				return true, nil
			}
			// Random ones succeed
			return false, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(sid) < defaultShortIdLen {
			t.Errorf("fallback short-id too short: %q", sid)
		}
	})
}

func TestIsBase58(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"123456789", true},
		{"ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz", true},
		{"", false},
		{"0OIl", false},
		{"hello world", false},
	}
	for _, tt := range tests {
		got := isBase58(tt.input)
		if got != tt.want {
			t.Errorf("isBase58(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

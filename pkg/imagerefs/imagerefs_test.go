package imagerefs

import "testing"

func TestGetRubyImage(t *testing.T) {
	cases := map[string]string{
		// With our pull-through caching registry, patch-level versions
		// are fully preserved instead of being truncated to major.minor.
		"3.3.7": "oci.miren.cloud/ruby:3.3.7-slim",
		"3.3.0": "oci.miren.cloud/ruby:3.3.0-slim",
		"3.4":   "oci.miren.cloud/ruby:3.4-slim",
		"4.0":   "oci.miren.cloud/ruby:4.0-slim",
	}
	for in, want := range cases {
		if got := GetRubyImage(in); got != want {
			t.Errorf("GetRubyImage(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGetGolangImage(t *testing.T) {
	cases := map[string]string{
		// With our pull-through caching registry, patch-level versions
		// are fully preserved instead of being truncated to major.minor.
		"1.21.5": "oci.miren.cloud/golang:1.21.5-alpine",
		"1.21.0": "oci.miren.cloud/golang:1.21.0-alpine",
		"1.22":   "oci.miren.cloud/golang:1.22-alpine",
		"1.23":   "oci.miren.cloud/golang:1.23-alpine",
	}
	for in, want := range cases {
		if got := GetGolangImage(in); got != want {
			t.Errorf("GetGolangImage(%q) = %q, want %q", in, got, want)
		}
	}
}

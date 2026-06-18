package imagerefs

import "testing"

func TestGetRubyImage(t *testing.T) {
	cases := map[string]string{
		// Patch-level versions (e.g. from .ruby-version) truncate to the
		// major.minor tag the registry actually mirrors.
		"3.3.7": "oci.miren.cloud/ruby:3.3-slim",
		"3.3.0": "oci.miren.cloud/ruby:3.3-slim",
		// Already major.minor passes through unchanged.
		"3.4": "oci.miren.cloud/ruby:3.4-slim",
		"4.0": "oci.miren.cloud/ruby:4.0-slim",
	}
	for in, want := range cases {
		if got := GetRubyImage(in); got != want {
			t.Errorf("GetRubyImage(%q) = %q, want %q", in, got, want)
		}
	}
}

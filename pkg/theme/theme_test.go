package theme

import (
	"testing"

	"github.com/muesli/termenv"
)

func lookupFrom(env map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) {
		v, ok := env[k]
		return v, ok
	}
}

func TestResolve(t *testing.T) {
	const lightBG = "#ffffff"
	const darkBG = "#000000"

	tests := []struct {
		name       string
		env        map[string]string
		configured string
		bg         string // background the probe returns
		isTTY      bool
		want       Variant
		wantForce  bool
	}{
		{
			name: "NO_COLOR disables color",
			env:  map[string]string{"NO_COLOR": "1"},
			want: VariantNoColor,
		},
		{
			name:  "empty NO_COLOR is ignored",
			env:   map[string]string{"NO_COLOR": ""},
			bg:    darkBG,
			isTTY: true,
			want:  VariantDark,
		},
		{
			name:      "FORCE_COLOR overrides NO_COLOR and forces color",
			env:       map[string]string{"NO_COLOR": "1", "FORCE_COLOR": "1"},
			isTTY:     false,
			want:      VariantDark,
			wantForce: true,
		},
		{
			name: "MIREN_THEME=light wins",
			env:  map[string]string{"MIREN_THEME": "light"},
			want: VariantLight,
		},
		{
			name: "MIREN_THEME=dark wins",
			env:  map[string]string{"MIREN_THEME": "dark"},
			bg:   lightBG, // ignored because override is explicit
			want: VariantDark,
		},
		{
			name: "MIREN_THEME=no disables color",
			env:  map[string]string{"MIREN_THEME": "no"},
			want: VariantNoColor,
		},
		{
			name:       "config theme is honored when no env override",
			configured: "light",
			want:       VariantLight,
		},
		{
			name:       "MIREN_THEME env overrides config",
			env:        map[string]string{"MIREN_THEME": "dark"},
			configured: "light",
			want:       VariantDark,
		},
		{
			name: "CLICOLOR=0 disables color",
			env:  map[string]string{"CLICOLOR": "0"},
			want: VariantNoColor,
		},
		{
			name:      "CLICOLOR_FORCE re-enables color",
			env:       map[string]string{"CLICOLOR": "0", "CLICOLOR_FORCE": "1"},
			want:      VariantDark,
			wantForce: true,
		},
		{
			name: "COLORFGBG dark background",
			env:  map[string]string{"COLORFGBG": "15;0"},
			want: VariantDark,
		},
		{
			name: "COLORFGBG light background",
			env:  map[string]string{"COLORFGBG": "0;15"},
			want: VariantLight,
		},
		{
			name:  "COLORFGBG preferred over probe",
			env:   map[string]string{"COLORFGBG": "0;15"},
			bg:    darkBG,
			isTTY: true,
			want:  VariantLight,
		},
		{
			name:  "OSC probe reports light background",
			bg:    lightBG,
			isTTY: true,
			want:  VariantLight,
		},
		{
			name:  "OSC probe reports dark background",
			bg:    darkBG,
			isTTY: true,
			want:  VariantDark,
		},
		{
			name:  "no TTY defaults to dark",
			bg:    lightBG, // would be light, but probe shouldn't run
			isTTY: false,
			want:  VariantDark,
		},
		{
			name: "FORCE_COLOR=0 does not force",
			env:  map[string]string{"FORCE_COLOR": "0"},
			want: VariantDark,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			probed := false
			detectBG := func() string {
				probed = true
				return tt.bg
			}

			got, force := resolve(lookupFrom(tt.env), tt.configured, detectBG, tt.isTTY)
			if got != tt.want {
				t.Errorf("variant = %v, want %v", got, tt.want)
			}
			if force != tt.wantForce {
				t.Errorf("force = %v, want %v", force, tt.wantForce)
			}

			// The active probe must never run when stdout isn't a TTY: it reads
			// from the terminal and would hang or corrupt piped/test output.
			if probed && !tt.isTTY {
				t.Errorf("background probe ran without a TTY")
			}
		})
	}
}

func TestIsLightHex(t *testing.T) {
	tests := []struct {
		hex  string
		want bool
	}{
		{"#ffffff", true},
		{"#fdf6e3", true}, // solarized light
		{"#000000", false},
		{"#1e1e1e", false}, // typical dark theme
		{"#4d4d4d", false}, // dark gray resolves dark
		{"", false},        // unparseable -> dark
		{"not-a-color", false},
	}
	for _, tt := range tests {
		if got := isLightHex(tt.hex); got != tt.want {
			t.Errorf("isLightHex(%q) = %v, want %v", tt.hex, got, tt.want)
		}
	}
}

func TestForcedProfile(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want termenv.Profile
	}{
		{"no COLORTERM floors at ANSI256", nil, termenv.ANSI256},
		{"COLORTERM=truecolor -> truecolor", map[string]string{"COLORTERM": "truecolor"}, termenv.TrueColor},
		{"COLORTERM=24bit -> truecolor", map[string]string{"COLORTERM": "24bit"}, termenv.TrueColor},
		{"COLORTERM=yes stays ANSI256", map[string]string{"COLORTERM": "yes"}, termenv.ANSI256},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := forcedProfile(lookupFrom(tt.env)); got != tt.want {
				t.Errorf("forcedProfile = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestColorCapableTerm(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"xterm-ghostty is capable", map[string]string{"TERM": "xterm-ghostty"}, true},
		{"xterm-256color is capable", map[string]string{"TERM": "xterm-256color"}, true},
		{"dumb is not", map[string]string{"TERM": "dumb"}, false},
		{"empty TERM is not", map[string]string{"TERM": ""}, false},
		{"unset TERM is not", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := colorCapableTerm(lookupFrom(tt.env)); got != tt.want {
				t.Errorf("colorCapableTerm = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVariantFromColorFGBG(t *testing.T) {
	tests := []struct {
		in      string
		want    Variant
		matched bool
	}{
		{"15;0", VariantDark, true},
		{"0;15", VariantLight, true},
		{"15;8", VariantDark, true},        // 8 is dark (bright black)
		{"0;7", VariantLight, true},        // 7 is light (white)
		{"15;default", VariantDark, false}, // unknown -> no match
		{"", VariantDark, false},
	}
	for _, tt := range tests {
		got, matched := variantFromColorFGBG(tt.in)
		if got != tt.want || matched != tt.matched {
			t.Errorf("variantFromColorFGBG(%q) = (%v, %v), want (%v, %v)", tt.in, got, matched, tt.want, tt.matched)
		}
	}
}

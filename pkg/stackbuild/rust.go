package stackbuild

import (
	"fmt"
	"strings"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/client/llb/imagemetaresolver"
	"github.com/pelletier/go-toml/v2"
	"miren.dev/runtime/pkg/imagerefs"
)

// RustStack implements Stack for Rust
type RustStack struct {
	MetaStack

	// Detection state set in Init()
	packageName string
	edition     string
}

func (s *RustStack) Name() string {
	return "rust"
}

func (s *RustStack) Detect() bool {
	if !s.hasFile("Cargo.toml") {
		return false
	}
	s.Event("file", "Cargo.toml", "Found Cargo.toml")
	return true
}

func (s *RustStack) Init(opts BuildOptions) {
	s.SetCwd("/app")

	// Parse Cargo.toml once and extract all info
	cargo := s.parseCargoToml()
	if cargo != nil {
		s.packageName = cargo.Package.Name
		if s.packageName != "" {
			s.Event("config", "package", "Package name: "+s.packageName)
		}

		s.edition = cargo.Package.Edition
		if s.edition != "" {
			s.Event("config", "edition", "Rust edition "+s.edition)
		}
	}

	// Check for Cargo.lock
	if s.hasFile("Cargo.lock") {
		s.Event("file", "Cargo.lock", "Found Cargo.lock")
	}
}

// cargoToml represents the structure of a Cargo.toml file
type cargoToml struct {
	Package struct {
		Name    string `toml:"name"`
		Edition string `toml:"edition"`
	} `toml:"package"`
}

func (s *RustStack) parseCargoToml() *cargoToml {
	content, err := s.readFile("Cargo.toml")
	if err != nil {
		return nil
	}

	var cargo cargoToml
	if err := toml.Unmarshal(content, &cargo); err != nil {
		return nil
	}
	return &cargo
}

func (s *RustStack) GenerateLLB(dir string, opts BuildOptions) (*llb.State, error) {
	// Set up local context with the directory
	localCtx := llb.Local("context",
		llb.SharedKeyHint(dir),
		llb.ExcludePatterns([]string{".git", "target"}),
		llb.FollowPaths([]string{"."}),
		llb.WithCustomName("application code"),
	)

	version := "1"
	if opts.Version != "" {
		version = opts.Version
	}

	// NOTE: If we don't pass this in with WithMetaResolver, then
	// buildkit doesn't add the info from the image, info like
	// the PATH env var.
	mr := imagemetaresolver.Default()

	base := llb.Image(imagerefs.GetRustImage(version), llb.WithMetaResolver(mr))

	base = s.addAppUser(base)

	h := &highlevelBuilder{opts}

	// Copy the application code
	state := h.copyApp(base, localCtx)

	// Determine the binary name
	binaryName := s.packageName
	if binaryName == "" {
		binaryName = opts.Name
	}
	if binaryName == "" {
		binaryName = "app"
	}

	// Cargo converts hyphens to underscores in binary names (e.g. my-app -> my_app)
	normalizedName := strings.ReplaceAll(binaryName, "-", "_")

	// Build the application and copy it out of the cache dir.
	// Try the normalized name first (with underscores), then fall back to the original name.
	var cpCmd string
	if normalizedName != binaryName {
		cpCmd = fmt.Sprintf("cp target/release/%s /bin/app 2>/dev/null || cp target/release/%s /bin/app", normalizedName, binaryName)
	} else {
		cpCmd = fmt.Sprintf("cp target/release/%s /bin/app", binaryName)
	}

	state = state.Dir("/app").Run(
		llb.Args([]string{"/bin/sh", "-c",
			fmt.Sprintf("cargo build --release && %s", cpCmd)}),
		h.CacheMount("/usr/local/cargo/registry"),
		h.CacheMount("/app/target"),
		llb.WithCustomName("[phase] Building Rust application"),
	).Root()

	state = state.AddEnv("APP", "/bin/app")

	state = s.applyOnBuild(state, opts)
	state = s.chownApp(state)

	return &state, nil
}

func (s *RustStack) WebCommand() string {
	return "/bin/app"
}

package stackbuild

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/client/llb/imagemetaresolver"
	"miren.dev/runtime/pkg/imagerefs"
)

// GoStack implements Stack for Go
type GoStack struct {
	MetaStack

	// Detection state set in Init()
	hasVendor    bool
	hasCmdDir    bool
	cmdDir       string
	goModVersion string
}

func (s *GoStack) Name() string {
	return "go"
}

func (s *GoStack) Detect() bool {
	if !s.hasFile("go.mod") {
		return false
	}
	s.Event("file", "go.mod", "Found go.mod")
	return true
}

func (s *GoStack) Init(opts BuildOptions) {
	s.SetCwd("/app")

	// Store detection state for later use
	s.hasVendor = s.hasDir("vendor")
	if s.hasVendor {
		s.Event("dir", "vendor", "Vendor directory detected (will use -mod=vendor)")
	}

	s.hasCmdDir = s.hasDir("cmd")
	if s.hasCmdDir {
		s.Event("dir", "cmd", "cmd directory detected")
	}

	// Pre-compute the command directory
	s.cmdDir = s.commandDir(opts)
	if s.cmdDir != "" {
		s.Event("dir", s.cmdDir, "Build target directory detected")
	} else {
		s.Event("dir", ".", "No specific command directory detected, using root")
	}

	s.goModVersion = s.parseGoModVersion()
	if s.goModVersion != "" {
		s.Event("config", "go-version", "Go version "+s.goModVersion+" specified in go.mod")
	}
}

func (s *GoStack) commandDir(opts BuildOptions) string {
	if !s.hasCmdDir {
		return ""
	}

	entries, err := os.ReadDir(filepath.Join(s.dir, "cmd"))
	if err != nil {
		return ""
	}

	if len(entries) == 1 && entries[0].IsDir() {
		return filepath.Join("cmd", entries[0].Name())
	}

	for _, entry := range entries {
		if entry.IsDir() && entry.Name() == opts.Name {
			return filepath.Join("cmd", entry.Name())
		}
	}

	return ""
}

func (s *GoStack) parseGoModVersion() string {
	content, err := s.readFile("go.mod")
	if err != nil {
		return ""
	}

	lines := strings.SplitSeq(string(content), "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "go ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}
	return ""
}

func (s *GoStack) GenerateLLB(dir string, opts BuildOptions) (*llb.State, error) {
	// Set up local context with the directory
	localCtx := llb.Local("context",
		llb.SharedKeyHint(dir),
		llb.ExcludePatterns([]string{".git"}),
		llb.FollowPaths([]string{"."}),
		llb.WithCustomName("application code"),
	)

	mr := imagemetaresolver.Default()
	version := "1.23"
	if opts.Version != "" {
		version = opts.Version
	} else if s.goModVersion != "" {
		version = s.goModVersion
	}
	base := llb.Image(imagerefs.GetGolangImage(version), llb.WithMetaResolver(mr))

	// At some later time, we should convert this to use persistent cache mounts
	// but ONLY when we can actually make them persistent. For now, cache
	// within the layers.

	h := &highlevelBuilder{opts}

	// Install git for private dependencies
	state := h.apkAdd(base, "git", "ca-certificates")

	// Copy the rest of the application code
	appState := h.copyApp(state, localCtx)

	// Use the pre-computed cmdDir from Init()
	buildDir := s.cmdDir

	// Build command - skip go mod download if vendor directory exists
	var buildCmd string
	if s.hasVendor {
		buildCmd = fmt.Sprintf("go build -mod=vendor -o /bin/app ./%s", buildDir)
	} else {
		buildCmd = fmt.Sprintf("sh -c 'go mod download -json && go build -o /bin/app ./%s'", buildDir)
	}

	// Build with cache
	state = appState.Dir("/app").Run(
		llb.Shlex(buildCmd),

		// This basically is just a scratch mount until we add the ability to
		// properly export and import the cache dirs.
		h.CacheMount("/root/.cache/go-build"),
		llb.WithCustomName("[phase] Building Go application"),
	).Root()

	if opts.AlpineImage == "" {
		opts.AlpineImage = imagerefs.AlpineDefault
	}

	state = state.AddEnv("APP", "/bin/app")

	state = s.addAppUser(state)
	state = s.applyOnBuild(state, opts)
	state = s.chownApp(state)

	return &state, nil
}

func (s *GoStack) WebCommand() string {
	return "/bin/app"
}

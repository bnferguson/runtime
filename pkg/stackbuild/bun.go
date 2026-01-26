package stackbuild

import (
	"encoding/json"

	"github.com/moby/buildkit/client/llb"
	"miren.dev/runtime/pkg/imagerefs"
)

// BunStack implements Stack for Bun
type BunStack struct {
	MetaStack

	// Detection state set in Init()
	scripts    map[string]string
	entryPoint string
}

func (s *BunStack) Name() string {
	return "bun"
}

func (s *BunStack) Detect() bool {
	if !s.hasFile("package.json") {
		return false
	}
	s.Event("file", "package.json", "Found package.json")

	if s.hasFile("bun.lock") {
		s.Event("file", "bun.lock", "Found bun.lock (Bun runtime)")
		return true
	}
	if s.hasFile("bun.lockb") {
		s.Event("file", "bun.lockb", "Found bun.lockb (Bun runtime, legacy)")
		return true
	}
	if s.detectInFile("Procfile", `web:\s+bun`) {
		s.Event("file", "Procfile", "Procfile references bun")
		return true
	}
	return false
}

func (s *BunStack) Init(opts BuildOptions) {
	s.SetCwd("/app")

	// Store scripts for later use
	s.scripts = s.getPackageScripts()
	if s.scripts != nil {
		if _, ok := s.scripts["start"]; ok {
			s.Event("script", "start", "bun start script detected")
		}
	}

	// Check for common entry points and store the first one found
	for _, entry := range []string{"index.ts", "index.js", "server.ts", "server.js", "app.ts", "app.js", "main.ts", "main.js"} {
		if s.hasFile(entry) {
			s.entryPoint = entry
			s.Event("file", entry, "Entry point file detected")
			break
		}
	}
}

func (s *BunStack) GenerateLLB(dir string, opts BuildOptions) (*llb.State, error) {
	// Set up local context with the directory
	localCtx := llb.Local("context",
		llb.SharedKeyHint(dir),
		llb.ExcludePatterns([]string{".git"}),
		llb.FollowPaths([]string{"."}),
		llb.WithCustomName("application code"),
	)

	version := "1"
	if opts.Version != "" {
		version = opts.Version
	}
	base := llb.Image(imagerefs.GetBunImage(version))

	base = s.addAppUser(base)

	// Copy package files first for better caching
	pkgFiles := []string{"package.json", "bun.lock", "bun.lockb"}
	depState := base.File(llb.Copy(localCtx, "/", "/app", &llb.CopyInfo{
		IncludePatterns: pkgFiles,
	}))

	// Create bun cache mount
	bunCache := llb.Scratch().File(
		llb.Mkdir("/bun-cache", 0755, llb.WithParents(true)),
	)

	// Install dependencies with cache
	state := depState.Dir("/app").Run(
		llb.Shlex("bun install"),
		llb.AddMount("/root/.bun", bunCache, llb.AsPersistentCacheDir("bun", llb.CacheMountShared)),
		llb.WithCustomName("[phase] Installing Bun dependencies"),
	).Root()

	h := &highlevelBuilder{opts}

	// Copy the rest of the application code
	state = h.copyApp(state, localCtx)

	state = s.applyOnBuild(state, opts)

	return &state, nil
}

func (s *BunStack) getPackageScripts() map[string]string {
	data, err := s.readFile("package.json")
	if err != nil {
		return nil
	}

	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}
	return pkg.Scripts
}

func (s *BunStack) WebCommand() string {
	// Check for common web server scripts in order of preference
	if s.scripts != nil {
		for _, script := range []string{"start", "serve", "server"} {
			if _, ok := s.scripts[script]; ok {
				return "bun run " + script
			}
		}
	}

	// Fallback: use detected entry point
	if s.entryPoint != "" {
		return "bun " + s.entryPoint
	}

	return ""
}

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

	// Parsed dependencies from package.json
	dependencies    map[string]string
	devDependencies map[string]string

	// Detected environment variable requirements
	requiredEnvVars []EnvVarRequirement
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

	// Parse package.json for scripts and dependencies
	s.parsePackageJSON()

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

	// Detect required environment variables
	s.requiredEnvVars = s.detectEnvVars()
	for _, ev := range s.requiredEnvVars {
		s.Event("env_var", ev.Name, ev.Reason)
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

func (s *BunStack) parsePackageJSON() {
	data, err := s.readFile("package.json")
	if err != nil {
		return
	}

	var pkg struct {
		Scripts         map[string]string `json:"scripts"`
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return
	}

	s.scripts = pkg.Scripts
	s.dependencies = pkg.Dependencies
	s.devDependencies = pkg.DevDependencies
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

// RequiredEnvVars returns the detected environment variable requirements
func (s *BunStack) RequiredEnvVars() []EnvVarRequirement {
	return s.requiredEnvVars
}

// detectEnvVars analyzes the app to find required environment variables
// Bun uses the same patterns as Node.js since it's compatible with the Node ecosystem
func (s *BunStack) detectEnvVars() []EnvVarRequirement {
	var results []EnvVarRequirement

	// 1. Scan source code first to know what env vars are actually used
	// Bun also supports Bun.env in addition to process.env
	sourceVars := scanSourceFilesForEnvVars(s.dir, []string{".js", ".ts", ".jsx", ".tsx", ".mjs", ".cjs"}, nodeEnvPatterns, nodeOptionalEnvPatterns)

	// 2. Framework defaults - NODE_ENV is recognized by Bun
	results = append(results, EnvVarRequirement{
		Name:         "NODE_ENV",
		Source:       "bun_core",
		Confidence:   "required",
		Reason:       "Bun/Node.js environment mode",
		DefaultValue: "production",
	})

	// 3. Package-based inference with elevation logic
	// Reuse the same package map as Node.js since Bun is npm-compatible
	packageVars := s.detectPackageEnvVars()
	for _, pv := range packageVars {
		confidence := pv.Confidence
		// Elevate to required if source code references this var
		if confidence == "recommended" && elevateToRequired(pv.Name, sourceVars) {
			confidence = "required"
		}
		if !hasEnvVar(results, pv.Name) {
			results = append(results, EnvVarRequirement{
				Name:       pv.Name,
				Source:     pv.Source,
				Confidence: confidence,
				Reason:     pv.Reason,
			})
		}
	}

	// 4. Add remaining source-detected vars not covered by packages
	for _, v := range sourceVars {
		if !hasEnvVar(results, v.name) {
			confidence := "recommended"
			reason := "Referenced in application code"
			if v.optional {
				confidence = "optional"
				reason = "Referenced in application code (has default)"
			}
			results = append(results, EnvVarRequirement{
				Name:       v.name,
				Source:     "code",
				Confidence: confidence,
				Reason:     reason,
			})
		}
	}

	// 5. Config file parsing (.env.sample, .env.example)
	for _, filename := range []string{".env.sample", ".env.example"} {
		sampleVars := parseEnvSampleFile(s.dir, filename)
		for _, v := range sampleVars {
			if !hasEnvVar(results, v) {
				results = append(results, EnvVarRequirement{
					Name:       v,
					Source:     "config",
					Confidence: "required",
					Reason:     "Declared in " + filename,
				})
			}
		}
	}

	return results
}

// detectPackageEnvVars analyzes package.json to infer required env vars from dependencies
func (s *BunStack) detectPackageEnvVars() []EnvVarRequirement {
	var results []EnvVarRequirement
	seen := make(map[string]bool)

	// Check both dependencies and devDependencies
	allDeps := make(map[string]bool)
	for dep := range s.dependencies {
		allDeps[dep] = true
	}
	for dep := range s.devDependencies {
		allDeps[dep] = true
	}

	// Reuse the same package map as Node.js
	for pkg, vars := range nodePackageEnvVars {
		if allDeps[pkg] {
			for _, v := range vars {
				if !seen[v.name] {
					seen[v.name] = true
					results = append(results, EnvVarRequirement{
						Name:       v.name,
						Source:     "package",
						Confidence: v.confidence,
						Reason:     pkg + " package detected",
					})
				}
			}
		}
	}

	return results
}

package stackbuild

import (
	"encoding/json"
	"strings"

	"github.com/moby/buildkit/client/llb"
	"miren.dev/runtime/pkg/imagerefs"
)

// nodePackageManager represents the detected package manager
type nodePackageManager string

const (
	nodePkgNpm  nodePackageManager = "npm"
	nodePkgYarn nodePackageManager = "yarn"
)

// NodeStack implements Stack for Node.js
type NodeStack struct {
	MetaStack

	// Detection state set in Init()
	packageManager nodePackageManager
	scripts        map[string]string
	entryPoint     string
}

func (s *NodeStack) Name() string {
	return "node"
}

func (s *NodeStack) Detect() bool {
	if !s.hasFile("package.json") {
		return false
	}
	s.Event("file", "package.json", "Found package.json")

	if s.hasFile("yarn.lock") {
		s.packageManager = nodePkgYarn
		s.Event("file", "yarn.lock", "Found yarn.lock (yarn)")
		return true
	}
	if s.hasFile("package-lock.json") {
		s.packageManager = nodePkgNpm
		s.Event("file", "package-lock.json", "Found package-lock.json (npm)")
		return true
	}
	if s.detectInFile("Procfile", `web:\s+(node|npm|yarn)`) {
		s.packageManager = nodePkgNpm // default to npm
		s.Event("file", "Procfile", "Procfile references node/npm/yarn")
		return true
	}
	return false
}

func (s *NodeStack) Init(opts BuildOptions) {
	s.SetCwd("/app")

	// Store scripts for later use
	s.scripts = s.getPackageScripts()
	if s.scripts != nil {
		if _, ok := s.scripts["start"]; ok {
			s.Event("script", "start", "npm start script detected")
		}
		if _, ok := s.scripts["build"]; ok {
			s.Event("script", "build", "npm build script detected")
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

func (s *NodeStack) GenerateLLB(dir string, opts BuildOptions) (*llb.State, error) {
	// Set up local context with the directory
	localCtx := llb.Local("context",
		llb.SharedKeyHint(dir),
		llb.ExcludePatterns([]string{".git"}),
		llb.FollowPaths([]string{"."}),
		llb.WithCustomName("application code"),
	)

	version := "20"
	if opts.Version != "" {
		version = opts.Version
	}
	base := llb.Image(imagerefs.GetNodeImage(version))

	base = s.addAppUser(base)

	h := &highlevelBuilder{opts}

	// Copy package files first for better caching
	pkgFiles := []string{"package.json", "package-lock.json", "yarn.lock"}
	depState := base.File(llb.Copy(localCtx, "/", "/app", &llb.CopyInfo{
		IncludePatterns: pkgFiles,
	}), llb.WithCustomName("copy package files"))

	// Use the detected package manager
	var state llb.State
	switch s.packageManager {
	case nodePkgYarn:
		yarnCache := llb.Scratch().File(
			llb.Mkdir("/yarn-cache", 0755, llb.WithParents(true)),
		)

		state = depState.Dir("/app").Run(
			llb.Shlex("yarn install"),
			llb.AddMount("/usr/local/share/.cache/yarn", yarnCache, llb.AsPersistentCacheDir("yarn", llb.CacheMountShared)),
			llb.WithCustomName("[phase] Installing Node.js dependencies with yarn"),
		).Root()
	default:
		// Create cache mounts
		npmCache := llb.Scratch().File(
			llb.Mkdir("/npm-cache", 0755, llb.WithParents(true)),
		)

		state = depState.Dir("/app").Run(
			llb.Shlex("npm install"),
			llb.AddMount("/root/.npm", npmCache, llb.AsPersistentCacheDir("npm", llb.CacheMountShared)),
			llb.WithCustomName("[phase] Installing Node.js dependencies with npm"),
		).Root()
	}

	state = h.copyApp(state, localCtx)

	state = s.applyOnBuild(state, opts)

	return &state, nil
}

func (s *NodeStack) getPackageScripts() map[string]string {
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

func (s *NodeStack) WebCommand() string {
	// Determine the runner based on detected package manager
	var runner string
	if s.packageManager == nodePkgYarn {
		runner = "yarn"
	} else {
		runner = "npm run"
	}

	// Check for common web server scripts in order of preference
	if s.scripts != nil {
		for _, script := range []string{"start", "serve", "server"} {
			if _, ok := s.scripts[script]; ok {
				return runner + " " + script
			}
		}
	}

	// Fallback: use detected entry point
	if s.entryPoint != "" {
		if strings.HasSuffix(s.entryPoint, ".ts") {
			return "npx tsx " + s.entryPoint
		}
		return "node " + s.entryPoint
	}

	return ""
}

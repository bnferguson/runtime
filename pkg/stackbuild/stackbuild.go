package stackbuild

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/containerd/platforms"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/util/system"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"miren.dev/runtime/pkg/imagerefs"
)

// BuildOptions contains configuration for stack builds
type BuildOptions struct {
	Log interface{ Info(string, ...any) }

	// Name is the name of the application being built
	Name string

	// Version specifies the language/runtime version to use
	// If empty, defaults to latest stable version
	Version string

	// CacheNS specifies the namespace for persistent cache mounts
	CacheNS string

	// The alpine image to use for the base image.
	AlpineImage string

	OnBuild []string

	// EnvVars are user-configured environment variables to inject into build steps
	// (onBuild commands, asset precompilation). These are set on intermediate LLB
	// states only and do not persist to the final image config.
	EnvVars map[string]string
}

// DetectionEvent represents something detected during stack analysis
type DetectionEvent struct {
	Kind    string // e.g., "file", "package", "framework", "config"
	Name    string // e.g., "Gemfile", "rails", "puma"
	Message string // Human-readable description
}

// Stack represents a programming language/framework stack
type Stack interface {
	Name() string
	// Detect returns true if the given directory contains code for this stack
	Detect() bool
	// Init is called after detection to perform common initialization
	Init(opts BuildOptions)
	// GenerateLLB creates the BuildKit LLB for building this stack
	GenerateLLB(dir string, opts BuildOptions) (*llb.State, error)

	Image() ocispecs.Image

	Entrypoint() string

	// WebCommand returns the default command for the web service in a Procfile
	WebCommand() string

	// Events returns detection events collected during Detect() and Init()
	Events() []DetectionEvent
}

// DetectStack identifies the programming stack in the given directory
func DetectStack(dir string, opts BuildOptions) (Stack, error) {
	ms := MetaStack{dir: dir}
	ms.setupResult()

	stacks := []Stack{
		&RubyStack{MetaStack: ms},
		&PythonStack{MetaStack: ms},
		&BunStack{MetaStack: ms},
		&NodeStack{MetaStack: ms},
		&GoStack{MetaStack: ms},
		&RustStack{MetaStack: ms},
	}
	for _, stack := range stacks {
		if stack.Detect() {
			stack.Init(opts)
			return stack, nil
		}
	}

	return nil, fmt.Errorf("no supported stack detected in %s", dir)
}

// MetaStack provides shared functionality for all stack implementations
type MetaStack struct {
	dir    string
	result ocispecs.Image
	events []DetectionEvent
}

func (s *MetaStack) Init(opts BuildOptions) {
	// Base implementation does nothing; stacks can override for specific initialization
}

func (s *MetaStack) Entrypoint() string {
	return ""
}

// Event adds a detection event
func (s *MetaStack) Event(kind, name, message string) {
	s.events = append(s.events, DetectionEvent{
		Kind:    kind,
		Name:    name,
		Message: message,
	})
}

// Events returns all detection events
func (s *MetaStack) Events() []DetectionEvent {
	return s.events
}

func (s *MetaStack) setupResult() {
	pl := platforms.Normalize(platforms.DefaultSpec())
	s.result.Architecture = pl.Architecture
	s.result.OS = pl.OS
	s.result.OSVersion = pl.OSVersion
	s.result.OSFeatures = pl.OSFeatures
	s.result.Variant = pl.Variant
	s.result.RootFS.Type = "layers"
	s.result.Config.WorkingDir = "/app"
	s.result.Config.Env = []string{"PATH=" + system.DefaultPathEnv(pl.OS)}
}

func (s *MetaStack) Image() ocispecs.Image {
	return s.result
}

func (s *MetaStack) AddEnv(key, value string) {
	s.result.Config.Env = append(s.result.Config.Env, fmt.Sprintf("%s=%s", key, value))
}

func (s *MetaStack) SetEntrypoint(ep []string) {
	s.result.Config.Entrypoint = ep
}

func (s *MetaStack) SetCwd(cwd string) {
	s.result.Config.WorkingDir = cwd
}

func (s *MetaStack) SetCmd(cmd []string) {
	s.result.Config.Cmd = cmd
}

func (s *MetaStack) hasFile(path string) bool {
	st, err := os.Stat(filepath.Join(s.dir, path))
	return err == nil && st.Mode().IsRegular()
}

func (s *MetaStack) hasDir(path string) bool {
	st, err := os.Stat(filepath.Join(s.dir, path))
	return err == nil && st.Mode().IsDir()
}

func (s *MetaStack) readFile(path string) ([]byte, error) {
	return os.ReadFile(filepath.Join(s.dir, path))
}

func (s *MetaStack) detectInFile(path, re string) bool {
	content, err := s.readFile(path)
	if err != nil {
		return false
	}

	r, err := regexp.Compile(re)
	if err != nil {
		return false
	}

	return r.Match(content)
}

func (s *MetaStack) applyOnBuild(cur llb.State, opts BuildOptions) llb.State {
	// Inject user env vars so they're available to onBuild commands
	for k, v := range opts.EnvVars {
		cur = cur.AddEnv(k, v)
	}

	for _, sh := range opts.OnBuild {
		cur = cur.Dir("/app").Run(
			llb.Args([]string{"/bin/sh", "-c", sh}),
			llb.WithCustomName("[phase] Application onbuild: "+sh),
		).Root()
	}

	return cur
}

func (m *MetaStack) addAppUser(cur llb.State) llb.State {
	m.result.Config.User = "2010"

	bb := llb.Image(imagerefs.BusyboxDefault)

	return cur.Run(
		llb.Args([]string{"/bin/sh", "-c",
			"/bin/busybox addgroup -g 2011 app && /bin/busybox adduser -u 2010 -G app -D app",
		}),
		llb.WithCustomName("[phase] Adding app user"),
		llb.AddMount("/bin/busybox", bb, llb.SourcePath("/bin/busybox"), llb.Readonly),
	).State
}

func (m *MetaStack) chownApp(cur llb.State) llb.State {
	return cur.Run(
		llb.Shlex("chown -R app:app /app"),
		llb.WithCustomName("[phase] Fixing application code permissions"),
	).Root()
}

// highlevelBuilder provides high-level build helpers
type highlevelBuilder struct {
	BuildOptions
}

func (h *highlevelBuilder) CacheMount(path string) llb.RunOption {
	return h.CacheMountFrom(path, llb.Scratch())
}

func (h *highlevelBuilder) CacheMountFrom(path string, from llb.State) llb.RunOption {
	return llb.AddMount(path, from,
		llb.AsPersistentCacheDir(h.CacheNS+"-"+path, llb.CacheMountShared),
	)
}

func (h *highlevelBuilder) Access(cur llb.State, path, into string) llb.RunOption {
	return llb.AddMount(into, cur, llb.SourcePath(path), llb.Readonly)
}

func (h *highlevelBuilder) aptInstall(cur llb.State, pkgs ...string) llb.State {
	return cur.Run(
		llb.Shlexf("sh -c 'apt-get update && apt-get install -y %s'", strings.Join(pkgs, " ")),
		h.CacheMount("/var/lib/apt/lists"),
		h.CacheMount("/var/cache/apt/archives"),
		llb.WithCustomName("[phase] Installing OS packages"),
	).State
}

func (h *highlevelBuilder) apkAdd(cur llb.State, pkgs ...string) llb.State {
	return cur.Run(
		llb.Shlexf("apk add --no-cache %s", strings.Join(pkgs, " ")),
		h.CacheMount("/var/cache/apk"),
		llb.WithCustomName("[phase] Installing OS packages"),
	).State
}

func (h *highlevelBuilder) bundleInstall(cur, mnt llb.State) llb.State {
	// Because bundle install likes to modify the lock file, we copy the Gemfile and Gemfile.lock
	// in rather than using h.Access to mount them in read only.

	origin := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
	cur = cur.File(
		llb.Copy(mnt, "Gemfile*", "/app/", &llb.CopyInfo{
			CopyDirContentsOnly: true,
			CreateDestPath:      true,
			FollowSymlinks:      true,
			AllowWildcard:       true,
			AllowEmptyWildcard:  true,
			CreatedTime:         &origin,
		}))

	return cur.Dir("/app").Run(
		llb.Shlex("bundle install"),
		llb.AddEnv("BUNDLE_SILENCE_ROOT_WARNING", "true"),
		llb.WithCustomName("[phase] Installing Ruby Gem dependencies"),
	).State
}

func (h *highlevelBuilder) bootsnap(cur llb.State, args ...string) llb.State {
	return cur.Dir("/app").Run(
		llb.Shlexf("bundle exec bootsnap precompile %s", strings.Join(args, " ")),
		llb.WithCustomName("[phase] Precompiling Bootsnap cache"),
	).State
}

// appChown specifies ownership for app files (UID 2010, GID 2011)
var appChown = llb.ChownOpt{
	User:  &llb.UserOpt{UID: 2010},
	Group: &llb.UserOpt{UID: 2011},
}

func (h *highlevelBuilder) copyApp(cur, mnt llb.State) llb.State {
	origin := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
	return cur.File(
		llb.Copy(mnt, "/", "/app/", &llb.CopyInfo{
			CopyDirContentsOnly: true,
			CreateDestPath:      true,
			FollowSymlinks:      true,
			AllowWildcard:       true,
			AllowEmptyWildcard:  true,
			CreatedTime:         &origin,
			ChownOpt:            &appChown,
		}),
		llb.WithCustomName("[phase] Copying application code"),
	)
}

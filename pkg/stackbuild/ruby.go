package stackbuild

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/client/llb/imagemetaresolver"
	"miren.dev/runtime/pkg/imagerefs"
)

// RubyStack implements Stack for Ruby on Rails
type RubyStack struct {
	MetaStack
	gemfile     []byte
	gemfileLock []byte

	// Detection state set in Init()
	hasRails      bool
	hasPuma       bool
	hasUnicorn    bool
	hasBootsnap   bool
	hasConfigRu   bool
	hasPumaConfig bool
	hasRakefile   bool
}

func (s *RubyStack) Name() string {
	return "ruby"
}

func (s *RubyStack) Detect() bool {
	if !s.hasFile("Gemfile") {
		return false
	}
	s.Event("file", "Gemfile", "Found Gemfile")
	return true
}

func (s *RubyStack) Init(opts BuildOptions) {
	s.SetCwd("/app")

	// Detect framework and libraries, store state for later use
	s.hasRails = s.detectGem("rails")
	if s.hasRails {
		s.Event("framework", "rails", "Rails framework detected")
	}

	s.hasPuma = s.detectGem("puma")
	if s.hasPuma {
		s.Event("package", "puma", "Puma web server detected")
	}

	s.hasUnicorn = s.detectGem("unicorn")
	if s.hasUnicorn {
		s.Event("package", "unicorn", "Unicorn web server detected")
	}

	s.hasBootsnap = s.detectGem("bootsnap")
	if s.hasBootsnap {
		s.Event("package", "bootsnap", "Bootsnap detected (will precompile)")
	}

	s.hasConfigRu = s.hasFile("config.ru")
	if s.hasConfigRu {
		s.Event("file", "config.ru", "Rack config file detected")
	}

	s.hasPumaConfig = s.hasFile("config/puma.rb")
	if s.hasPumaConfig {
		s.Event("config", "puma.rb", "Puma configuration file detected")
	}

	s.hasRakefile = s.hasFile("Rakefile")
}

func (s *RubyStack) Gemfile() ([]byte, []byte, error) {
	if s.gemfile != nil {
		return s.gemfile, s.gemfileLock, nil
	}

	gemfilePath := "Gemfile"
	gemfileContent, err := os.ReadFile(filepath.Join(s.dir, gemfilePath))
	if err != nil {
		return nil, nil, err
	}

	s.gemfile = gemfileContent

	gemfileLockPath := "Gemfile.lock"
	gemfileLockContent, err := os.ReadFile(filepath.Join(s.dir, gemfileLockPath))
	if err != nil {
		if os.IsNotExist(err) {
			// Gemfile.lock is optional - proceed without it
			return gemfileContent, nil, nil
		}
		return gemfileContent, nil, err
	}

	s.gemfileLock = gemfileLockContent

	return gemfileContent, gemfileLockContent, nil
}

func (s *RubyStack) detectGem(gem string) bool {
	data, lock, err := s.Gemfile()
	if err != nil {
		return false
	}

	if strings.Contains(string(lock), gem) {
		return true
	}

	return strings.Contains(string(data), gem)
}

func (s *RubyStack) GenerateLLB(dir string, opts BuildOptions) (*llb.State, error) {
	// Set up local context with the directory
	localCtx := llb.Local("context",
		llb.SharedKeyHint(dir),
		llb.ExcludePatterns([]string{".git"}),
		llb.FollowPaths([]string{"."}),
		llb.WithCustomName("application code"),
	)

	mr := imagemetaresolver.Default()

	version := "3.2"
	if opts.Version != "" {
		version = opts.Version
	}
	base := llb.Image(imagerefs.GetRubyImage(version), llb.WithMetaResolver(mr))

	base = s.addAppUser(base)

	h := &highlevelBuilder{opts}

	// My kingdom for a pipe operator.
	base = h.aptInstall(base, "build-essential", "libpq-dev", "nodejs", "libyaml-dev", "postgresql-client", "git", "curl", "ssh")

	base = base.
		AddEnv("SECRET_KEY_BASE_DUMMY", "1").
		AddEnv("BUNDLE_PATH", "/usr/local/bundle").
		AddEnv("BUNDLE_WITHOUT", "development")

	base = h.bundleInstall(base, localCtx)
	base = h.copyApp(base, localCtx)

	if s.hasBootsnap {
		base = h.bootsnap(base, "--gemfile")
		base = h.bootsnap(base, "app/", "lib/")
	}

	if s.hasRakefile {
		base = base.Dir("/app").
			AddEnv("RAILS_ENV", "production").
			AddEnv("RACK_ENV", "production").
			Run(
				llb.Shlex(`sh -c 'bundle exec rake -T | grep -q "rake assets:precompile" && bundle exec rake assets:precompile || echo "no assets:precompile"'`),
				llb.AddEnv("SECRET_KEY_BASE_DUMMY", "1"),
				llb.WithCustomName("[phase] Precompiling assets"),
			).State
	}

	base = s.applyOnBuild(base, opts)

	s.AddEnv("BUNDLE_PATH", "/usr/local/bundle")
	s.AddEnv("BUNDLE_WITHOUT", "development")
	s.AddEnv("RACK_ENV", "production")

	if s.hasRails {
		s.AddEnv("RAILS_ENV", "production")
	}

	return &base, nil
}

func (s *RubyStack) Entrypoint() string {
	return "bundle exec"
}

func (s *RubyStack) WebCommand() string {
	switch {
	case s.hasRails:
		return "rails server -b 0.0.0.0 -p $PORT"
	case s.hasPuma:
		if s.hasPumaConfig {
			return "puma -C config/puma.rb"
		}
		return "puma -b tcp://0.0.0.0 -p $PORT"
	case s.hasUnicorn:
		return "unicorn -p $PORT"
	case s.hasConfigRu:
		// Covers Sinatra and other Rack apps
		return "rackup -p $PORT"
	}
	return ""
}

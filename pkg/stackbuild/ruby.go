package stackbuild

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/client/llb/imagemetaresolver"
	"miren.dev/runtime/pkg/imagerefs"
)

// rubyGemEnvVars maps gem names to the environment variables they typically require
var rubyGemEnvVars = map[string][]rubyEnvVarDef{
	"pg":            {{name: "DATABASE_URL", confidence: "required"}},
	"mysql2":        {{name: "DATABASE_URL", confidence: "required"}},
	"redis":         {{name: "REDIS_URL", confidence: "required"}},
	"sidekiq":       {{name: "REDIS_URL", confidence: "required"}},
	"aws-sdk-s3":    {{name: "AWS_ACCESS_KEY_ID", confidence: "required"}, {name: "AWS_SECRET_ACCESS_KEY", confidence: "required"}, {name: "AWS_REGION", confidence: "recommended"}},
	"aws-sdk-core":  {{name: "AWS_ACCESS_KEY_ID", confidence: "required"}, {name: "AWS_SECRET_ACCESS_KEY", confidence: "required"}, {name: "AWS_REGION", confidence: "recommended"}},
	"stripe":        {{name: "STRIPE_API_KEY", confidence: "required"}},
	"sentry-ruby":   {{name: "SENTRY_DSN", confidence: "required"}},
	"sentry-rails":  {{name: "SENTRY_DSN", confidence: "required"}},
	"honeybadger":   {{name: "HONEYBADGER_API_KEY", confidence: "required"}},
	"rollbar":       {{name: "ROLLBAR_ACCESS_TOKEN", confidence: "required"}},
	"bugsnag":       {{name: "BUGSNAG_API_KEY", confidence: "required"}},
	"newrelic_rpm":  {{name: "NEW_RELIC_LICENSE_KEY", confidence: "required"}},
	"scout_apm":     {{name: "SCOUT_KEY", confidence: "required"}},
	"sendgrid":      {{name: "SENDGRID_API_KEY", confidence: "required"}},
	"mailgun-ruby":  {{name: "MAILGUN_API_KEY", confidence: "required"}},
	"postmark":      {{name: "POSTMARK_API_TOKEN", confidence: "required"}},
	"twilio-ruby":   {{name: "TWILIO_ACCOUNT_SID", confidence: "required"}, {name: "TWILIO_AUTH_TOKEN", confidence: "required"}},
	"pusher":        {{name: "PUSHER_APP_ID", confidence: "required"}, {name: "PUSHER_KEY", confidence: "required"}, {name: "PUSHER_SECRET", confidence: "required"}},
	"elasticsearch": {{name: "ELASTICSEARCH_URL", confidence: "required"}},
	"searchkick":    {{name: "ELASTICSEARCH_URL", confidence: "required"}},
	"cloudinary":    {{name: "CLOUDINARY_URL", confidence: "required"}},
}

type rubyEnvVarDef struct {
	name       string
	confidence string
}

// rubyEnvPatterns are regex patterns to find ENV usage in Ruby source code
var rubyEnvPatterns = []*regexp.Regexp{
	// ENV['VAR'] or ENV["VAR"]
	regexp.MustCompile(`ENV\[['"]([A-Z][A-Z0-9_]+)['"]\]`),
	// ENV.fetch('VAR') or ENV.fetch('VAR', default) or ENV.fetch('VAR') { block }
	regexp.MustCompile(`ENV\.fetch\(['"]([A-Z][A-Z0-9_]+)['"]`),
}

// Patterns to detect optional ENV usage (has a default/fallback)
var optionalEnvPatterns = []*regexp.Regexp{
	// ENV.fetch('VAR') { default } - fetch with block
	regexp.MustCompile(`ENV\.fetch\(['"]([A-Z][A-Z0-9_]+)['"]\)\s*\{`),
	// ENV.fetch('VAR', default) - fetch with second argument
	regexp.MustCompile(`ENV\.fetch\(['"]([A-Z][A-Z0-9_]+)['"],`),
	// ENV['VAR'] || default - bracket access with fallback
	regexp.MustCompile(`ENV\[['"]([A-Z][A-Z0-9_]+)['"]\]\s*\|\|`),
}

// configEnvPatterns are patterns for finding ENV usage in config files (YAML with ERB, etc.)
var configEnvPatterns = []*regexp.Regexp{
	// ERB: <%= ENV['VAR'] %> or <%= ENV["VAR"] %>
	regexp.MustCompile(`<%=\s*ENV\[['"]([A-Z][A-Z0-9_]+)['"]\]\s*%>`),
	// ERB: <%= ENV.fetch('VAR') %> or with default
	regexp.MustCompile(`<%=\s*ENV\.fetch\(['"]([A-Z][A-Z0-9_]+)['"]`),
	// Plain Ruby patterns (also valid in ERB)
	regexp.MustCompile(`ENV\[['"]([A-Z][A-Z0-9_]+)['"]\]`),
	regexp.MustCompile(`ENV\.fetch\(['"]([A-Z][A-Z0-9_]+)['"]`),
}

// configEnvVar holds an env var found in a config file along with its source file
type configEnvVar struct {
	name     string
	file     string
	optional bool
}

// detectedEnvVar holds an env var found in source code with its optionality
type detectedEnvVar struct {
	name     string
	optional bool
}

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

	// Detected environment variable requirements
	requiredEnvVars []EnvVarRequirement
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

	// Detect required environment variables
	s.requiredEnvVars = s.detectEnvVars()
	for _, ev := range s.requiredEnvVars {
		s.Event("env_var", ev.Name, ev.Reason)
	}
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

// RequiredEnvVars returns the detected environment variable requirements
func (s *RubyStack) RequiredEnvVars() []EnvVarRequirement {
	return s.requiredEnvVars
}

// detectEnvVars analyzes the app to find required environment variables
func (s *RubyStack) detectEnvVars() []EnvVarRequirement {
	var results []EnvVarRequirement

	// 1. Rails core vars - SECRET_KEY_BASE is required for all Rails apps in production
	if s.hasRails {
		// RAILS_ENV should be set to production by default
		results = append(results, EnvVarRequirement{
			Name:         "RAILS_ENV",
			Source:       "rails_core",
			Confidence:   "required",
			Reason:       "Rails environment mode",
			DefaultValue: "production",
		})

		results = append(results, EnvVarRequirement{
			Name:        "SECRET_KEY_BASE",
			Source:      "rails_core",
			Confidence:  "required",
			Reason:      "Required by Rails in production",
			CanGenerate: true,
		})

		// RAILS_MASTER_KEY is used to decrypt credentials.yml.enc
		// Check if credentials file exists before recommending
		if s.hasFile("config/credentials.yml.enc") || s.hasFile("config/credentials/production.yml.enc") {
			results = append(results, EnvVarRequirement{
				Name:         "RAILS_MASTER_KEY",
				Source:       "rails_core",
				Confidence:   "required",
				Reason:       "Required to decrypt Rails encrypted credentials",
				ReadFromFile: "config/master.key",
			})
		}
	}

	// 2. Gem-based inference
	gemfile, gemfileLock, _ := s.Gemfile()
	gemVars := s.detectGemEnvVars(gemfile, gemfileLock)
	results = append(results, gemVars...)

	// 3. Source code scan
	codeVars := s.scanRubySourceForEnvVars()
	for _, v := range codeVars {
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

	// 4. Config file parsing (.env.sample, .env.example)
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

	// 5. Scan config/ directory for ENV references in YAML and other config files
	configVars := s.scanConfigDirectory()
	for _, v := range configVars {
		if !hasEnvVar(results, v.name) {
			confidence := "recommended"
			reason := "Referenced in " + v.file
			if v.optional {
				confidence = "optional"
				reason = "Referenced in " + v.file + " (has default)"
			}
			results = append(results, EnvVarRequirement{
				Name:       v.name,
				Source:     "config",
				Confidence: confidence,
				Reason:     reason,
			})
		}
	}

	return results
}

// detectGemEnvVars analyzes Gemfile and Gemfile.lock to infer required env vars from gems
func (s *RubyStack) detectGemEnvVars(gemfile, gemfileLock []byte) []EnvVarRequirement {
	var results []EnvVarRequirement
	seen := make(map[string]bool)

	// Combine gemfile and lock for searching
	content := string(gemfile) + "\n" + string(gemfileLock)

	for gem, vars := range rubyGemEnvVars {
		// Check if gem is present in Gemfile or Gemfile.lock
		if strings.Contains(content, gem) {
			for _, v := range vars {
				if !seen[v.name] {
					seen[v.name] = true
					results = append(results, EnvVarRequirement{
						Name:       v.name,
						Source:     "gem",
						Confidence: v.confidence,
						Reason:     gem + " gem detected in Gemfile",
					})
				}
			}
		}
	}

	return results
}

// scanRubySourceForEnvVars walks .rb files in the directory and extracts ENV usage
func (s *RubyStack) scanRubySourceForEnvVars() []detectedEnvVar {
	var found []detectedEnvVar
	seen := make(map[string]bool)

	_ = filepath.Walk(s.dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Skip common non-source directories
		if info.IsDir() {
			base := filepath.Base(path)
			if base == "vendor" || base == "node_modules" || base == ".git" || base == "tmp" || base == "log" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only scan Ruby files
		if !strings.HasSuffix(path, ".rb") {
			return nil
		}

		vars := scanRubyFileForEnvVars(path)
		for _, v := range vars {
			if !seen[v.name] && !ignoredEnvVars[v.name] {
				seen[v.name] = true
				found = append(found, v)
			}
		}

		return nil
	})

	return found
}

// scanRubyFileForEnvVars extracts ENV variable names from a single Ruby file
func scanRubyFileForEnvVars(path string) []detectedEnvVar {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	var found []detectedEnvVar
	seen := make(map[string]bool)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		for _, pattern := range rubyEnvPatterns {
			matches := pattern.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				if len(match) > 1 {
					varName := match[1]
					if !seen[varName] {
						seen[varName] = true
						optional := isOptionalEnvUsage(line, varName)
						found = append(found, detectedEnvVar{
							name:     varName,
							optional: optional,
						})
					}
				}
			}
		}
	}

	return found
}

// isOptionalEnvUsage checks if the line indicates the ENV var has a default/fallback
func isOptionalEnvUsage(line, varName string) bool {
	for _, pattern := range optionalEnvPatterns {
		if match := pattern.FindStringSubmatch(line); len(match) > 1 && match[1] == varName {
			return true
		}
	}
	return false
}

// scanConfigDirectory scans all files in the config/ directory for ENV references
func (s *RubyStack) scanConfigDirectory() []configEnvVar {
	var found []configEnvVar
	seen := make(map[string]bool)

	configDir := filepath.Join(s.dir, "config")

	// Check if config directory exists
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		return nil
	}

	_ = filepath.Walk(configDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			return nil
		}

		// Scan files that might contain ENV references
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".yml", ".yaml", ".erb", ".rb":
			// These file types might contain ENV references
		default:
			return nil
		}

		vars := scanConfigFileForEnvVars(path)
		relPath, _ := filepath.Rel(s.dir, path)
		if relPath == "" {
			relPath = filepath.Base(path)
		}

		for _, v := range vars {
			if !seen[v.name] && !ignoredEnvVars[v.name] {
				seen[v.name] = true
				found = append(found, configEnvVar{
					name:     v.name,
					file:     relPath,
					optional: v.optional,
				})
			}
		}

		return nil
	})

	return found
}

// scanConfigFileForEnvVars extracts ENV variable names from a config file
func scanConfigFileForEnvVars(path string) []detectedEnvVar {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	var found []detectedEnvVar
	seen := make(map[string]bool)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		for _, pattern := range configEnvPatterns {
			matches := pattern.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				if len(match) > 1 {
					varName := match[1]
					if !seen[varName] {
						seen[varName] = true
						optional := isOptionalEnvUsage(line, varName)
						found = append(found, detectedEnvVar{
							name:     varName,
							optional: optional,
						})
					}
				}
			}
		}
	}

	return found
}

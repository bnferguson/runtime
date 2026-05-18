package stackbuild

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/client/llb/imagemetaresolver"
	"miren.dev/runtime/pkg/imagerefs"
)

// goModuleEnvVars maps Go module paths to the environment variables they typically require
var goModuleEnvVars = map[string][]packageEnvVarDef{
	// Database drivers
	"github.com/lib/pq":                   {{name: "DATABASE_URL", confidence: "recommended"}},
	"github.com/jackc/pgx":                {{name: "DATABASE_URL", confidence: "recommended"}},
	"github.com/go-sql-driver/mysql":      {{name: "DATABASE_URL", confidence: "recommended"}},
	"go.mongodb.org/mongo-driver":         {{name: "MONGODB_URI", confidence: "recommended"}},
	"github.com/go-redis/redis":           {{name: "REDIS_URL", confidence: "recommended"}},
	"github.com/redis/go-redis":           {{name: "REDIS_URL", confidence: "recommended"}},
	"github.com/elastic/go-elasticsearch": {{name: "ELASTICSEARCH_URL", confidence: "recommended"}},
	"github.com/olivere/elastic":          {{name: "ELASTICSEARCH_URL", confidence: "recommended"}},

	// Cloud services
	"github.com/aws/aws-sdk-go":    {{name: "AWS_ACCESS_KEY_ID", confidence: "recommended"}, {name: "AWS_SECRET_ACCESS_KEY", confidence: "recommended"}},
	"github.com/aws/aws-sdk-go-v2": {{name: "AWS_ACCESS_KEY_ID", confidence: "recommended"}, {name: "AWS_SECRET_ACCESS_KEY", confidence: "recommended"}},

	// Third-party services
	"github.com/getsentry/sentry-go":   {{name: "SENTRY_DSN", confidence: "recommended"}},
	"github.com/stripe/stripe-go":      {{name: "STRIPE_API_KEY", confidence: "recommended"}},
	"github.com/newrelic/go-agent":     {{name: "NEW_RELIC_LICENSE_KEY", confidence: "recommended"}},
	"github.com/sendgrid/sendgrid-go":  {{name: "SENDGRID_API_KEY", confidence: "recommended"}},
	"github.com/twilio/twilio-go":      {{name: "TWILIO_ACCOUNT_SID", confidence: "recommended"}, {name: "TWILIO_AUTH_TOKEN", confidence: "recommended"}},
	"github.com/pusher/pusher-http-go": {{name: "PUSHER_APP_ID", confidence: "recommended"}, {name: "PUSHER_KEY", confidence: "recommended"}, {name: "PUSHER_SECRET", confidence: "recommended"}},
}

// goEnvPatterns are regex patterns to find env var usage in Go source code
var goEnvPatterns = []*regexp.Regexp{
	// os.Getenv("VAR")
	regexp.MustCompile(`os\.Getenv\(['"]([A-Z][A-Z0-9_]+)['"]\)`),
	// os.LookupEnv("VAR")
	regexp.MustCompile(`os\.LookupEnv\(['"]([A-Z][A-Z0-9_]+)['"]\)`),
}

// goOptionalEnvPatterns detect patterns where env var has a fallback.
//
// LookupEnv on its own is *not* optional — apps frequently use it to
// distinguish "unset" from "empty" for a hard requirement, so blanket-
// matching every LookupEnv call would silently downgrade those to
// optional. The patterns below only match when a default value or
// fallback expression is visible on the same line.
var goOptionalEnvPatterns = []*regexp.Regexp{
	// cmp.Or(os.Getenv("VAR"), default) - single-line default expression
	regexp.MustCompile(`cmp\.Or\(\s*os\.Getenv\(['"]([A-Z][A-Z0-9_]+)['"]\)\s*,`),
	// cmp.Or(os.LookupEnv("VAR"), default) — though typically wrapped
	regexp.MustCompile(`cmp\.Or\(\s*os\.LookupEnv\(['"]([A-Z][A-Z0-9_]+)['"]\)\s*,`),
}

// GoStack implements Stack for Go
type GoStack struct {
	MetaStack

	// Detection state set in Init()
	hasVendor    bool
	hasCmdDir    bool
	cmdDir       string
	goModVersion string

	// Cached go.mod content for dependency detection
	goModContent []byte

	// Detected environment variable requirements
	requiredEnvVars []EnvVarRequirement
}

func (s *GoStack) BaseDistro() string {
	return "alpine"
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

	// Cache go.mod content for later use
	s.goModContent, _ = s.readFile("go.mod")

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

	// Detect required environment variables
	s.requiredEnvVars = s.detectEnvVars()
	for _, ev := range s.requiredEnvVars {
		s.Event("env_var", ev.Name, ev.Reason)
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

	// Add app user before copying code so copyApp can set ownership
	state = s.addAppUser(state)

	state = h.applyAugmentations(state, localCtx, s.BaseDistro(), s.Augmentations(), s.SkipJSInstall())

	// Copy the application code (now owned by app user)
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

	state = s.applyOnBuild(state, opts)

	return &state, nil
}

func (s *GoStack) WebCommand() string {
	return "/bin/app"
}

// RequiredEnvVars returns the detected environment variable requirements
func (s *GoStack) RequiredEnvVars() []EnvVarRequirement {
	return s.requiredEnvVars
}

// detectEnvVars analyzes the app to find required environment variables
func (s *GoStack) detectEnvVars() []EnvVarRequirement {
	var results []EnvVarRequirement

	// 1. Scan source code first to know what env vars are actually used
	sourceVars := scanSourceFilesForEnvVars(s.dir, []string{".go"}, goEnvPatterns, goOptionalEnvPatterns)

	// 2. Framework defaults - GO_ENV is a Buffalo/framework-specific convention,
	// not a general Go convention. Surface it as optional so we don't suggest it
	// as a best practice for arbitrary Go apps. Elevate to required if the
	// source code reads it directly without a fallback.
	goEnvConfidence := "optional"
	goEnvReason := "Go environment mode (Buffalo/framework convention)"
	if elevateToRequired("GO_ENV", sourceVars) {
		goEnvConfidence = "required"
		goEnvReason = "Referenced in application code"
	}
	results = append(results, EnvVarRequirement{
		Name:         "GO_ENV",
		Source:       "go_core",
		Confidence:   goEnvConfidence,
		Reason:       goEnvReason,
		DefaultValue: "production",
	})

	// 3. Module-based inference with elevation logic
	moduleVars := s.detectModuleEnvVars()
	for _, mv := range moduleVars {
		confidence := mv.Confidence
		// Elevate to required if source code references this var
		if confidence == "recommended" && elevateToRequired(mv.Name, sourceVars) {
			confidence = "required"
		}
		if !hasEnvVar(results, mv.Name) {
			results = append(results, EnvVarRequirement{
				Name:       mv.Name,
				Source:     mv.Source,
				Confidence: confidence,
				Reason:     mv.Reason,
			})
		}
	}

	// 4. Add remaining source-detected vars not covered by modules.
	// Direct, non-default code references are hard requirements; default
	// to "required" rather than the weaker "recommended" used for
	// module-inferred guesses.
	for _, v := range sourceVars {
		if !hasEnvVar(results, v.name) {
			confidence := "required"
			reason := "Referenced in application code"
			if v.optional {
				confidence = "optional"
				reason = "Referenced in application code (has fallback)"
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

// detectModuleEnvVars analyzes go.mod to infer required env vars from dependencies
func (s *GoStack) detectModuleEnvVars() []EnvVarRequirement {
	var results []EnvVarRequirement
	seen := make(map[string]bool)

	if s.goModContent == nil {
		return results
	}

	content := string(s.goModContent)

	// Also check go.sum for more accurate dependency detection
	goSum, _ := s.readFile("go.sum")
	if goSum != nil {
		content += "\n" + string(goSum)
	}

	for modulePath, vars := range goModuleEnvVars {
		// Check if the module path appears in go.mod or go.sum
		if strings.Contains(content, modulePath) {
			for _, v := range vars {
				if !seen[v.name] {
					seen[v.name] = true
					results = append(results, EnvVarRequirement{
						Name:       v.name,
						Source:     "module",
						Confidence: v.confidence,
						Reason:     modulePath + " module detected",
					})
				}
			}
		}
	}

	return results
}

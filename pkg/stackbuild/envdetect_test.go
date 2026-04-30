package stackbuild

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRubyEnvPatterns(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "ENV with single quotes",
			input:    `config.api_key = ENV['API_KEY']`,
			expected: []string{"API_KEY"},
		},
		{
			name:     "ENV with double quotes",
			input:    `database_url = ENV["DATABASE_URL"]`,
			expected: []string{"DATABASE_URL"},
		},
		{
			name:     "ENV.fetch with single quotes",
			input:    `secret = ENV.fetch('SECRET_KEY')`,
			expected: []string{"SECRET_KEY"},
		},
		{
			name:     "ENV.fetch with double quotes and default",
			input:    `port = ENV.fetch("PORT", "3000")`,
			expected: []string{"PORT"},
		},
		{
			name:     "ENV.fetch with block default",
			input:    `value = ENV.fetch("MY_VAR") { "default" }`,
			expected: []string{"MY_VAR"},
		},
		{
			name:     "multiple ENV vars on one line",
			input:    `url = "#{ENV['PROTOCOL']}://#{ENV['HOST']}"`,
			expected: []string{"PROTOCOL", "HOST"},
		},
		{
			name:     "no matches",
			input:    `regular = "just a string"`,
			expected: nil,
		},
		{
			name:     "lowercase env var ignored",
			input:    `ENV['lowercase']`,
			expected: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var found []string
			for _, pattern := range rubyEnvPatterns {
				matches := pattern.FindAllStringSubmatch(tc.input, -1)
				for _, match := range matches {
					if len(match) > 1 {
						found = append(found, match[1])
					}
				}
			}
			assert.Equal(t, tc.expected, found)
		})
	}
}

func TestScanRubySourceForEnvVars(t *testing.T) {
	// Create a temporary directory with test files
	tmpDir, err := os.MkdirTemp("", "envdetect-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a Gemfile so RubyStack can detect
	err = os.WriteFile(filepath.Join(tmpDir, "Gemfile"), []byte("gem 'sinatra'"), 0644)
	require.NoError(t, err)

	// Create a Ruby file with various ENV usages
	rubyContent := `
class MyApp
  def initialize
    @api_key = ENV['API_KEY']
    @db_url = ENV["DATABASE_URL"]
    @secret = ENV.fetch('SECRET_TOKEN')
    @port = ENV.fetch("PORT", "3000")
  end
end
`
	err = os.WriteFile(filepath.Join(tmpDir, "app.rb"), []byte(rubyContent), 0644)
	require.NoError(t, err)

	// Create another file in a subdirectory
	err = os.MkdirAll(filepath.Join(tmpDir, "lib"), 0755)
	require.NoError(t, err)

	libContent := `
module Config
  REDIS_URL = ENV['REDIS_URL']
end
`
	err = os.WriteFile(filepath.Join(tmpDir, "lib", "config.rb"), []byte(libContent), 0644)
	require.NoError(t, err)

	// Create a vendor directory that should be skipped
	err = os.MkdirAll(filepath.Join(tmpDir, "vendor", "gems"), 0755)
	require.NoError(t, err)
	vendorContent := `IGNORED_VAR = ENV['VENDOR_VAR']`
	err = os.WriteFile(filepath.Join(tmpDir, "vendor", "gems", "something.rb"), []byte(vendorContent), 0644)
	require.NoError(t, err)

	// Create RubyStack and scan
	ms := MetaStack{dir: tmpDir}
	ms.setupResult()
	stack := &RubyStack{MetaStack: ms}

	found := stack.scanRubySourceForEnvVars()

	// Build a map for easier assertions
	foundNames := make(map[string]bool)
	for _, v := range found {
		foundNames[v.name] = true
	}

	// Should find our env vars but not vendor ones
	assert.True(t, foundNames["API_KEY"])
	assert.True(t, foundNames["DATABASE_URL"])
	assert.True(t, foundNames["SECRET_TOKEN"])
	assert.True(t, foundNames["REDIS_URL"])
	assert.False(t, foundNames["VENDOR_VAR"])
	// PORT should be filtered out as it's in ignoredEnvVars
	assert.False(t, foundNames["PORT"])
}

func TestParseEnvSampleFile(t *testing.T) {
	// Create a temporary directory with test files
	tmpDir, err := os.MkdirTemp("", "envdetect-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a .env.sample file
	sampleContent := `
# Database configuration
DATABASE_URL=postgres://localhost/myapp

# API Keys
API_KEY=your_api_key_here
STRIPE_SECRET_KEY=

# These should be ignored
# COMMENTED_VAR=ignored
RAILS_ENV=production
PORT=3000
`
	err = os.WriteFile(filepath.Join(tmpDir, ".env.sample"), []byte(sampleContent), 0644)
	require.NoError(t, err)

	found := parseEnvSampleFile(tmpDir, ".env.sample")

	assert.Contains(t, found, "DATABASE_URL")
	assert.Contains(t, found, "API_KEY")
	assert.Contains(t, found, "STRIPE_SECRET_KEY")
	// RAILS_ENV and PORT should be filtered out
	assert.NotContains(t, found, "RAILS_ENV")
	assert.NotContains(t, found, "PORT")
	// Comments should not be parsed
	assert.NotContains(t, found, "COMMENTED_VAR")
}

func TestParseEnvSampleFile_NotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "envdetect-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	found := parseEnvSampleFile(tmpDir, ".env.sample")
	assert.Nil(t, found)
}

func TestDetectGemEnvVars(t *testing.T) {
	// Create a minimal RubyStack for testing
	tmpDir, err := os.MkdirTemp("", "gem-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	ms := MetaStack{dir: tmpDir}
	ms.setupResult()
	stack := &RubyStack{MetaStack: ms}

	testCases := []struct {
		name        string
		gemfile     string
		gemfileLock string
		expected    []string
	}{
		{
			name:     "postgres gem",
			gemfile:  `gem 'pg'`,
			expected: []string{"DATABASE_URL"},
		},
		{
			name:     "redis and sidekiq gems",
			gemfile:  `gem 'redis'\ngem 'sidekiq'`,
			expected: []string{"REDIS_URL"},
		},
		{
			name:     "aws-sdk-s3 gem",
			gemfile:  `gem 'aws-sdk-s3'`,
			expected: []string{"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_REGION"},
		},
		{
			name:        "gem in lock file only",
			gemfile:     ``,
			gemfileLock: `pg (1.2.3)`,
			expected:    []string{"DATABASE_URL"},
		},
		{
			name:     "multiple services",
			gemfile:  `gem 'pg'\ngem 'stripe'\ngem 'sentry-ruby'`,
			expected: []string{"DATABASE_URL", "STRIPE_API_KEY", "SENTRY_DSN"},
		},
		{
			name:     "no relevant gems",
			gemfile:  `gem 'rails'\ngem 'puma'`,
			expected: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := stack.detectGemEnvVars([]byte(tc.gemfile), []byte(tc.gemfileLock))

			var foundNames []string
			for _, r := range result {
				foundNames = append(foundNames, r.Name)
			}

			if tc.expected == nil {
				assert.Empty(t, foundNames)
			} else {
				for _, exp := range tc.expected {
					assert.Contains(t, foundNames, exp)
				}
			}

			// Verify all results have proper metadata
			for _, r := range result {
				assert.Equal(t, "gem", r.Source)
				assert.NotEmpty(t, r.Confidence)
				assert.NotEmpty(t, r.Reason)
			}
		})
	}
}

func TestIsValidEnvVarName(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid uppercase", "DATABASE_URL", true},
		{"valid with numbers", "AWS_S3_BUCKET_1", true},
		{"valid single letter", "A", true},
		{"valid underscore start", "_PRIVATE", true},
		{"invalid lowercase", "database_url", false},
		{"invalid mixed case", "Database_URL", false},
		{"invalid starts with number", "1_VAR", false},
		{"invalid empty", "", false},
		{"invalid special chars", "VAR-NAME", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, isValidEnvVarName(tc.input))
		})
	}
}

func TestHasEnvVar(t *testing.T) {
	vars := []EnvVarRequirement{
		{Name: "DATABASE_URL"},
		{Name: "API_KEY"},
	}

	assert.True(t, hasEnvVar(vars, "DATABASE_URL"))
	assert.True(t, hasEnvVar(vars, "API_KEY"))
	assert.False(t, hasEnvVar(vars, "SECRET_KEY"))
	assert.False(t, hasEnvVar(vars, ""))
}

func TestRubyStackDetectEnvVars(t *testing.T) {
	// Create a temporary directory with a Rails app
	tmpDir, err := os.MkdirTemp("", "rails-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create Gemfile with rails and pg
	gemfile := `
source 'https://rubygems.org'

gem 'rails', '~> 7.0'
gem 'pg'
gem 'redis'
gem 'stripe'
`
	err = os.WriteFile(filepath.Join(tmpDir, "Gemfile"), []byte(gemfile), 0644)
	require.NoError(t, err)

	// Create a .env.sample
	envSample := `
DATABASE_URL=postgres://localhost/myapp
CUSTOM_VAR=value
`
	err = os.WriteFile(filepath.Join(tmpDir, ".env.sample"), []byte(envSample), 0644)
	require.NoError(t, err)

	// Create an app directory with a Ruby file using ENV
	err = os.MkdirAll(filepath.Join(tmpDir, "app", "models"), 0755)
	require.NoError(t, err)
	modelContent := `
class User
  API_ENDPOINT = ENV['CUSTOM_API_ENDPOINT']
end
`
	err = os.WriteFile(filepath.Join(tmpDir, "app", "models", "user.rb"), []byte(modelContent), 0644)
	require.NoError(t, err)

	// Create and initialize the RubyStack
	ms := MetaStack{dir: tmpDir}
	ms.setupResult()
	stack := &RubyStack{MetaStack: ms}

	// Verify detection
	assert.True(t, stack.Detect())

	// Initialize to trigger env var detection
	stack.Init(BuildOptions{})

	// Get required env vars
	envVars := stack.RequiredEnvVars()

	// Build a map for easier assertions
	varMap := make(map[string]EnvVarRequirement)
	for _, v := range envVars {
		varMap[v.Name] = v
	}

	// Rails core var
	if assert.Contains(t, varMap, "SECRET_KEY_BASE") {
		assert.Equal(t, "rails_core", varMap["SECRET_KEY_BASE"].Source)
		assert.Equal(t, "required", varMap["SECRET_KEY_BASE"].Confidence)
	}

	// Gem-based vars
	if assert.Contains(t, varMap, "DATABASE_URL") {
		assert.Equal(t, "gem", varMap["DATABASE_URL"].Source)
	}
	if assert.Contains(t, varMap, "REDIS_URL") {
		assert.Equal(t, "gem", varMap["REDIS_URL"].Source)
	}
	if assert.Contains(t, varMap, "STRIPE_API_KEY") {
		assert.Equal(t, "gem", varMap["STRIPE_API_KEY"].Source)
	}

	// Code-scanned vars: a direct, non-default reference is required.
	if assert.Contains(t, varMap, "CUSTOM_API_ENDPOINT") {
		assert.Equal(t, "code", varMap["CUSTOM_API_ENDPOINT"].Source)
		assert.Equal(t, "required", varMap["CUSTOM_API_ENDPOINT"].Confidence)
	}

	// Config file vars (CUSTOM_VAR from .env.sample, DATABASE_URL already from gem)
	if assert.Contains(t, varMap, "CUSTOM_VAR") {
		assert.Equal(t, "config", varMap["CUSTOM_VAR"].Source)
	}

	// Verify events were emitted
	events := stack.Events()
	var envVarEvents []DetectionEvent
	for _, e := range events {
		if e.Kind == "env_var" {
			envVarEvents = append(envVarEvents, e)
		}
	}
	assert.NotEmpty(t, envVarEvents, "should have emitted env_var events")
}

func TestRubyStackMasterKeyDetection(t *testing.T) {
	// Test that RAILS_MASTER_KEY is detected when credentials file exists
	tmpDir, err := os.MkdirTemp("", "rails-master-key-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create Gemfile with rails
	gemfile := `
source 'https://rubygems.org'
gem 'rails', '~> 8.0'
`
	err = os.WriteFile(filepath.Join(tmpDir, "Gemfile"), []byte(gemfile), 0644)
	require.NoError(t, err)

	// Create config directory and credentials file
	err = os.MkdirAll(filepath.Join(tmpDir, "config"), 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "config", "credentials.yml.enc"), []byte("encrypted content"), 0644)
	require.NoError(t, err)

	// Create and initialize the RubyStack
	ms := MetaStack{dir: tmpDir}
	ms.setupResult()
	stack := &RubyStack{MetaStack: ms}

	stack.Init(BuildOptions{})
	envVars := stack.RequiredEnvVars()

	// Build a map for easier assertions
	varMap := make(map[string]EnvVarRequirement)
	for _, v := range envVars {
		varMap[v.Name] = v
	}

	// Should have RAILS_MASTER_KEY
	if assert.Contains(t, varMap, "RAILS_MASTER_KEY") {
		assert.Equal(t, "rails_core", varMap["RAILS_MASTER_KEY"].Source)
		assert.Equal(t, "required", varMap["RAILS_MASTER_KEY"].Confidence)
		assert.Contains(t, varMap["RAILS_MASTER_KEY"].Reason, "decrypt")
	}

	// Should also have SECRET_KEY_BASE
	assert.Contains(t, varMap, "SECRET_KEY_BASE")
}

func TestRubyStackMasterKeyNotDetectedWithoutCredentials(t *testing.T) {
	// Test that RAILS_MASTER_KEY is NOT detected when no credentials file exists
	tmpDir, err := os.MkdirTemp("", "rails-no-creds-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create Gemfile with rails but no credentials file
	gemfile := `
source 'https://rubygems.org'
gem 'rails', '~> 7.0'
`
	err = os.WriteFile(filepath.Join(tmpDir, "Gemfile"), []byte(gemfile), 0644)
	require.NoError(t, err)

	// Create and initialize the RubyStack
	ms := MetaStack{dir: tmpDir}
	ms.setupResult()
	stack := &RubyStack{MetaStack: ms}

	stack.Init(BuildOptions{})
	envVars := stack.RequiredEnvVars()

	// Build a map for easier assertions
	varMap := make(map[string]EnvVarRequirement)
	for _, v := range envVars {
		varMap[v.Name] = v
	}

	// Should NOT have RAILS_MASTER_KEY (no credentials file)
	assert.NotContains(t, varMap, "RAILS_MASTER_KEY")

	// Should still have SECRET_KEY_BASE
	assert.Contains(t, varMap, "SECRET_KEY_BASE")
}

func TestRubyStackProductionCredentials(t *testing.T) {
	// Test detection of production-specific credentials file
	tmpDir, err := os.MkdirTemp("", "rails-prod-creds-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create Gemfile with rails
	gemfile := `gem 'rails'`
	err = os.WriteFile(filepath.Join(tmpDir, "Gemfile"), []byte(gemfile), 0644)
	require.NoError(t, err)

	// Create production credentials file (Rails 6+ multi-environment)
	err = os.MkdirAll(filepath.Join(tmpDir, "config", "credentials"), 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "config", "credentials", "production.yml.enc"), []byte("encrypted"), 0644)
	require.NoError(t, err)

	// Create and initialize the RubyStack
	ms := MetaStack{dir: tmpDir}
	ms.setupResult()
	stack := &RubyStack{MetaStack: ms}

	stack.Init(BuildOptions{})
	envVars := stack.RequiredEnvVars()

	// Build a map
	varMap := make(map[string]EnvVarRequirement)
	for _, v := range envVars {
		varMap[v.Name] = v
	}

	// Should have RAILS_MASTER_KEY for production credentials
	assert.Contains(t, varMap, "RAILS_MASTER_KEY")
}

func TestRubyStackNonRails(t *testing.T) {
	// Create a temporary directory with a non-Rails Ruby app (e.g., Sinatra)
	tmpDir, err := os.MkdirTemp("", "sinatra-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create Gemfile without rails
	gemfile := `
source 'https://rubygems.org'

gem 'sinatra'
gem 'pg'
`
	err = os.WriteFile(filepath.Join(tmpDir, "Gemfile"), []byte(gemfile), 0644)
	require.NoError(t, err)

	// Create and initialize the RubyStack
	ms := MetaStack{dir: tmpDir}
	ms.setupResult()
	stack := &RubyStack{MetaStack: ms}

	assert.True(t, stack.Detect())
	stack.Init(BuildOptions{})

	envVars := stack.RequiredEnvVars()

	// Build a map for easier assertions
	varMap := make(map[string]EnvVarRequirement)
	for _, v := range envVars {
		varMap[v.Name] = v
	}

	// Should NOT have SECRET_KEY_BASE since it's not Rails
	assert.NotContains(t, varMap, "SECRET_KEY_BASE")

	// Should still have DATABASE_URL from pg gem
	assert.Contains(t, varMap, "DATABASE_URL")
}

func TestRubyStackConfigDirectoryScanning(t *testing.T) {
	// Create a temporary directory with config files
	tmpDir, err := os.MkdirTemp("", "rails-config-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create Gemfile with rails
	gemfile := `
source 'https://rubygems.org'
gem 'rails', '~> 7.0'
`
	err = os.WriteFile(filepath.Join(tmpDir, "Gemfile"), []byte(gemfile), 0644)
	require.NoError(t, err)

	// Create config directory
	err = os.MkdirAll(filepath.Join(tmpDir, "config"), 0755)
	require.NoError(t, err)

	// Create config/database.yml with ERB ENV references
	databaseYml := `
default: &default
  adapter: postgresql
  encoding: unicode
  pool: <%= ENV.fetch("RAILS_MAX_THREADS") { 5 } %>

development:
  <<: *default
  database: myapp_development

production:
  <<: *default
  url: <%= ENV['DATABASE_URL'] %>
`
	err = os.WriteFile(filepath.Join(tmpDir, "config", "database.yml"), []byte(databaseYml), 0644)
	require.NoError(t, err)

	// Create config/secrets.yml with ENV references
	secretsYml := `
production:
  secret_key_base: <%= ENV["SECRET_KEY_BASE"] %>
  api_key: <%= ENV.fetch('EXTERNAL_API_KEY') %>
`
	err = os.WriteFile(filepath.Join(tmpDir, "config", "secrets.yml"), []byte(secretsYml), 0644)
	require.NoError(t, err)

	// Create config/application.rb with ENV reference
	applicationRb := `
module MyApp
  class Application < Rails::Application
    config.custom_setting = ENV['CUSTOM_SETTING']
  end
end
`
	err = os.WriteFile(filepath.Join(tmpDir, "config", "application.rb"), []byte(applicationRb), 0644)
	require.NoError(t, err)

	// Create and initialize the RubyStack
	ms := MetaStack{dir: tmpDir}
	ms.setupResult()
	stack := &RubyStack{MetaStack: ms}

	assert.True(t, stack.Detect())
	stack.Init(BuildOptions{})

	envVars := stack.RequiredEnvVars()

	// Build a map for easier assertions
	varMap := make(map[string]EnvVarRequirement)
	for _, v := range envVars {
		varMap[v.Name] = v
	}

	// Should find RAILS_MAX_THREADS from database.yml
	assert.Contains(t, varMap, "RAILS_MAX_THREADS")

	// DATABASE_URL is already detected from rails_core or would be from config
	// But since it's in the YAML, let's verify it's in the map
	assert.Contains(t, varMap, "DATABASE_URL")

	// Should find EXTERNAL_API_KEY from secrets.yml
	if assert.Contains(t, varMap, "EXTERNAL_API_KEY") {
		assert.Equal(t, "config", varMap["EXTERNAL_API_KEY"].Source)
		assert.Contains(t, varMap["EXTERNAL_API_KEY"].Reason, "config/secrets.yml")
	}

	// Should find CUSTOM_SETTING from application.rb (scanned as config file)
	assert.Contains(t, varMap, "CUSTOM_SETTING")
}

func TestOptionalEnvPatterns(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		varName  string
		expected bool
	}{
		{
			name:     "ENV.fetch with block is optional",
			input:    `ENV.fetch('MY_VAR') { 'default' }`,
			varName:  "MY_VAR",
			expected: true,
		},
		{
			name:     "ENV.fetch with second arg is optional",
			input:    `ENV.fetch("PORT", "3000")`,
			varName:  "PORT",
			expected: true,
		},
		{
			name:     "ENV bracket with || fallback is optional",
			input:    `ENV['TIMEOUT'] || 30`,
			varName:  "TIMEOUT",
			expected: true,
		},
		{
			name:     "ENV bracket with || string fallback is optional",
			input:    `ENV["HOST"] || "localhost"`,
			varName:  "HOST",
			expected: true,
		},
		{
			name:     "plain ENV bracket is required",
			input:    `ENV['API_KEY']`,
			varName:  "API_KEY",
			expected: false,
		},
		{
			name:     "plain ENV.fetch is required",
			input:    `ENV.fetch('SECRET')`,
			varName:  "SECRET",
			expected: false,
		},
		{
			name:     "fetch with block and spacing",
			input:    `ENV.fetch("TIMEOUT")  {  10  }`,
			varName:  "TIMEOUT",
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isOptionalEnvUsage(tc.input, tc.varName)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestOptionalEnvVarDetection(t *testing.T) {
	// Create a temporary directory with test files
	tmpDir, err := os.MkdirTemp("", "optional-env-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create Gemfile
	err = os.WriteFile(filepath.Join(tmpDir, "Gemfile"), []byte("gem 'sinatra'"), 0644)
	require.NoError(t, err)

	// Create a Ruby file with mixed required/optional ENV vars
	rubyContent := `
class App
  # Required vars (no defaults)
  @api_key = ENV['API_KEY']
  @secret = ENV.fetch('SECRET_TOKEN')

  # Optional vars (have defaults)
  @port = ENV.fetch('MY_PORT', '3000')
  @timeout = ENV.fetch('TIMEOUT') { 30 }
  @host = ENV['HOST'] || 'localhost'
end
`
	err = os.WriteFile(filepath.Join(tmpDir, "app.rb"), []byte(rubyContent), 0644)
	require.NoError(t, err)

	// Create and initialize the RubyStack
	ms := MetaStack{dir: tmpDir}
	ms.setupResult()
	stack := &RubyStack{MetaStack: ms}

	stack.Init(BuildOptions{})
	envVars := stack.RequiredEnvVars()

	// Build a map for easier assertions
	varMap := make(map[string]EnvVarRequirement)
	for _, v := range envVars {
		varMap[v.Name] = v
	}

	// Direct, non-default code references are hard requirements.
	if assert.Contains(t, varMap, "API_KEY") {
		assert.Equal(t, "required", varMap["API_KEY"].Confidence)
		assert.NotContains(t, varMap["API_KEY"].Reason, "has default")
	}
	if assert.Contains(t, varMap, "SECRET_TOKEN") {
		assert.Equal(t, "required", varMap["SECRET_TOKEN"].Confidence)
	}

	// Optional vars should have optional confidence
	if assert.Contains(t, varMap, "MY_PORT") {
		assert.Equal(t, "optional", varMap["MY_PORT"].Confidence)
		assert.Contains(t, varMap["MY_PORT"].Reason, "has default")
	}
	if assert.Contains(t, varMap, "TIMEOUT") {
		assert.Equal(t, "optional", varMap["TIMEOUT"].Confidence)
		assert.Contains(t, varMap["TIMEOUT"].Reason, "has default")
	}
}

func TestConfigEnvPatterns(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "ERB with ENV brackets single quotes",
			input:    `url: <%= ENV['DATABASE_URL'] %>`,
			expected: []string{"DATABASE_URL"},
		},
		{
			name:     "ERB with ENV brackets double quotes",
			input:    `key: <%= ENV["API_KEY"] %>`,
			expected: []string{"API_KEY"},
		},
		{
			name:     "ERB with ENV.fetch",
			input:    `pool: <%= ENV.fetch("RAILS_MAX_THREADS") { 5 } %>`,
			expected: []string{"RAILS_MAX_THREADS"},
		},
		{
			name:     "ERB with ENV.fetch and default",
			input:    `port: <%= ENV.fetch('PORT', '3000') %>`,
			expected: []string{"PORT"},
		},
		{
			name:     "Multiple ENV vars",
			input:    `<%= ENV['HOST'] %>:<%= ENV['PORT'] %>`,
			expected: []string{"HOST", "PORT"},
		},
		{
			name:     "Plain Ruby in config",
			input:    `config.key = ENV['SOME_KEY']`,
			expected: []string{"SOME_KEY"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			seen := make(map[string]bool)
			var found []string
			for _, pattern := range configEnvPatterns {
				matches := pattern.FindAllStringSubmatch(tc.input, -1)
				for _, match := range matches {
					if len(match) > 1 && !seen[match[1]] {
						seen[match[1]] = true
						found = append(found, match[1])
					}
				}
			}
			assert.Equal(t, tc.expected, found)
		})
	}
}

func TestPythonEnvPatterns(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "os.getenv with single quotes",
			input:    `api_key = os.getenv('API_KEY')`,
			expected: []string{"API_KEY"},
		},
		{
			name:     "os.getenv with double quotes",
			input:    `db_url = os.getenv("DATABASE_URL")`,
			expected: []string{"DATABASE_URL"},
		},
		{
			name:     "os.environ bracket access single quotes",
			input:    `secret = os.environ['SECRET_KEY']`,
			expected: []string{"SECRET_KEY"},
		},
		{
			name:     "os.environ bracket access double quotes",
			input:    `token = os.environ["AUTH_TOKEN"]`,
			expected: []string{"AUTH_TOKEN"},
		},
		{
			name:     "os.environ.get with single quotes",
			input:    `redis = os.environ.get('REDIS_URL')`,
			expected: []string{"REDIS_URL"},
		},
		{
			name:     "os.environ.get with double quotes",
			input:    `host = os.environ.get("DB_HOST")`,
			expected: []string{"DB_HOST"},
		},
		{
			name:     "multiple env vars on one line",
			input:    `conn = f"{os.getenv('DB_HOST')}:{os.getenv('DB_PORT')}"`,
			expected: []string{"DB_HOST", "DB_PORT"},
		},
		{
			name:     "no matches",
			input:    `regular = "just a string"`,
			expected: nil,
		},
		{
			name:     "lowercase env var ignored",
			input:    `os.getenv('lowercase')`,
			expected: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var found []string
			for _, pattern := range pythonEnvPatterns {
				matches := pattern.FindAllStringSubmatch(tc.input, -1)
				for _, match := range matches {
					if len(match) > 1 {
						found = append(found, match[1])
					}
				}
			}
			assert.Equal(t, tc.expected, found)
		})
	}
}

func TestNodeEnvPatterns(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "process.env.VAR",
			input:    `const apiKey = process.env.API_KEY`,
			expected: []string{"API_KEY"},
		},
		{
			name:     "process.env bracket single quotes",
			input:    `const dbUrl = process.env['DATABASE_URL']`,
			expected: []string{"DATABASE_URL"},
		},
		{
			name:     "process.env bracket double quotes",
			input:    `const secret = process.env["SECRET_KEY"]`,
			expected: []string{"SECRET_KEY"},
		},
		{
			name:     "multiple env vars",
			input:    `const url = process.env.DB_HOST + ":" + process.env.DB_PORT`,
			expected: []string{"DB_HOST", "DB_PORT"},
		},
		{
			name:     "no matches",
			input:    `const regular = "just a string"`,
			expected: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var found []string
			for _, pattern := range nodeEnvPatterns {
				matches := pattern.FindAllStringSubmatch(tc.input, -1)
				for _, match := range matches {
					if len(match) > 1 {
						found = append(found, match[1])
					}
				}
			}
			assert.Equal(t, tc.expected, found)
		})
	}
}

func TestGoEnvPatterns(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "os.Getenv with double quotes",
			input:    `apiKey := os.Getenv("API_KEY")`,
			expected: []string{"API_KEY"},
		},
		{
			name:     "os.LookupEnv with double quotes",
			input:    `dbUrl, ok := os.LookupEnv("DATABASE_URL")`,
			expected: []string{"DATABASE_URL"},
		},
		{
			name:     "multiple env vars",
			input:    `host := os.Getenv("DB_HOST") + ":" + os.Getenv("DB_PORT")`,
			expected: []string{"DB_HOST", "DB_PORT"},
		},
		{
			name:     "no matches",
			input:    `regular := "just a string"`,
			expected: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var found []string
			for _, pattern := range goEnvPatterns {
				matches := pattern.FindAllStringSubmatch(tc.input, -1)
				for _, match := range matches {
					if len(match) > 1 {
						found = append(found, match[1])
					}
				}
			}
			assert.Equal(t, tc.expected, found)
		})
	}
}

func TestRustEnvPatterns(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "std::env::var",
			input:    `let api_key = std::env::var("API_KEY").unwrap();`,
			expected: []string{"API_KEY"},
		},
		{
			name:     "env::var with use",
			input:    `let db_url = env::var("DATABASE_URL")?;`,
			expected: []string{"DATABASE_URL"},
		},
		{
			name:     "std::env::var_os",
			input:    `let path = std::env::var_os("CUSTOM_PATH");`,
			expected: []string{"CUSTOM_PATH"},
		},
		{
			name:     "env! macro",
			input:    `const VERSION: &str = env!("CARGO_PKG_VERSION");`,
			expected: []string{"CARGO_PKG_VERSION"},
		},
		{
			name:     "option_env! macro",
			input:    `const DEBUG: Option<&str> = option_env!("DEBUG_MODE");`,
			expected: []string{"DEBUG_MODE"},
		},
		{
			name:     "multiple env vars",
			input:    `let url = format!("{}:{}", env::var("HOST")?, env::var("PORT")?);`,
			expected: []string{"HOST", "PORT"},
		},
		{
			name:     "no matches",
			input:    `let regular = "just a string";`,
			expected: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Use a map to deduplicate matches (some patterns overlap)
			seen := make(map[string]bool)
			var found []string
			for _, pattern := range rustEnvPatterns {
				matches := pattern.FindAllStringSubmatch(tc.input, -1)
				for _, match := range matches {
					if len(match) > 1 && !seen[match[1]] {
						seen[match[1]] = true
						found = append(found, match[1])
					}
				}
			}
			assert.Equal(t, tc.expected, found)
		})
	}
}

func TestScanSourceFilesForEnvVars_AllFileTypes(t *testing.T) {
	// Create a temporary directory with various source file types
	tmpDir, err := os.MkdirTemp("", "source-scan-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create .js file
	jsContent := `const apiKey = process.env.JS_API_KEY;`
	err = os.WriteFile(filepath.Join(tmpDir, "app.js"), []byte(jsContent), 0644)
	require.NoError(t, err)

	// Create .ts file
	tsContent := `const dbUrl: string = process.env.TS_DATABASE_URL || '';`
	err = os.WriteFile(filepath.Join(tmpDir, "config.ts"), []byte(tsContent), 0644)
	require.NoError(t, err)

	// Create .jsx file
	jsxContent := `
function App() {
  const apiUrl = process.env.JSX_API_URL;
  return <div>{apiUrl}</div>;
}
`
	err = os.WriteFile(filepath.Join(tmpDir, "App.jsx"), []byte(jsxContent), 0644)
	require.NoError(t, err)

	// Create .tsx file
	tsxContent := `
interface Props {}
const Component: React.FC<Props> = () => {
  const secret = process.env.TSX_SECRET_KEY;
  return <span>{secret}</span>;
};
`
	err = os.WriteFile(filepath.Join(tmpDir, "Component.tsx"), []byte(tsxContent), 0644)
	require.NoError(t, err)

	// Create .mjs file
	mjsContent := `export const config = { key: process.env.MJS_CONFIG_KEY };`
	err = os.WriteFile(filepath.Join(tmpDir, "module.mjs"), []byte(mjsContent), 0644)
	require.NoError(t, err)

	// Create .cjs file
	cjsContent := `module.exports = { token: process.env.CJS_AUTH_TOKEN };`
	err = os.WriteFile(filepath.Join(tmpDir, "common.cjs"), []byte(cjsContent), 0644)
	require.NoError(t, err)

	// Scan all Node/Bun file types
	found := scanSourceFilesForEnvVars(tmpDir, []string{".js", ".ts", ".jsx", ".tsx", ".mjs", ".cjs"}, nodeEnvPatterns, nodeOptionalEnvPatterns)

	// Build a map for easier assertions
	foundNames := make(map[string]bool)
	for _, v := range found {
		foundNames[v.name] = true
	}

	// Verify all file types were scanned
	assert.True(t, foundNames["JS_API_KEY"], ".js file should be scanned")
	assert.True(t, foundNames["TS_DATABASE_URL"], ".ts file should be scanned")
	assert.True(t, foundNames["JSX_API_URL"], ".jsx file should be scanned")
	assert.True(t, foundNames["TSX_SECRET_KEY"], ".tsx file should be scanned")
	assert.True(t, foundNames["MJS_CONFIG_KEY"], ".mjs file should be scanned")
	assert.True(t, foundNames["CJS_AUTH_TOKEN"], ".cjs file should be scanned")
}

func TestScanSourceFilesForEnvVars_Python(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "python-scan-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create Python files
	mainContent := `
import os

API_KEY = os.getenv('PY_API_KEY')
DB_URL = os.environ['PY_DATABASE_URL']
REDIS = os.environ.get('PY_REDIS_URL')
`
	err = os.WriteFile(filepath.Join(tmpDir, "main.py"), []byte(mainContent), 0644)
	require.NoError(t, err)

	// Create a subdirectory
	err = os.MkdirAll(filepath.Join(tmpDir, "lib"), 0755)
	require.NoError(t, err)

	libContent := `
import os
SECRET = os.environ.get('PY_SECRET_TOKEN')
`
	err = os.WriteFile(filepath.Join(tmpDir, "lib", "config.py"), []byte(libContent), 0644)
	require.NoError(t, err)

	// Create a __pycache__ directory that should be skipped
	err = os.MkdirAll(filepath.Join(tmpDir, "__pycache__"), 0755)
	require.NoError(t, err)
	cacheContent := `IGNORED = os.getenv('SHOULD_BE_IGNORED')`
	err = os.WriteFile(filepath.Join(tmpDir, "__pycache__", "cached.py"), []byte(cacheContent), 0644)
	require.NoError(t, err)

	found := scanSourceFilesForEnvVars(tmpDir, []string{".py"}, pythonEnvPatterns, pythonOptionalEnvPatterns)

	foundNames := make(map[string]bool)
	for _, v := range found {
		foundNames[v.name] = true
	}

	assert.True(t, foundNames["PY_API_KEY"])
	assert.True(t, foundNames["PY_DATABASE_URL"])
	assert.True(t, foundNames["PY_REDIS_URL"])
	assert.True(t, foundNames["PY_SECRET_TOKEN"])
	assert.False(t, foundNames["SHOULD_BE_IGNORED"], "__pycache__ should be skipped")
}

// TestBunEnvPatterns verifies that the Bun-specific Bun.env accessor is
// recognized alongside process.env so the scanner doesn't miss vars in
// idiomatic Bun code.
func TestBunEnvPatterns(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bun-scan-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Mix of process.env and Bun.env, including bracket and nullish forms.
	// (PORT is in ignoredEnvVars, so it doesn't appear in results.)
	jsContent := `
const sentryKey = process.env.MY_SENTRY_KEY;
const apiKey = Bun.env.BUN_API_KEY;
const dbUrl = Bun.env["BUN_DATABASE_URL"];
const optional = Bun.env.BUN_LOG_LEVEL ?? "info";
const fallback = Bun.env['BUN_REGION'] || "us-east-1";
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "index.ts"), []byte(jsContent), 0644))

	envPatterns := append(append([]*regexp.Regexp{}, nodeEnvPatterns...), bunEnvPatterns...)
	optionalPatterns := append(append([]*regexp.Regexp{}, nodeOptionalEnvPatterns...), bunOptionalEnvPatterns...)
	found := scanSourceFilesForEnvVars(tmpDir, []string{".ts"}, envPatterns, optionalPatterns)

	byName := make(map[string]detectedEnvVar)
	for _, v := range found {
		byName[v.name] = v
	}

	assert.Contains(t, byName, "MY_SENTRY_KEY")
	assert.Contains(t, byName, "BUN_API_KEY")
	assert.Contains(t, byName, "BUN_DATABASE_URL")
	if assert.Contains(t, byName, "BUN_LOG_LEVEL") {
		assert.True(t, byName["BUN_LOG_LEVEL"].optional, "Bun.env.X ?? default should be optional")
	}
	if assert.Contains(t, byName, "BUN_REGION") {
		assert.True(t, byName["BUN_REGION"].optional, "Bun.env['X'] || default should be optional")
	}
}

func TestScanSourceFilesForEnvVars_Go(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "go-scan-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	mainContent := `
package main

import "os"

func main() {
	apiKey := os.Getenv("GO_API_KEY")
	dbUrl, _ := os.LookupEnv("GO_DATABASE_URL")
	_ = apiKey + dbUrl
}
`
	err = os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainContent), 0644)
	require.NoError(t, err)

	// Create cmd subdirectory
	err = os.MkdirAll(filepath.Join(tmpDir, "cmd"), 0755)
	require.NoError(t, err)

	cmdContent := `
package main

import "os"

var secret = os.Getenv("GO_SECRET_TOKEN")
`
	err = os.WriteFile(filepath.Join(tmpDir, "cmd", "server.go"), []byte(cmdContent), 0644)
	require.NoError(t, err)

	// Create vendor directory that should be skipped
	err = os.MkdirAll(filepath.Join(tmpDir, "vendor", "pkg"), 0755)
	require.NoError(t, err)
	vendorContent := `var ignored = os.Getenv("GO_VENDOR_VAR")`
	err = os.WriteFile(filepath.Join(tmpDir, "vendor", "pkg", "dep.go"), []byte(vendorContent), 0644)
	require.NoError(t, err)

	found := scanSourceFilesForEnvVars(tmpDir, []string{".go"}, goEnvPatterns, goOptionalEnvPatterns)

	foundNames := make(map[string]bool)
	for _, v := range found {
		foundNames[v.name] = true
	}

	assert.True(t, foundNames["GO_API_KEY"])
	assert.True(t, foundNames["GO_DATABASE_URL"])
	assert.True(t, foundNames["GO_SECRET_TOKEN"])
	assert.False(t, foundNames["GO_VENDOR_VAR"], "vendor should be skipped")
}

func TestScanSourceFilesForEnvVars_Rust(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rust-scan-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	mainContent := `
use std::env;

fn main() {
    let api_key = env::var("RUST_API_KEY").unwrap();
    let db_url = std::env::var("RUST_DATABASE_URL").expect("DB URL required");
    println!("{} {}", api_key, db_url);
}
`
	err = os.WriteFile(filepath.Join(tmpDir, "main.rs"), []byte(mainContent), 0644)
	require.NoError(t, err)

	// Create src subdirectory
	err = os.MkdirAll(filepath.Join(tmpDir, "src"), 0755)
	require.NoError(t, err)

	libContent := `
use std::env;

pub fn get_secret() -> String {
    env::var("RUST_SECRET_TOKEN").unwrap_or_default()
}
`
	err = os.WriteFile(filepath.Join(tmpDir, "src", "lib.rs"), []byte(libContent), 0644)
	require.NoError(t, err)

	// Create target directory that should be skipped
	err = os.MkdirAll(filepath.Join(tmpDir, "target", "debug"), 0755)
	require.NoError(t, err)
	targetContent := `let ignored = env::var("RUST_TARGET_VAR");`
	err = os.WriteFile(filepath.Join(tmpDir, "target", "debug", "build.rs"), []byte(targetContent), 0644)
	require.NoError(t, err)

	found := scanSourceFilesForEnvVars(tmpDir, []string{".rs"}, rustEnvPatterns, rustOptionalEnvPatterns)

	foundNames := make(map[string]bool)
	for _, v := range found {
		foundNames[v.name] = true
	}

	assert.True(t, foundNames["RUST_API_KEY"])
	assert.True(t, foundNames["RUST_DATABASE_URL"])
	assert.True(t, foundNames["RUST_SECRET_TOKEN"])
	assert.False(t, foundNames["RUST_TARGET_VAR"], "target should be skipped")
}

func TestElevateToRequired(t *testing.T) {
	sourceVars := []detectedEnvVar{
		{name: "DATABASE_URL", optional: false},
		{name: "OPTIONAL_VAR", optional: true},
		{name: "API_KEY", optional: false},
	}

	// Non-optional vars should elevate
	assert.True(t, elevateToRequired("DATABASE_URL", sourceVars))
	assert.True(t, elevateToRequired("API_KEY", sourceVars))

	// Optional vars should not elevate
	assert.False(t, elevateToRequired("OPTIONAL_VAR", sourceVars))

	// Vars not in source should not elevate
	assert.False(t, elevateToRequired("NOT_IN_SOURCE", sourceVars))
}

func TestScanTimeout(t *testing.T) {
	// Verify the scan timeout is configured to 10 seconds
	assert.Equal(t, 10*time.Second, scanTimeout, "scan timeout should be 10 seconds")

	// Verify errScanTimeout is defined
	assert.NotNil(t, errScanTimeout)
	assert.Equal(t, "scan timeout exceeded", errScanTimeout.Error())
}

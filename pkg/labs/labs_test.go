package labs

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestDefaultFeatureValues(t *testing.T) {
	Reset()

	// All features should be disabled by default
	if GlobalRouter() {
		t.Error("GlobalRouter should be false by default")
	}
	if DistributedRunners() {
		t.Error("DistributedRunners should be false by default")
	}
	if UserSubdomains() {
		t.Error("UserSubdomains should be false by default")
	}
}

func TestEnableFeatureViaInit(t *testing.T) {
	Reset()

	Init(nil, []string{"globalrouter"})

	if !GlobalRouter() {
		t.Error("GlobalRouter should be enabled after Init with 'globalrouter'")
	}
	if DistributedRunners() {
		t.Error("DistributedRunners should still be false")
	}
	if UserSubdomains() {
		t.Error("UserSubdomains should still be false")
	}
}

func TestEnableMultipleFeatures(t *testing.T) {
	Reset()

	Init(nil, []string{"globalrouter", "usersubdomains"})

	if !GlobalRouter() {
		t.Error("GlobalRouter should be enabled")
	}
	if DistributedRunners() {
		t.Error("DistributedRunners should still be false")
	}
	if !UserSubdomains() {
		t.Error("UserSubdomains should be enabled")
	}
}

func TestDisableFeatureWithPrefix(t *testing.T) {
	Reset()

	// Enable first, then disable
	Init(nil, []string{"globalrouter", "-globalrouter"})

	if GlobalRouter() {
		t.Error("GlobalRouter should be disabled after '-globalrouter'")
	}
}

func TestCaseInsensitiveFeatureNames(t *testing.T) {
	Reset()

	Init(nil, []string{"GlobalRouter", "USERSUBDOMAINS"})

	if !GlobalRouter() {
		t.Error("GlobalRouter should be enabled (case-insensitive)")
	}
	if !UserSubdomains() {
		t.Error("UserSubdomains should be enabled (case-insensitive)")
	}
}

func TestUnknownFeatureLogsWarning(t *testing.T) {
	Reset()

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	Init(logger, []string{"unknownfeature"})

	logOutput := buf.String()
	if !strings.Contains(logOutput, "unknown labs feature flag") {
		t.Errorf("Expected warning about unknown feature, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "unknownfeature") {
		t.Errorf("Expected warning to contain the unknown feature name, got: %s", logOutput)
	}
}

func TestIsEnabledFunction(t *testing.T) {
	Reset()

	Init(nil, []string{"globalrouter"})

	if !IsEnabled("globalrouter") {
		t.Error("IsEnabled('globalrouter') should return true")
	}
	if !IsEnabled("GlobalRouter") {
		t.Error("IsEnabled('GlobalRouter') should return true (case-insensitive)")
	}
	if IsEnabled("distributedrunners") {
		t.Error("IsEnabled('distributedrunners') should return false")
	}
	if IsEnabled("unknownfeature") {
		t.Error("IsEnabled('unknownfeature') should return false")
	}
}

func TestAllFeatures(t *testing.T) {
	features := AllFeatures()

	if len(features) != 3 {
		t.Errorf("Expected 3 features, got %d", len(features))
	}

	expected := []string{"globalrouter", "distributedrunners", "usersubdomains"}
	for _, name := range expected {
		found := false
		for _, f := range features {
			if f == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected feature %q not found in AllFeatures()", name)
		}
	}
}

func TestFeatureDescriptions(t *testing.T) {
	descriptions := FeatureDescriptions()

	if len(descriptions) != 3 {
		t.Errorf("Expected 3 descriptions, got %d", len(descriptions))
	}

	if descriptions[FeatureGlobalRouter] == "" {
		t.Error("GlobalRouter description should not be empty")
	}
	if descriptions[FeatureDistributedRunners] == "" {
		t.Error("DistributedRunners description should not be empty")
	}
	if descriptions[FeatureUserSubdomains] == "" {
		t.Error("UserSubdomains description should not be empty")
	}
}

func TestResetFunction(t *testing.T) {
	Init(nil, []string{"globalrouter", "distributedrunners", "usersubdomains"})

	if !GlobalRouter() || !DistributedRunners() || !UserSubdomains() {
		t.Error("All features should be enabled before reset")
	}

	Reset()

	if GlobalRouter() || DistributedRunners() || UserSubdomains() {
		t.Error("All features should be back to defaults (false) after reset")
	}
}

func TestEmptyAndWhitespaceFlags(t *testing.T) {
	Reset()

	Init(nil, []string{"", "  ", "globalrouter", "  ", ""})

	if !GlobalRouter() {
		t.Error("GlobalRouter should be enabled despite empty/whitespace flags")
	}
}

func TestInitLogsEnabledFeatures(t *testing.T) {
	Reset()

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	Init(logger, []string{"globalrouter"})

	logOutput := buf.String()
	if !strings.Contains(logOutput, "labs feature configured") {
		t.Errorf("Expected info log about configured feature, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "globalrouter") {
		t.Errorf("Expected log to contain feature name, got: %s", logOutput)
	}
}

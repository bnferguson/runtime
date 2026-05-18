package stackbuild

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	}
}

func TestDetectAugmentations_NpmFromPackageLock(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"package.json":      `{"name":"frontend"}`,
		"package-lock.json": `{}`,
	})

	augs, skipInstall, events := DetectAugmentations(dir, "python")
	require.False(t, skipInstall)
	require.Equal(t, []Augmentation{AugNpm}, augs)
	require.Len(t, events, 1)
	require.Equal(t, "augmentation", events[0].Kind)
	require.Equal(t, "npm", events[0].Name)
	require.Contains(t, events[0].Message, "package-lock.json")
}

func TestDetectAugmentations_NpmFromPackageJsonOnly(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"package.json": `{"name":"frontend"}`,
	})

	augs, skipInstall, events := DetectAugmentations(dir, "ruby")
	require.False(t, skipInstall)
	require.Equal(t, []Augmentation{AugNpm}, augs)
	require.Len(t, events, 1)
	require.Contains(t, events[0].Message, "package.json")
}

func TestDetectAugmentations_YarnFromLock(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"package.json": `{"name":"frontend"}`,
		"yarn.lock":    ``,
	})

	augs, skipInstall, events := DetectAugmentations(dir, "ruby")
	require.False(t, skipInstall)
	require.Equal(t, []Augmentation{AugYarn}, augs)
	require.Len(t, events, 1)
	require.Equal(t, "yarn", events[0].Name)
	require.Contains(t, events[0].Message, "yarn.lock")
}

func TestDetectAugmentations_YarnWinsOverPackageLock(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"package.json":      `{"name":"frontend"}`,
		"yarn.lock":         ``,
		"package-lock.json": `{}`,
	})

	augs, _, _ := DetectAugmentations(dir, "ruby")
	require.Equal(t, []Augmentation{AugYarn}, augs, "yarn.lock should win when both lockfiles exist")
}

func TestDetectAugmentations_BunFromLock(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"package.json": `{"name":"frontend"}`,
		"bun.lock":     ``,
	})

	augs, _, _ := DetectAugmentations(dir, "ruby")
	require.Equal(t, []Augmentation{AugBun}, augs, "bun owns install when bun.lock is present; npm should not also run")
}

func TestDetectAugmentations_BunWinsOverPackageLock(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"package.json":      `{"name":"frontend"}`,
		"package-lock.json": `{}`,
		"bun.lockb":         ``,
	})

	augs, _, _ := DetectAugmentations(dir, "ruby")
	require.Equal(t, []Augmentation{AugBun}, augs, "bun should win even when package-lock.json is present")
}

func TestDetectAugmentations_BunFromLockb(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"bun.lockb": ``,
	})

	augs, skipInstall, events := DetectAugmentations(dir, "go")
	require.False(t, skipInstall)
	require.Equal(t, []Augmentation{AugBun}, augs)
	require.Len(t, events, 1)
	require.Equal(t, "bun", events[0].Name)
}

func TestDetectAugmentations_SkipForNode(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"package.json":      `{"name":"app"}`,
		"package-lock.json": `{}`,
		"bun.lock":          ``,
	})

	augs, skipInstall, events := DetectAugmentations(dir, "node")
	require.Nil(t, augs)
	require.False(t, skipInstall)
	require.Nil(t, events)
}

func TestDetectAugmentations_SkipForBun(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"package.json": `{"name":"app"}`,
		"bun.lock":     ``,
	})

	augs, _, events := DetectAugmentations(dir, "bun")
	require.Nil(t, augs)
	require.Nil(t, events)
}

func TestDetectAugmentations_NoneWithoutLockfiles(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"go.mod": `module example.com/app`,
	})

	augs, _, _ := DetectAugmentations(dir, "go")
	require.Nil(t, augs)
}

func TestDetectAugmentations_SkipInstallWhenNodeModulesPresent(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"package.json":              `{"name":"frontend"}`,
		"package-lock.json":         `{}`,
		"node_modules/.keep":        ``,
		"node_modules/foo/index.js": ``,
	})

	augs, skipInstall, events := DetectAugmentations(dir, "ruby")
	require.Equal(t, []Augmentation{AugNpm}, augs, "tool should still be installed")
	require.True(t, skipInstall, "JS install should be skipped when node_modules is present")

	var sawSkipEvent bool
	for _, e := range events {
		if e.Name == "node_modules" {
			sawSkipEvent = true
		}
	}
	require.True(t, sawSkipEvent, "expected node_modules skip event")
}

func TestDetectAugmentations_NoSkipWhenNoLockfiles(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"go.mod":             `module example.com/app`,
		"node_modules/.keep": ``,
	})

	augs, skipInstall, _ := DetectAugmentations(dir, "go")
	require.Nil(t, augs)
	require.False(t, skipInstall, "skipInstall is only relevant when there's something to skip")
}

func TestDetectStack_AttachesAugmentationsToPython(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"requirements.txt":  `flask==2.0`,
		"package.json":      `{"name":"frontend"}`,
		"package-lock.json": `{}`,
	})

	stack, err := DetectStack(dir, BuildOptions{Name: "test"})
	require.NoError(t, err)
	require.Equal(t, "python", stack.Name())

	ms := stack.metaStack()
	require.Equal(t, []Augmentation{AugNpm}, ms.Augmentations())

	hasAugmentationEvent := false
	for _, e := range stack.Events() {
		if e.Kind == "augmentation" && e.Name == "npm" {
			hasAugmentationEvent = true
			break
		}
	}
	require.True(t, hasAugmentationEvent, "expected augmentation event in stack Events()")
}

func TestDetectStack_NoAugmentationForNode(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"package.json":      `{"name":"app"}`,
		"package-lock.json": `{}`,
	})

	stack, err := DetectStack(dir, BuildOptions{Name: "test"})
	require.NoError(t, err)
	require.Equal(t, "node", stack.Name())

	ms := stack.metaStack()
	require.Nil(t, ms.Augmentations())
}

func TestDetectStack_SkipInstallWhenNodeModulesPresent(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"requirements.txt":   `flask==2.0`,
		"package.json":       `{"name":"frontend"}`,
		"node_modules/.keep": ``,
	})

	stack, err := DetectStack(dir, BuildOptions{Name: "test"})
	require.NoError(t, err)
	require.Equal(t, "python", stack.Name())

	ms := stack.metaStack()
	require.Equal(t, []Augmentation{AugNpm}, ms.Augmentations())
	require.True(t, ms.SkipJSInstall())
}

func TestDetectStack_AttachesAugmentationsToGo(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"go.mod":       "module example.com/app\n\ngo 1.23\n",
		"package.json": `{"name":"frontend"}`,
	})

	stack, err := DetectStack(dir, BuildOptions{Name: "test"})
	require.NoError(t, err)
	require.Equal(t, "go", stack.Name())
	require.Equal(t, "alpine", stack.BaseDistro())

	ms := stack.metaStack()
	require.Equal(t, []Augmentation{AugNpm}, ms.Augmentations())
}

func TestBaseDistros(t *testing.T) {
	cases := []struct {
		stack  Stack
		distro string
	}{
		{&RubyStack{}, "debian"},
		{&PythonStack{}, "debian"},
		{&NodeStack{}, "debian"},
		{&BunStack{}, "debian"},
		{&RustStack{}, "debian"},
		{&GoStack{}, "alpine"},
	}
	for _, c := range cases {
		require.Equal(t, c.distro, c.stack.BaseDistro(), "%s base distro", c.stack.Name())
	}
}

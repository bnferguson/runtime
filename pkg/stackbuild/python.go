package stackbuild

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/moby/buildkit/client/llb"
	"github.com/pelletier/go-toml/v2"
	"miren.dev/runtime/pkg/imagerefs"
)

// pythonPackageManager represents the detected package manager
type pythonPackageManager string

const (
	pythonPkgPip    pythonPackageManager = "pip"
	pythonPkgPipenv pythonPackageManager = "pipenv"
	pythonPkgPoetry pythonPackageManager = "poetry"
	pythonPkgUv     pythonPackageManager = "uv"
)

// PythonStack implements Stack for Python
type PythonStack struct {
	MetaStack

	// Detection state set in Init()
	packageManager    pythonPackageManager
	hasDjango         bool
	hasFlask          bool
	hasFastAPI        bool
	hasGunicorn       bool
	hasUvicorn        bool
	hasManagePy       bool
	wsgiModule        string
	asgiModule        string
	fastapiEntrypoint string // from [tool.fastapi] entrypoint in pyproject.toml

	// Cached uv.lock packages for accurate detection
	uvPackages map[string]bool
}

func (s *PythonStack) Name() string {
	return "python"
}

func (s *PythonStack) Detect() bool {
	if s.hasFile("Pipfile") {
		s.packageManager = pythonPkgPipenv
		s.Event("file", "Pipfile", "Found Pipfile (pipenv)")
		return true
	}

	// Check for uv.lock before pyproject.toml since uv also uses pyproject.toml
	if s.hasFile("uv.lock") {
		s.packageManager = pythonPkgUv
		s.Event("file", "uv.lock", "Found uv.lock (uv)")
		return true
	}

	if s.hasFile("pyproject.toml") {
		// Check if this is actually a Poetry project by looking for [tool.poetry]
		if data, err := s.readFile("pyproject.toml"); err == nil {
			if strings.Contains(string(data), "[tool.poetry]") {
				s.packageManager = pythonPkgPoetry
				s.Event("file", "pyproject.toml", "Found pyproject.toml (poetry)")
				return true
			}
		}
		// pyproject.toml without poetry - use pip
		s.packageManager = pythonPkgPip
		s.Event("file", "pyproject.toml", "Found pyproject.toml (pip)")
		return true
	}

	if s.hasFile("requirements.txt") {
		s.packageManager = pythonPkgPip
		s.Event("file", "requirements.txt", "Found requirements.txt (pip)")
		return true
	}

	return false
}

func (s *PythonStack) Init(opts BuildOptions) {
	s.SetCwd("/app")

	// Detect frameworks and libraries, store state for later use
	s.hasDjango = s.detectPackage("django")
	if s.hasDjango {
		s.Event("framework", "django", "Django framework detected")
	}

	s.hasFlask = s.detectPackage("flask")
	if s.hasFlask {
		s.Event("framework", "flask", "Flask framework detected")
	}

	s.hasFastAPI = s.detectPackage("fastapi")
	if s.hasFastAPI {
		s.Event("framework", "fastapi", "FastAPI framework detected")
	}

	s.hasGunicorn = s.detectPackage("gunicorn")
	if s.hasGunicorn {
		s.Event("package", "gunicorn", "Gunicorn WSGI server detected")
	}

	s.hasUvicorn = s.detectPackage("uvicorn")
	if s.hasUvicorn {
		s.Event("package", "uvicorn", "Uvicorn ASGI server detected")
	}

	s.hasManagePy = s.hasFile("manage.py")
	if s.hasManagePy {
		s.Event("file", "manage.py", "Django manage.py detected")
	}

	// Pre-compute WSGI/ASGI modules
	s.wsgiModule = s.findWSGIModule()
	s.asgiModule = s.findASGIModule()

	// Check for FastAPI entrypoint in pyproject.toml [tool.fastapi]
	s.fastapiEntrypoint = s.findFastAPIEntrypoint()
	if s.fastapiEntrypoint != "" {
		s.Event("config", "fastapi", "FastAPI entrypoint: "+s.fastapiEntrypoint)
	}
}

func (s *PythonStack) detectPackage(pkg string) bool {
	// Normalize package name for comparison
	pkgLower := strings.ToLower(pkg)

	// Check uv.lock first using parsed TOML for accurate detection
	if uvPkgs := s.parseUvLock(); uvPkgs != nil {
		if uvPkgs[pkgLower] {
			return true
		}
	}

	// Check requirements.txt
	if data, err := s.readFile("requirements.txt"); err == nil {
		if strings.Contains(strings.ToLower(string(data)), pkgLower) {
			return true
		}
	}

	// Check Pipfile and Pipfile.lock
	if data, err := s.readFile("Pipfile"); err == nil {
		if strings.Contains(strings.ToLower(string(data)), pkgLower) {
			return true
		}
	}
	if data, err := s.readFile("Pipfile.lock"); err == nil {
		if strings.Contains(strings.ToLower(string(data)), pkgLower) {
			return true
		}
	}

	// Check pyproject.toml
	if data, err := s.readFile("pyproject.toml"); err == nil {
		if strings.Contains(strings.ToLower(string(data)), pkgLower) {
			return true
		}
	}

	return false
}

func (s *PythonStack) GenerateLLB(dir string, opts BuildOptions) (*llb.State, error) {
	// Set up local context with the directory
	localCtx := llb.Local("context",
		llb.SharedKeyHint(dir),
		llb.ExcludePatterns([]string{".git"}),
		llb.FollowPaths([]string{"."}),
		llb.WithCustomName("application code"),
	)

	version := "3.11"
	if opts.Version != "" {
		version = opts.Version
	}

	base := llb.Image(imagerefs.GetPythonImage(version))

	base = s.addAppUser(base)

	// Create pip cache mount
	pipCache := llb.Scratch().File(
		llb.Mkdir("/pip-cache", 0777, llb.WithParents(true)),
	)
	userPipCache := llb.Scratch().File(
		llb.Mkdir("/pip-cache", 0777, llb.WithParents(true)),
	)

	var state llb.State
	state = base

	// Handle different dependency management systems
	switch s.packageManager {
	case pythonPkgPip:
		// Copy only requirements.txt first
		pipState := state.File(llb.Copy(localCtx, "/", "/app", &llb.CopyInfo{
			IncludePatterns: []string{"requirements.txt"},
		}), llb.WithCustomName("copy requirements.txt"))

		// Install dependencies with cache
		state = pipState.Dir("/app").Run(
			llb.Shlex("pip install -r requirements.txt"),
			llb.AddMount("/root/.cache/pip", pipCache, llb.AsPersistentCacheDir("pip", llb.CacheMountShared)),
			llb.WithCustomName("[phase] Installing Python dependencies with pip"),
		).Root()

	case pythonPkgPipenv:
		// Copy only Pipfile and Pipfile.lock first
		pipState := state.File(llb.Copy(localCtx, "/", "/app", &llb.CopyInfo{
			IncludePatterns: []string{"Pipfile", "Pipfile.lock"},
		}), llb.WithCustomName("copy Pipfile"))

		state = pipState.Dir("/app").Run(
			llb.Shlex("pip install pipenv"),
			llb.AddMount("/root/.cache/pip", pipCache, llb.AsPersistentCacheDir("pip", llb.CacheMountShared)),
			llb.WithCustomName("[phase] Installing Python pipenv"),
		).Root()

		state = state.File(llb.Mkdir("/home/app/.cache", 0777, llb.WithParents(true)))

		// Install pipenv and dependencies with cache
		state = state.Dir("/app").Run(
			llb.Shlex("pipenv install --deploy"),
			llb.AddMount("/home/app/.cache/pip", userPipCache, llb.AsPersistentCacheDir("user-pip", llb.CacheMountShared)),
			llb.User("app"),
			llb.WithCustomName("[phase] Installing Python dependencies with pipenv"),
		).Root()

	case pythonPkgPoetry:
		// Copy only pyproject.toml and poetry.lock first
		poetryState := state.File(llb.Copy(localCtx, "/", "/app", &llb.CopyInfo{
			IncludePatterns: []string{"pyproject.toml", "poetry.lock", "README.md"},
		}), llb.WithCustomName("copy pyproject.toml"))

		state = poetryState.Run(
			llb.Shlex("pip install poetry"),
			llb.AddMount("/root/.cache/pip", pipCache, llb.AsPersistentCacheDir("pip", llb.CacheMountShared)),
			llb.WithCustomName("[phase] Installing Python poetry"),
		).Root()

		state = state.File(llb.Mkdir("/home/app/.cache", 0777, llb.WithParents(true)))

		// Install poetry and dependencies with cache
		state = state.Dir("/app").Run(
			llb.Shlex("poetry install --no-root"),
			llb.AddMount("/home/app/.cache/pip", userPipCache, llb.AsPersistentCacheDir("user-pip", llb.CacheMountShared)),
			llb.User("app"),
			llb.WithCustomName("[phase] Installing Python dependencies with poetry"),
		).Root()

	case pythonPkgUv:
		// Copy pyproject.toml and uv.lock first
		uvState := state.File(llb.Copy(localCtx, "/", "/app", &llb.CopyInfo{
			IncludePatterns: []string{"pyproject.toml", "uv.lock", "README.md"},
		}), llb.WithCustomName("copy pyproject.toml and uv.lock"))

		// Install uv
		state = uvState.Run(
			llb.Shlex("pip install uv"),
			llb.AddMount("/root/.cache/pip", pipCache, llb.AsPersistentCacheDir("pip", llb.CacheMountShared)),
			llb.WithCustomName("[phase] Installing uv"),
		).Root()

		// Install dependencies with uv sync
		state = s.chownApp(state).Dir("/app").Run(
			llb.Shlex("uv sync --no-dev"),
			llb.AddMount("/home/app/.cache", llb.Scratch().File(
				llb.Mkdir("/uv", 0777, llb.WithParents(true)),
			), llb.AsPersistentCacheDir("user-uv", llb.CacheMountShared)),
			llb.User("app"),
			llb.WithCustomName("[phase] Installing Python dependencies with uv"),
		).Root()
	}

	h := &highlevelBuilder{opts}

	// Copy the rest of the application code
	state = h.copyApp(state, localCtx)

	state = s.applyOnBuild(state, opts)

	state = s.chownApp(state)

	return &state, nil
}

func (s *PythonStack) Entrypoint() string {
	switch s.packageManager {
	case pythonPkgPoetry:
		return "poetry run"
	case pythonPkgPipenv:
		return "pipenv run"
	case pythonPkgUv:
		return "uv run"
	default:
		return ""
	}
}

func (s *PythonStack) findWSGIModule() string {
	// Look for wsgi.py in subdirectories (Django convention)
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() {
			wsgiPath := filepath.Join(entry.Name(), "wsgi.py")
			if s.hasFile(wsgiPath) {
				return entry.Name() + ".wsgi:application"
			}
		}
	}
	// Check for wsgi.py in root
	if s.hasFile("wsgi.py") {
		return "wsgi:app"
	}
	return ""
}

func (s *PythonStack) findASGIModule() string {
	// Look for asgi.py in subdirectories (Django ASGI convention)
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() {
			asgiPath := filepath.Join(entry.Name(), "asgi.py")
			if s.hasFile(asgiPath) {
				return entry.Name() + ".asgi:application"
			}
		}
	}
	// Check for asgi.py in root
	if s.hasFile("asgi.py") {
		return "asgi:app"
	}
	return ""
}

// pyprojectToml represents the structure of a pyproject.toml file for FastAPI config
type pyprojectToml struct {
	Tool struct {
		FastAPI struct {
			Entrypoint string `toml:"entrypoint"`
		} `toml:"fastapi"`
	} `toml:"tool"`
}

func (s *PythonStack) findFastAPIEntrypoint() string {
	content, err := s.readFile("pyproject.toml")
	if err != nil {
		return ""
	}

	var pyproject pyprojectToml
	if err := toml.Unmarshal(content, &pyproject); err != nil {
		return ""
	}

	return pyproject.Tool.FastAPI.Entrypoint
}

// uvLock represents the structure of a uv.lock file
type uvLock struct {
	Package []struct {
		Name    string `toml:"name"`
		Version string `toml:"version"`
	} `toml:"package"`
}

func (s *PythonStack) parseUvLock() map[string]bool {
	if s.uvPackages != nil {
		return s.uvPackages
	}

	content, err := s.readFile("uv.lock")
	if err != nil {
		return nil
	}

	var lock uvLock
	if err := toml.Unmarshal(content, &lock); err != nil {
		return nil
	}

	s.uvPackages = make(map[string]bool)
	for _, pkg := range lock.Package {
		// Normalize package name (replace - with _ for consistent matching)
		name := strings.ToLower(pkg.Name)
		s.uvPackages[name] = true
		// Also store with underscores replaced by hyphens and vice versa
		s.uvPackages[strings.ReplaceAll(name, "-", "_")] = true
		s.uvPackages[strings.ReplaceAll(name, "_", "-")] = true
	}

	return s.uvPackages
}

func (s *PythonStack) WebCommand() string {
	// Check for gunicorn with Django WSGI
	if s.hasGunicorn && !s.hasFastAPI {
		if s.wsgiModule != "" {
			return "gunicorn " + s.wsgiModule + " -b 0.0.0.0:$PORT"
		}
		// Fallback to common entry point
		return "gunicorn app:app -b 0.0.0.0:$PORT"
	}

	// FastAPI - use fastapi run command (FastAPI CLI)
	// This takes precedence over uvicorn since fastapi run is the recommended way
	if s.hasFastAPI {
		// Use configured entrypoint from [tool.fastapi] if available
		if s.fastapiEntrypoint != "" {
			return "fastapi run " + s.fastapiEntrypoint + " --host 0.0.0.0 --port $PORT"
		}
		// Fallback: check common entry points
		if s.hasFile("main.py") {
			return "fastapi run main.py --host 0.0.0.0 --port $PORT"
		}
		if s.hasFile("app.py") {
			return "fastapi run app.py --host 0.0.0.0 --port $PORT"
		}
		return "fastapi run main.py --host 0.0.0.0 --port $PORT"
	}

	// Check for uvicorn (ASGI - Starlette, other ASGI apps)
	if s.hasUvicorn {
		if s.asgiModule != "" {
			return "uvicorn " + s.asgiModule + " --host 0.0.0.0 --port $PORT"
		}
		// Fallback: check common entry points
		if s.hasFile("main.py") {
			return "uvicorn main:app --host 0.0.0.0 --port $PORT"
		}
		if s.hasFile("app.py") {
			return "uvicorn app:app --host 0.0.0.0 --port $PORT"
		}
		return "uvicorn main:app --host 0.0.0.0 --port $PORT"
	}

	// Flask without gunicorn (dev server)
	if s.hasFlask {
		return "flask run --host=0.0.0.0 --port=$PORT"
	}

	// Django without gunicorn (dev server - not recommended for production)
	if s.hasDjango && s.hasManagePy {
		return "python manage.py runserver 0.0.0.0:$PORT"
	}

	return ""
}

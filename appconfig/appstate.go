package appconfig

import (
	"errors"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"
	"golang.org/x/sys/unix"
)

const appStateFile = "app-state.toml"

type appStateStore struct {
	Apps map[string]AppState `toml:"apps"`
}

type AppState struct {
	Cluster string `toml:"cluster"`
}

func appStatePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "miren", appStateFile), nil
}

var appStatePathOverride string

func resolveAppStatePath() (string, error) {
	if appStatePathOverride != "" {
		return appStatePathOverride, nil
	}
	return appStatePath()
}

func loadAppStateStore(path string) (*appStateStore, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &appStateStore{Apps: make(map[string]AppState)}, nil
		}
		return nil, err
	}

	var store appStateStore
	if err := toml.Unmarshal(data, &store); err != nil {
		return nil, err
	}

	if store.Apps == nil {
		store.Apps = make(map[string]AppState)
	}

	return &store, nil
}

// LoadAppState reads the cluster state for the named app.
// Returns nil, nil if no state has been saved for this app.
func LoadAppState(appName string) (*AppState, error) {
	path, err := resolveAppStatePath()
	if err != nil {
		return nil, err
	}

	store, err := loadAppStateStore(path)
	if err != nil {
		return nil, err
	}

	state, ok := store.Apps[appName]
	if !ok {
		return nil, nil
	}

	return &state, nil
}

// SaveAppState writes the cluster state for the named app.
// Uses a file lock to make the read-modify-write atomic across processes.
func SaveAppState(appName string, state *AppState) error {
	path, err := resolveAppStatePath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	lockPath := path + ".lock"
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer lockFile.Close()

	if err := unix.Flock(int(lockFile.Fd()), unix.LOCK_EX); err != nil {
		return err
	}
	defer unix.Flock(int(lockFile.Fd()), unix.LOCK_UN)

	store, err := loadAppStateStore(path)
	if err != nil {
		return err
	}

	store.Apps[appName] = *state

	data, err := toml.Marshal(store)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

package build

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"miren.dev/runtime/api/build/build_v1alpha"
)

type sourceCache struct {
	dataPath string
	log      *slog.Logger
	locks    *appLocks
}

// appLocks provides per-app mutexes so concurrent builds for the same app
// serialize their cache reads and writes.
type appLocks struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func newAppLocks() *appLocks {
	return &appLocks{locks: make(map[string]*sync.Mutex)}
}

func (al *appLocks) lock(app string) {
	al.mu.Lock()
	l, ok := al.locks[app]
	if !ok {
		l = &sync.Mutex{}
		al.locks[app] = l
	}
	al.mu.Unlock()
	l.Lock()
}

func (al *appLocks) unlock(app string) {
	al.mu.Lock()
	l := al.locks[app]
	al.mu.Unlock()
	l.Unlock()
}

type cachedFileInfo struct {
	Hash string `json:"hash"`
	Size int64  `json:"size"`
	Mode int32  `json:"mode"`
}

func (sc *sourceCache) cacheDir(app string) (string, error) {
	if strings.ContainsAny(app, "/\\") || strings.Contains(app, "..") || app == "" {
		return "", fmt.Errorf("invalid app name for source cache: %q", app)
	}
	return filepath.Join(sc.dataPath, "source-code", app), nil
}

func (sc *sourceCache) loadManifest(app string) (map[string]cachedFileInfo, error) {
	dir, err := sc.cacheDir(app)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]cachedFileInfo{}, nil
		}
		return nil, err
	}

	var manifest map[string]cachedFileInfo
	if err := json.Unmarshal(data, &manifest); err != nil {
		sc.log.Debug("corrupt manifest.json in source cache, ignoring", "app", app, "error", err)
		return map[string]cachedFileInfo{}, nil
	}
	return manifest, nil
}

// stageMatchingFiles compares the client manifest against the cached source code
// and extracts matching files from the cache into buildDir. Returns the count of
// matched files and the list of paths that still need to be uploaded.
func (sc *sourceCache) stageMatchingFiles(app string, buildDir string, clientManifest []*build_v1alpha.FileManifestEntry) (matched int, needed []string, err error) {
	sc.locks.lock(app)
	defer sc.locks.unlock(app)

	cached, err := sc.loadManifest(app)
	if err != nil {
		return 0, nil, err
	}

	if len(cached) == 0 {
		// No cache — all files needed
		for _, entry := range clientManifest {
			needed = append(needed, entry.Path())
		}
		return 0, needed, nil
	}

	// Determine which files match
	var matchedPaths []string
	for _, entry := range clientManifest {
		ci, ok := cached[entry.Path()]
		if ok && ci.Hash == entry.Hash() {
			matchedPaths = append(matchedPaths, entry.Path())
		} else {
			needed = append(needed, entry.Path())
		}
	}

	if len(matchedPaths) == 0 {
		return 0, needed, nil
	}

	// Extract matching files from the cached layer
	dir, err := sc.cacheDir(app)
	if err != nil {
		return 0, nil, err
	}
	layerPath := filepath.Join(dir, "layer.tar.gz")
	if err := extractFilesFromLayer(layerPath, buildDir, matchedPaths); err != nil {
		// Cache is corrupt — fall back to full upload
		needed = append(needed, matchedPaths...)
		return 0, needed, nil
	}

	return len(matchedPaths), needed, nil
}

// safePath validates that joining base and rel stays within base, preventing path traversal.
func safePath(base, rel string) (string, error) {
	joined := filepath.Join(base, rel)
	if !strings.HasPrefix(filepath.Clean(joined), filepath.Clean(base)+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes base directory: %q", rel)
	}
	return joined, nil
}

func extractFilesFromLayer(layerPath, buildDir string, paths []string) error {
	f, err := os.Open(layerPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()

	wantSet := make(map[string]bool, len(paths))
	for _, p := range paths {
		wantSet[p] = true
	}

	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if hdr.Typeflag != tar.TypeReg || !wantSet[hdr.Name] {
			continue
		}

		outPath, pathErr := safePath(buildDir, hdr.Name)
		if pathErr != nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
			return err
		}

		outFile, err := os.Create(outPath)
		if err != nil {
			return err
		}

		if _, err := io.Copy(outFile, tr); err != nil {
			outFile.Close()
			return err
		}
		outFile.Chmod(os.FileMode(hdr.Mode) & os.ModePerm)
		outFile.Close()
	}

	return nil
}

// saveSourceImage creates a cached source code layer from the build directory.
// This should be called after all files are resolved but before the build starts.
func (sc *sourceCache) saveSourceImage(app string, buildDir string) error {
	sc.locks.lock(app)
	defer sc.locks.unlock(app)

	dir, err := sc.cacheDir(app)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Write to temporary files and rename on success to avoid leaving
	// a corrupt cache if we fail partway through.
	manifest := make(map[string]cachedFileInfo)

	tmpLayerPath := filepath.Join(dir, "layer.tar.gz.tmp")
	layerFile, err := os.Create(tmpLayerPath)
	if err != nil {
		return err
	}
	defer func() {
		layerFile.Close()
		os.Remove(tmpLayerPath) // no-op if already renamed
	}()

	gzw := gzip.NewWriter(layerFile)
	tw := tar.NewWriter(gzw)

	err = filepath.Walk(buildDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if path == buildDir {
			return nil
		}

		rp, _ := filepath.Rel(buildDir, path)

		hdr, hdrErr := tar.FileInfoHeader(info, "")
		if hdrErr != nil {
			return hdrErr
		}
		hdr.Name = rp

		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}

		if info.Mode().IsRegular() {
			f, fErr := os.Open(path)
			if fErr != nil {
				return fErr
			}

			h := sha256.New()
			w := io.MultiWriter(tw, h)

			_, copyErr := io.Copy(w, f)
			f.Close()
			if copyErr != nil {
				return copyErr
			}

			manifest[rp] = cachedFileInfo{
				Hash: hex.EncodeToString(h.Sum(nil)),
				Size: info.Size(),
				Mode: int32(info.Mode().Perm()),
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	if err := tw.Close(); err != nil {
		return err
	}
	if err := gzw.Close(); err != nil {
		return err
	}

	manifestData, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("marshaling manifest: %w", err)
	}

	tmpManifestPath := filepath.Join(dir, "manifest.json.tmp")
	if err := os.WriteFile(tmpManifestPath, manifestData, 0644); err != nil {
		return err
	}

	// Rename manifest first: if we crash between renames, the new manifest
	// won't match the old layer's contents, causing cache misses (safe)
	// rather than hash/content mismatch (corrupt).
	if err := os.Rename(tmpManifestPath, filepath.Join(dir, "manifest.json")); err != nil {
		os.Remove(tmpLayerPath)
		return err
	}
	layerPath := filepath.Join(dir, "layer.tar.gz")
	return os.Rename(tmpLayerPath, layerPath)
}

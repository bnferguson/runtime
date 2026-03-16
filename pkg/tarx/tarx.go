package tarx

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/shibumi/go-pathspec"
	"github.com/tonistiigi/fsutil"
)

// FileManifest represents a file's metadata for delta upload comparison.
type FileManifest struct {
	Path string
	Hash string
	Size int64
	Mode int32
}

// parseGitignore reads a .gitignore file and returns its patterns.
// Returns nil if the file doesn't exist or can't be read.
func parseGitignore(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var patterns []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			patterns = append(patterns, line)
		}
	}
	return patterns
}

// isGitignored checks whether a relative path is matched by any applicable
// gitignore in the map. The map is keyed by directory relative path ("." for
// root). Only gitignores whose directory is a parent of rp are consulted.
func isGitignored(rp string, isDir bool, gitignoreMap map[string][]string) (bool, error) {
	for dir, patterns := range gitignoreMap {
		if len(patterns) == 0 {
			continue
		}
		// Check that rp is within this gitignore's subtree
		if dir != "." {
			if !strings.HasPrefix(rp, dir+"/") {
				continue
			}
		}
		// Compute the path relative to the gitignore's directory
		relPath := rp
		if dir != "." {
			relPath = strings.TrimPrefix(rp, dir+"/")
		}
		paths := []string{relPath}
		if isDir {
			paths = append(paths, relPath+"/")
		}
		for _, checkPath := range paths {
			ignore, err := pathspec.GitIgnore(patterns, checkPath)
			if err != nil {
				return false, fmt.Errorf("invalid gitignore pattern: %w", err)
			}
			if ignore {
				return true, nil
			}
		}
	}
	return false, nil
}

// ValidatePattern checks if a pattern is valid for use with pathspec.GitIgnore
func ValidatePattern(pattern string) error {
	// Test the pattern with a dummy path to ensure it's valid
	_, err := pathspec.GitIgnore([]string{pattern}, "test")
	if err != nil {
		return fmt.Errorf("invalid pattern syntax: %w", err)
	}
	return nil
}

func MakeTar(dir string, includePatterns []string, uncompressedBytes *atomic.Int64) (io.ReadCloser, error) {
	return makeTarWithFilter(dir, includePatterns, func(string) bool { return true }, uncompressedBytes)
}

func TarToMap(r io.Reader) (map[string][]byte, error) {
	tr := tar.NewReader(r)

	m := make(map[string][]byte)

	for {
		th, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if th == nil {
			break
		}

		if th.Typeflag != tar.TypeReg {
			continue
		}

		buf := make([]byte, th.Size)
		n, err := io.ReadFull(tr, buf)
		if n == 0 && err != nil && err != io.EOF {
			return nil, err
		}

		m[th.Name] = buf[:n]
	}

	return m, nil
}

// ComputeManifest walks a directory using the same gitignore/include logic as MakeTar
// and returns a manifest of all regular files with their SHA-256 hashes.
func ComputeManifest(dir string, includePatterns []string) ([]FileManifest, error) {
	gitignoreMap := make(map[string][]string)
	rootPatterns := parseGitignore(filepath.Join(dir, ".gitignore"))
	rootPatterns = append(rootPatterns, ".git")
	gitignoreMap["."] = rootPatterns

	var manifest []FileManifest

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if dir == path {
			return nil
		}

		rp, _ := filepath.Rel(dir, path)

		if filepath.Base(rp) == ".gitignore" {
			return nil
		}

		isIncluded := false
		if len(includePatterns) > 0 {
			paths := []string{rp}
			if info.IsDir() {
				paths = append(paths, rp+"/")
			}
			for _, checkPath := range paths {
				match, matchErr := pathspec.GitIgnore(includePatterns, checkPath)
				if matchErr != nil {
					return fmt.Errorf("invalid include pattern: %w", matchErr)
				}
				if match {
					isIncluded = true
					break
				}
			}
		}

		if !isIncluded {
			ignored, ignoreErr := isGitignored(rp, info.IsDir(), gitignoreMap)
			if ignoreErr != nil {
				return ignoreErr
			}
			if ignored {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		if info.IsDir() {
			nestedGitignore := filepath.Join(path, ".gitignore")
			if patterns := parseGitignore(nestedGitignore); patterns != nil {
				gitignoreMap[rp] = patterns
			}
			return nil
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		hash, hashErr := hashFile(path)
		if hashErr != nil {
			return fmt.Errorf("hashing %s: %w", rp, hashErr)
		}

		manifest = append(manifest, FileManifest{
			Path: rp,
			Hash: hash,
			Size: info.Size(),
			Mode: int32(info.Mode().Perm()),
		})

		return nil
	})
	if err != nil {
		return nil, err
	}

	return manifest, nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// MakeEmptyTar returns a valid empty gzipped tar archive.
func MakeEmptyTar() io.ReadCloser {
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)
	tw.Close()
	gzw.Close()
	return io.NopCloser(&buf)
}

// MakeFilteredTar creates a gzipped tar like MakeTar but only includes files
// whose relative paths are in the onlyPaths set. Directory entries are included
// as needed to contain the requested files. If uncompressedBytes is non-nil,
// each file's uncompressed size is atomically added after it is written.
func MakeFilteredTar(dir string, includePatterns []string, onlyPaths map[string]bool, uncompressedBytes *atomic.Int64) (io.ReadCloser, error) {
	return makeTarWithFilter(dir, includePatterns, func(rp string) bool { return onlyPaths[rp] }, uncompressedBytes)
}

// makeTarWithFilter creates a gzipped tar of dir, applying gitignore/include logic,
// and only including regular files for which accept returns true. Directory entries
// are emitted lazily as needed to contain accepted files.
func makeTarWithFilter(dir string, includePatterns []string, accept func(string) bool, uncompressedBytes *atomic.Int64) (io.ReadCloser, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	gzw := gzip.NewWriter(w)
	tw := tar.NewWriter(gzw)

	gitignoreMap := make(map[string][]string)
	rootPatterns := parseGitignore(filepath.Join(dir, ".gitignore"))
	rootPatterns = append(rootPatterns, ".git")
	gitignoreMap["."] = rootPatterns

	go func() {
		defer w.Close()
		defer tw.Close()
		defer gzw.Close()

		emittedDirs := make(map[string]bool)

		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if dir == path {
				return nil
			}

			rp, _ := filepath.Rel(dir, path)

			if filepath.Base(rp) == ".gitignore" {
				return nil
			}

			isIncluded := false
			if len(includePatterns) > 0 {
				paths := []string{rp}
				if info.IsDir() {
					paths = append(paths, rp+"/")
				}
				for _, checkPath := range paths {
					match, matchErr := pathspec.GitIgnore(includePatterns, checkPath)
					if matchErr != nil {
						return fmt.Errorf("invalid include pattern: %w", matchErr)
					}
					if match {
						isIncluded = true
						break
					}
				}
			}

			if !isIncluded {
				ignored, ignoreErr := isGitignored(rp, info.IsDir(), gitignoreMap)
				if ignoreErr != nil {
					return ignoreErr
				}
				if ignored {
					if info.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}

			if info.IsDir() {
				nestedGitignore := filepath.Join(path, ".gitignore")
				if patterns := parseGitignore(nestedGitignore); patterns != nil {
					gitignoreMap[rp] = patterns
				}
				return nil
			}

			if !accept(rp) {
				return nil
			}

			// Emit parent directories if not yet emitted
			parts := strings.Split(filepath.Dir(rp), string(filepath.Separator))
			for i := range parts {
				if parts[i] == "." {
					continue
				}
				dirPath := strings.Join(parts[:i+1], string(filepath.Separator))
				if emittedDirs[dirPath] {
					continue
				}
				emittedDirs[dirPath] = true

				dirInfo, dirErr := os.Stat(filepath.Join(dir, dirPath))
				if dirErr != nil {
					continue
				}
				dirHdr, hdrErr := tar.FileInfoHeader(dirInfo, "")
				if hdrErr != nil {
					continue
				}
				dirHdr.Name = dirPath
				if err := tw.WriteHeader(dirHdr); err != nil {
					return err
				}
			}

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
				defer f.Close()
				if _, err := io.Copy(tw, &countingReader{r: f, counter: uncompressedBytes}); err != nil {
					return err
				}
			}

			return nil
		})
	}()

	return r, nil
}

// countingReader wraps an io.Reader and atomically adds bytes read to a counter.
type countingReader struct {
	r       io.Reader
	counter *atomic.Int64
}

func (cr *countingReader) Read(p []byte) (int, error) {
	n, err := cr.r.Read(p)
	if n > 0 && cr.counter != nil {
		cr.counter.Add(int64(n))
	}
	return n, err
}

func TarFS(r io.Reader, dir string) (fsutil.FS, error) {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}

	tr := tar.NewReader(gzr)

	for {
		th, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		path := filepath.Join(dir, th.Name)
		if th.Typeflag == tar.TypeDir {
			if err := os.Mkdir(path, 0755); err != nil {
				return nil, err
			}
		}

		if th.Typeflag == tar.TypeReg {
			f, err := os.Create(path)
			if err != nil {
				return nil, err
			}

			if _, err := io.Copy(f, tr); err != nil {
				return nil, err
			}

			f.Chmod(os.FileMode(th.FileInfo().Mode()) & os.ModePerm)
		}
	}

	return fsutil.NewFS(dir)
}

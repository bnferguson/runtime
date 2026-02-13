package tarx

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/shibumi/go-pathspec"
	"github.com/tonistiigi/fsutil"
)

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

func MakeTar(dir string, includePatterns []string) (io.ReadCloser, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	gzw := gzip.NewWriter(w)
	tw := tar.NewWriter(gzw)

	// Load gitignore patterns keyed by relative directory path ("." for root)
	gitignoreMap := make(map[string][]string)
	rootPatterns := parseGitignore(filepath.Join(dir, ".gitignore"))
	rootPatterns = append(rootPatterns, ".git") // Always ignore .git directory
	gitignoreMap["."] = rootPatterns

	go func() {
		defer w.Close()
		defer tw.Close()
		defer gzw.Close()

		// tar up dir and output it to tw
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if dir == path {
				return nil
			}

			rp, _ := filepath.Rel(dir, path)

			// Skip all .gitignore files
			if filepath.Base(rp) == ".gitignore" {
				return nil
			}

			// Check if file matches include patterns first
			isIncluded := false
			if len(includePatterns) > 0 {
				// Try both with and without trailing slash for directories
				paths := []string{rp}
				if info.IsDir() {
					paths = append(paths, rp+"/")
				}

				for _, checkPath := range paths {
					match, err := pathspec.GitIgnore(includePatterns, checkPath)
					if err != nil {
						return fmt.Errorf("invalid include pattern: %w", err)
					}
					if match {
						isIncluded = true
						break
					}
				}
			}

			// Skip gitignore check if file is explicitly included
			if !isIncluded {
				ignored, err := isGitignored(rp, info.IsDir(), gitignoreMap)
				if err != nil {
					return err
				}
				if ignored {
					if info.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}

			// If this directory survived the ignore check, load its .gitignore
			if info.IsDir() {
				nestedGitignore := filepath.Join(path, ".gitignore")
				if patterns := parseGitignore(nestedGitignore); patterns != nil {
					gitignoreMap[rp] = patterns
				}
			}

			hdr, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return err
			}

			hdr.Name = rp

			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}

			if info.Mode().IsRegular() {
				f, err := os.Open(path)
				if err != nil {
					return err
				}
				defer f.Close()

				if _, err := io.Copy(tw, f); err != nil {
					return err
				}
			}

			return nil
		})
	}()

	return r, nil
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

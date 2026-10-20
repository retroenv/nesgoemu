package mcpserver

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// roots restricts file paths to the configured directories.
type roots struct {
	dirs []string
}

// newRoots normalizes the root directories. No root selects the working
// directory.
func newRoots(dirs []string) (*roots, error) {
	if len(dirs) == 0 {
		current, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("determining working directory: %w", err)
		}
		dirs = []string{current}
	}

	absolute := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		resolved, err := absolutePath(dir)
		if err != nil {
			return nil, fmt.Errorf("resolving root %q: %w", dir, err)
		}
		absolute = append(absolute, resolved)
	}
	return &roots{dirs: absolute}, nil
}

// first returns the first root directory.
func (r *roots) first() string {
	return r.dirs[0]
}

// resolve returns the absolute path when it lies inside a root. A relative path
// starts at the first root. It follows symbolic links, so a link inside a root
// cannot lead out of the root.
func (r *roots) resolve(path string) (string, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(r.first(), path)
	}
	resolved, err := absolutePath(path)
	if err != nil {
		return "", fmt.Errorf("resolving path %q: %w", path, err)
	}

	for _, dir := range r.dirs {
		if resolved == dir || strings.HasPrefix(resolved, dir+string(filepath.Separator)) {
			return resolved, nil
		}
	}
	return "", fmt.Errorf("path %q is outside the allowed roots", path)
}

// absolutePath returns the clean absolute path with symbolic links resolved.
// A part of the path that does not exist yet stays as it is, so the result can
// name a file that a tool creates.
func absolutePath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("making path absolute: %w", err)
	}

	current := filepath.Clean(absolute)
	var missing []string
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			slices.Reverse(missing)
			return filepath.Join(append([]string{resolved}, missing...)...), nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("resolving symbolic links: %w", err)
		}

		parent := filepath.Dir(current)
		if parent == current {
			return filepath.Clean(absolute), nil
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}

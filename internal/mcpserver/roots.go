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

	return &roots{
		dirs: absolute,
	}, nil
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
		if within(dir, resolved) {
			return resolved, nil
		}
	}

	return "", fmt.Errorf("path %q is outside the allowed roots", path)
}

// resolveOutput resolves a path that a tool creates. The file name must have
// the extension, and an existing target must be a regular file, so a tool
// cannot replace a ROM, a directory, or a device.
func (r *roots) resolveOutput(path, extension string) (string, error) {
	resolved, err := r.resolve(path)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(filepath.Ext(resolved), extension) {
		return "", fmt.Errorf("output path %q must end in %s", path, extension)
	}

	info, err := os.Stat(resolved)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return "", fmt.Errorf("checking output path %q: %w", path, err)
	case !info.Mode().IsRegular():
		return "", fmt.Errorf("output path %q is not a regular file", path)
	}

	return resolved, nil
}

// within reports whether path is dir or lies below it.
func within(dir, path string) bool {
	relative, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

// maxSymlinkHops limits the symbolic links that absolutePath follows through
// dangling links.
const maxSymlinkHops = 40

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
	hops := 0
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			slices.Reverse(missing)
			return filepath.Join(append([]string{resolved}, missing...)...), nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("resolving symbolic links: %w", err)
		}

		// A dangling link has a target that does not exist yet. A tool that
		// creates the file would follow the link, so continue at the target.
		if info, statErr := os.Lstat(current); statErr == nil && info.Mode()&fs.ModeSymlink != 0 {
			hops++
			if hops > maxSymlinkHops {
				return "", errors.New("too many symbolic links")
			}
			target, readErr := os.Readlink(current)
			if readErr != nil {
				return "", fmt.Errorf("reading symbolic link: %w", readErr)
			}
			if !filepath.IsAbs(target) {
				target = filepath.Join(filepath.Dir(current), target)
			}

			current = filepath.Clean(target)

			continue
		}

		parent := filepath.Dir(current)
		if parent == current {
			return filepath.Clean(absolute), nil
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}

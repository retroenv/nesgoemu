package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type romMedia struct {
	dir  string
	file *os.File
}

func (rm *romMedia) open(request string) (io.ReaderAt, error) {
	rm.close()

	name, err := filepath.Localize(request)
	if err != nil {
		return nil, fmt.Errorf("invalid media path %q: %w", request, err)
	}

	dir, err := filepath.Abs(rm.dir)
	if err != nil {
		return nil, fmt.Errorf("resolving ROM directory: %w", err)
	}

	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("opening ROM directory: %w", err)
	}
	defer func() { _ = root.Close() }()

	file, err := openMediaFile(root, name)
	if errors.Is(err, fs.ErrNotExist) && strings.HasSuffix(filepath.ToSlash(dir), "/"+path.Dir(request)) {
		// If the ROM directory ends with the requested directory, look for
		// the file next to the ROM.
		file, err = openMediaFile(root, filepath.Base(name))
	}

	if err != nil {
		return nil, err
	}

	rm.file = file

	return file, nil
}

func (rm *romMedia) close() {
	if rm.file != nil {
		_ = rm.file.Close()
		rm.file = nil
	}
}

func openMediaFile(root *os.Root, name string) (*os.File, error) {
	info, err := root.Stat(name)
	if err != nil {
		return nil, fmt.Errorf("checking media file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("media file %q must be a regular file", name)
	}

	file, err := root.Open(name)
	if err != nil {
		return nil, fmt.Errorf("opening media file: %w", err)
	}
	return file, nil
}

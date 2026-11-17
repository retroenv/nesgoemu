package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// maxEventLogFileBytes limits the size of an event log file that the server
// reads or writes.
const maxEventLogFileBytes = 64 << 20

// maxBatteryFileBytes bounds initial cartridge save data.
const maxBatteryFileBytes = 64 << 20

// writeFileAtomic writes a file through a temporary file in the same
// directory. The target stays unchanged when the write fails, and a reader
// never sees a partial file.
func writeFileAtomic(path string, write func(io.Writer) error) (err error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	file, err := os.CreateTemp(dir, ".nesgoemu-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temporary file: %w", err)
	}
	defer func() {
		if err != nil {
			_ = file.Close()
			_ = os.Remove(file.Name())
		}
	}()

	if err := write(file); err != nil {
		return err
	}
	if err := file.Chmod(0o644); err != nil {
		return fmt.Errorf("setting file mode: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("closing file: %w", err)
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return fmt.Errorf("replacing file: %w", err)
	}
	return nil
}

// readRegularFile reads a regular file of at most limit bytes.
func readRegularFile(ctx context.Context, path string, limit int64) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf(readingFileFormat, err)
	}

	file, err := openRegularFile(path)
	if err != nil {
		return nil, fmt.Errorf(openingFileFormat, err)
	}
	defer func() { _ = file.Close() }()
	stop := context.AfterFunc(ctx, func() {
		_ = file.Close()
	})
	defer stop()

	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, fmt.Errorf(readingFileFormat, err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("file is larger than %d bytes", limit)
	}
	return data, nil
}

// openRegularFile rejects special files before an open can block.
func openRegularFile(path string) (*os.File, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("checking file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("not a regular file")
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf(openingFileFormat, err)
	}
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		_ = file.Close()
		return nil, errors.New("file changed while opening")
	}
	return file, nil
}

// loadBatteryData uses the stored initial save or reads it once at creation.
func loadBatteryData(ctx context.Context, rom string, enabled bool, initial *[]byte) ([]byte, error) {
	if !enabled {
		return nil, nil
	}
	if initial != nil {
		return *initial, nil
	}
	path := rom + ".sav"
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("checking battery save: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("save file %q is a symbolic link", path)
	}
	data, err := readRegularFile(ctx, path, maxBatteryFileBytes)
	if err != nil {
		return nil, fmt.Errorf("reading battery save: %w", err)
	}
	return data, nil
}

package nes

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// SaveBattery writes cartridge save data to the configured path.
// Call this method while emulation is stopped.
func (sys *System) SaveBattery() error {
	mapper, ok := sys.Bus.Mapper.(batteryMapper)
	if !ok || sys.opts.savePath == "" {
		return nil
	}
	if err := sys.storage.Save(sys.opts.savePath, mapper.SaveBattery); err != nil {
		return fmt.Errorf("saving battery: %w", err)
	}
	return nil
}

func (sys *System) loadBattery() error {
	mapper, ok := sys.Bus.Mapper.(batteryMapper)
	if !ok || sys.opts.savePath == "" {
		return nil
	}
	if err := sys.storage.Load(sys.opts.savePath, mapper.LoadBattery); err != nil {
		return fmt.Errorf("loading battery: %w", err)
	}
	return nil
}

type batteryMapper interface {
	LoadBattery(io.Reader) error
	SaveBattery(io.Writer) error
}

type batteryStorage interface {
	Load(path string, load func(io.Reader) error) error
	Save(path string, save func(io.Writer) error) error
}

type diskBatteryStorage struct {
	files batteryFiles
}

type batterySaveFile interface {
	io.WriteCloser
	Name() string
	Sync() error
}

type batteryFiles interface {
	CreateTemp(dir, pattern string) (batterySaveFile, error)
	Open(path string) (io.ReadCloser, error)
	Remove(path string) error
	Rename(oldPath, newPath string) error
}

type osBatteryFiles struct{}

func (store diskBatteryStorage) Load(path string, load func(io.Reader) error) error {
	file, err := store.files.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("opening save: %w", err)
	}
	defer func() { _ = file.Close() }()

	if err := load(file); err != nil {
		return fmt.Errorf("loading cartridge save: %w", err)
	}
	return nil
}

func (store diskBatteryStorage) Save(path string, save func(io.Writer) error) error {
	file, err := store.files.CreateTemp(filepath.Dir(path), ".nesgoemu-save-*")
	if err != nil {
		return fmt.Errorf("creating save: %w", err)
	}
	defer func() { _ = file.Close(); _ = store.files.Remove(file.Name()) }()

	if err := save(file); err != nil {
		return fmt.Errorf("saving cartridge: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("syncing save: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("closing save: %w", err)
	}
	if err := store.files.Rename(file.Name(), path); err != nil {
		return fmt.Errorf("replacing save: %w", err)
	}
	return nil
}

func (osBatteryFiles) CreateTemp(dir, pattern string) (batterySaveFile, error) {
	file, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return nil, fmt.Errorf("creating temporary file: %w", err)
	}
	return file, nil
}

func (osBatteryFiles) Open(path string) (io.ReadCloser, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	return file, nil
}

func (osBatteryFiles) Remove(path string) error {
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("removing file: %w", err)
	}
	return nil
}

func (osBatteryFiles) Rename(oldPath, newPath string) error {
	if err := os.Rename(oldPath, newPath); err != nil {
		return fmt.Errorf("renaming file: %w", err)
	}
	return nil
}

package nes

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
	"github.com/retroenv/retrogolib/gui"
)

func TestBatteryRoundTrip(t *testing.T) {
	const path = "test.sav"
	mapper := &testBatteryMapper{value: 0xAB}
	files := &memoryBatteryFiles{}
	sys := &System{
		Bus:     &bus.Bus{Mapper: mapper},
		opts:    NewOptions(WithSavePath(path)),
		storage: diskBatteryStorage{files: files},
	}

	assert.NoError(t, sys.loadBattery())
	assert.NoError(t, sys.SaveBattery())
	assert.Equal(t, []byte{0xAB}, files.data[path])
	mapper.value = 0
	assert.NoError(t, sys.loadBattery())
	assert.Equal(t, byte(0xAB), mapper.value)

	files.data[path] = nil
	assert.Error(t, sys.loadBattery())
	assert.Equal(t, byte(0xAB), mapper.value)
}

func TestBatterySaveKeepsOldDataOnError(t *testing.T) {
	const path = "test.sav"
	files := &memoryBatteryFiles{data: map[string][]byte{path: {0xAB}}}
	mapper := &testBatteryMapper{saveErr: errors.New("save failed")}
	sys := &System{
		Bus:     &bus.Bus{Mapper: mapper},
		opts:    NewOptions(WithSavePath(path)),
		storage: diskBatteryStorage{files: files},
	}

	assert.Error(t, sys.SaveBattery())
	assert.Equal(t, []byte{0xAB}, files.data[path])
	assert.Len(t, files.data, 1)
}

func TestBatteryHooksAreOptional(t *testing.T) {
	const path = "test.sav"
	files := &memoryBatteryFiles{}
	sys := &System{
		Bus:     &bus.Bus{Mapper: plainMapper{}},
		opts:    NewOptions(WithSavePath(path)),
		storage: diskBatteryStorage{files: files},
	}
	assert.NoError(t, sys.SaveBattery())
	assert.NoError(t, sys.loadBattery())
	assert.Equal(t, 0, files.calls)

	sys.Bus.Mapper = &testBatteryMapper{value: 0xAB}
	sys.opts = NewOptions()
	assert.NoError(t, sys.SaveBattery())
	assert.NoError(t, sys.loadBattery())
	assert.Equal(t, 0, files.calls)
}

func TestEmulatorStopsOnCancellation(t *testing.T) {
	sys, err := NewSystem(NewOptions(WithCartridge(cartridge.New())))
	assert.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	assert.NoError(t, sys.runEmulatorSteps(ctx, -1))
}

func TestRendererWaitsForCPUShutdown(t *testing.T) {
	cart := cartridge.New()
	copy(cart.PRG, []byte{0x4C, 0, 0x80})
	cart.PRG[0x7FFD] = 0x80
	sys, err := NewSystem(NewOptions(WithCartridge(cart)))
	assert.NoError(t, err)
	sys.CPU.Reset()
	starter := func(_ gui.Backend) (func() (bool, error), func(), error) {
		return func() (bool, error) { return false, nil }, func() {}, nil
	}

	assert.NoError(t, sys.runRenderer(t.Context(), sys.opts, starter, nil))
	// The CPU worker has stopped before the next memory access.
	sys.Bus.Memory.Write(0, 0xAB)
	assert.Equal(t, byte(0xAB), sys.Bus.Memory.Read(0))
}

type testBatteryMapper struct {
	plainMapper
	value   byte
	saveErr error
}

type memoryBatteryFiles struct {
	data  map[string][]byte
	calls int
}

type memoryBatteryFile struct {
	bytes.Buffer
	files  *memoryBatteryFiles
	name   string
	closed bool
}

func (files *memoryBatteryFiles) CreateTemp(dir, _ string) (batterySaveFile, error) {
	files.calls++
	name := filepath.Join(dir, fmt.Sprintf(".nesgoemu-save-%d", files.calls))
	return &memoryBatteryFile{
		files: files,
		name:  name,
	}, nil
}

func (files *memoryBatteryFiles) Open(path string) (io.ReadCloser, error) {
	files.calls++
	data, ok := files.data[path]
	if !ok {
		return nil, fmt.Errorf("test file missing: %w", os.ErrNotExist)
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (files *memoryBatteryFiles) Remove(path string) error {
	files.calls++
	delete(files.data, path)
	return nil
}

func (files *memoryBatteryFiles) Rename(oldPath, newPath string) error {
	files.calls++
	data, ok := files.data[oldPath]
	if !ok {
		return errors.New("temporary test file missing")
	}
	files.data[newPath] = data
	delete(files.data, oldPath)
	return nil
}

func (file *memoryBatteryFile) Close() error {
	if file.closed {
		return nil
	}
	if file.files.data == nil {
		file.files.data = make(map[string][]byte)
	}
	file.files.data[file.name] = bytes.Clone(file.Buffer.Bytes())
	file.closed = true
	return nil
}

func (file *memoryBatteryFile) Name() string { return file.name }

func (*memoryBatteryFile) Sync() error { return nil }

func (mapper *testBatteryMapper) LoadBattery(reader io.Reader) error {
	var data [1]byte
	if _, err := io.ReadFull(reader, data[:]); err != nil {
		return fmt.Errorf("reading test save: %w", err)
	}
	mapper.value = data[0]
	return nil
}

func (mapper *testBatteryMapper) SaveBattery(writer io.Writer) error {
	if mapper.saveErr != nil {
		return mapper.saveErr
	}
	_, err := writer.Write([]byte{mapper.value})
	if err != nil {
		return fmt.Errorf("writing test save: %w", err)
	}
	return nil
}

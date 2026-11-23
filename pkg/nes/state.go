package nes

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/retroenv/nesgoemu/internal/stateio"
	"github.com/retroenv/retrogolib/arch/cpu/cpu6502"
)

const systemStateVersion = 2

// MaxStateSize is the maximum size in bytes of a system snapshot.
const MaxStateSize = 64 << 20

// ErrStateUnsupported means that the mapper needs event-log replay.
var ErrStateUnsupported = errors.New("mapper does not support direct snapshots")

// SaveState serializes device state at an instruction boundary.
// Stop emulation and the audio worker before calling it.
func (sys *System) SaveState() ([]byte, error) {
	mapper, ok := sys.Bus.Mapper.(stateMapper)
	if !ok {
		return nil, ErrStateUnsupported
	}

	var mapperData bytes.Buffer
	if err := mapper.TransferState(stateio.NewWriter(&mapperData)); err != nil {
		return nil, fmt.Errorf("saving mapper: %w", err)
	}

	var out bytes.Buffer
	archive := stateio.NewWriter(&out)
	header, err := sys.stateHeader(mapper.SnapshotFormat())
	if err != nil {
		return nil, err
	}

	cpu := sys.CPU.SaveExecutionState()
	data := mapperData.Bytes()
	if err := archive.Values(&header, &data); err != nil {
		return nil, fmt.Errorf("saving system state: %w", err)
	}

	if err := sys.transferState(archive); err != nil {
		return nil, fmt.Errorf("saving system state: %w", err)
	}

	if err := archive.Values(&cpu); err != nil {
		return nil, fmt.Errorf("saving system state: %w", err)
	}
	return out.Bytes(), nil
}

// LoadState restores a snapshot into a new, stopped system with the same ROM.
// On error, discard this system. The caller must limit the input to 64 MiB.
func (sys *System) LoadState(data []byte) error {
	mapper, ok := sys.Bus.Mapper.(stateMapper)
	if !ok {
		return ErrStateUnsupported
	}

	if len(data) > MaxStateSize {
		return errors.New("system state exceeds 64 MiB")
	}

	archive := stateio.NewReader(bytes.NewReader(data))
	var header systemStateHeader
	var mapperData []byte
	if err := archive.Values(&header, &mapperData); err != nil {
		return fmt.Errorf("transferring nes state: %w", err)
	}

	expected, err := sys.stateHeader(mapper.SnapshotFormat())
	if err != nil {
		return err
	}

	if header != expected {
		return errors.New("snapshot version, mapper, or ROM differs")
	}

	mapperArchive := stateio.NewReader(bytes.NewReader(mapperData))
	if err := mapper.TransferState(mapperArchive); err != nil {
		return fmt.Errorf("restoring mapper: %w", err)
	}

	if err := mapperArchive.End(); err != nil {
		return fmt.Errorf("checking mapper state end: %w", err)
	}

	if err := sys.transferState(archive); err != nil {
		return fmt.Errorf("transferring nes state: %w", err)
	}

	var cpu cpu6502.ExecutionState
	if err := archive.Values(&cpu); err != nil {
		return fmt.Errorf("transferring nes state: %w", err)
	}

	if err := archive.End(); err != nil {
		return fmt.Errorf("checking state end: %w", err)
	}

	if err := sys.CPU.RestoreExecutionState(cpu); err != nil {
		return fmt.Errorf("restoring CPU state: %w", err)
	}

	return nil
}

func (sys *System) stateHeader(format string) (systemStateHeader, error) {
	cart := sys.Bus.Cartridge
	var misc []byte
	if cart.NES2 != nil {
		misc = cart.NES2.MiscROM
	}

	raw, err := cart.HeaderBytes()
	if err != nil {
		return systemStateHeader{}, fmt.Errorf("encoding snapshot ROM identity: %w", err)
	}
	return systemStateHeader{
		Format:  format,
		Mapper:  cart.Mapper,
		Version: systemStateVersion,

		CHR:     sha256.Sum256(cart.CHR),
		Header:  raw,
		Misc:    sha256.Sum256(misc),
		PRG:     sha256.Sum256(cart.PRG),
		Trainer: sha256.Sum256(cart.Trainer),
	}, nil
}

func (sys *System) transferState(archive *stateio.Archive) error {
	d := &sys.dma
	if err := archive.Values(&sys.frameBoundary, &d.halted, &d.dmcAddress, &d.dmcArmed, &d.dmcCycles, &d.dmcPhase,
		&d.oamActive, &d.oamPending, &d.oamPage, &d.oamOffset, &d.oamValue, &d.oamFull); err != nil {
		return fmt.Errorf("transferring nes state: %w", err)
	}

	if features, ok := sys.Bus.Mapper.(mapperFeatureState); ok {
		if err := features.TransferFeatures(archive); err != nil {
			return fmt.Errorf("transferring mapper features: %w", err)
		}
	}

	components := []any{sys.memory, sys.apu, sys.Bus.PPU, sys.Bus.NameTable, sys.Bus.Controller1, sys.Bus.Controller2, sys.Bus}
	for _, component := range components {
		device, ok := component.(stateio.Device)
		if !ok {
			return fmt.Errorf("snapshot unavailable for %T", component)
		}
		if err := device.TransferState(archive); err != nil {
			return fmt.Errorf("transferring nes state: %w", err)
		}
	}

	return nil
}

type stateMapper interface {
	TransferState(*stateio.Archive) error
	SnapshotFormat() string
}

type systemStateHeader struct {
	Format  string
	Mapper  uint16
	Version int

	CHR     [sha256.Size]byte
	Header  [16]byte
	Misc    [sha256.Size]byte
	PRG     [sha256.Size]byte
	Trainer [sha256.Size]byte
}

type mapperFeatureState interface {
	TransferFeatures(*stateio.Archive) error
}

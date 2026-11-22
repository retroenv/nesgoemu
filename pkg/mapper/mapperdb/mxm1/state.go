package mxm1

import (
	"errors"
	"fmt"

	"github.com/retroenv/nesgoemu/internal/stateio"
)

const registerStateErrorFormat = "transferring MXM-1 registers: %w"

// SnapshotFormat identifies the direct mapper state format.
func (*Mapper) SnapshotFormat() string { return "mxm1-v1" }

// TransferState keeps effective windows separate from register readback.
func (m *Mapper) TransferState(archive *stateio.Archive) error {
	if err := m.Base.TransferState(archive); err != nil {
		return fmt.Errorf("transferring MXM-1 base: %w", err)
	}
	if err := m.transferRegistersA(archive); err != nil {
		return err
	}
	if err := m.transferRegistersB(archive); err != nil {
		return err
	}
	if err := archive.Values(&m.fetch.dummyAttribute); err != nil {
		return fmt.Errorf("transferring MXM-1 latch: %w", err)
	}

	for _, windows := range [][]window{m.cpuWindows[:], m.ppuWindows[:], m.ntWindows[:]} {
		for i := range windows {
			if err := m.transferWindow(archive, &windows[i]); err != nil {
				return err
			}
		}
	}

	return nil
}

func (m *Mapper) transferWindow(archive *stateio.Archive, w *window) error {
	source := 0
	memories := [][]byte{nil, m.prgROM, m.chrROM, m.ram, m.chrMemory}

	if !archive.Reading() && len(w.data) > 0 {
		source = -1
		for i, data := range memories {
			if len(data) == len(w.data) && len(data) > 0 && &data[0] == &w.data[0] {
				source = i
				break
			}
		}

		if source < 0 {
			return errors.New("unknown MXM-1 snapshot window backing")
		}
	}

	if err := archive.Values(&source, &w.offset, &w.writable, &w.prg); err != nil {
		return fmt.Errorf("transferring MXM-1 window: %w", err)
	}
	if source < 0 || source >= len(memories) || w.offset < 0 {
		return errors.New("invalid MXM-1 snapshot window")
	}
	if archive.Reading() {
		w.data = memories[source]
	}

	return nil
}

func (m *Mapper) transferRegistersA(archive *stateio.Archive) error {
	if err := archive.Values(&m.prgMode, &m.prgSelect, &m.prgFlags, &m.prgBanks, &m.readLatch, &m.altEnabled); err != nil {
		return fmt.Errorf(registerStateErrorFormat, err)
	}

	if err := archive.Values(&m.altRAM, &m.altBank, &m.chrMode, &m.chrSelect, &m.chrRAM, &m.chrBanks); err != nil {
		return fmt.Errorf(registerStateErrorFormat, err)
	}

	if err := archive.Values(&m.irqEnabled, &m.irqTarget, &m.irqOffset, &m.scanline, &m.irqPending, &m.previousPending); err != nil {
		return fmt.Errorf(registerStateErrorFormat, err)
	}

	if err := archive.Values(&m.irqPhase, &m.readInhibit, &m.descriptor.enabled, &m.attributeMode, &m.descriptor.selected, &m.descriptorRAM); err != nil {
		return fmt.Errorf(registerStateErrorFormat, err)
	}

	if err := archive.Values(&m.patternRAM, &m.descriptor.banks, &m.manualScroll, &m.verticalPosition, &m.nametable.enabled, &m.ntMode); err != nil {
		return fmt.Errorf(registerStateErrorFormat, err)
	}
	if err := archive.Values(&m.nametable.selected, &m.ntRAM, &m.nametable.banks); err != nil {
		return fmt.Errorf(registerStateErrorFormat, err)
	}
	return nil
}

func (m *Mapper) transferRegistersB(archive *stateio.Archive) error {
	if err := archive.Values(&m.wideBanks, &m.spriteMode, &m.secondPlane, &m.spriteSecondPlane, &m.writeBank, &m.copySource); err != nil {
		return fmt.Errorf(registerStateErrorFormat, err)
	}

	if err := archive.Values(&m.copyDestination, &m.copyFrom, &m.copyTo, &m.metatileEnabled, &m.metatileTables, &m.metatileStride); err != nil {
		return fmt.Errorf(registerStateErrorFormat, err)
	}

	if err := archive.Values(&m.metatileFrame, &m.remapEnabled, &m.remapTable, &m.remapCursor, &m.remapPage, &m.streamCursor); err != nil {
		return fmt.Errorf(registerStateErrorFormat, err)
	}

	if err := archive.Values(&m.remapNTFlip801, &m.remapOffset801, &m.remapPosition801); err != nil {
		return fmt.Errorf(registerStateErrorFormat, err)
	}

	if err := archive.Values(&m.ppuRegisters, &m.scrollDirty, &m.fetch.lastAddress, &m.fetch.repeatCount, &m.fetch.frameStart, &m.fetch.inFrame); err != nil {
		return fmt.Errorf(registerStateErrorFormat, err)
	}

	if err := archive.Values(&m.fetch.nameReads, &m.fetch.previousNameReads, &m.fetch.metatileFetchesLeft, &m.fetch.metatileLow, &m.fetch.metatileHigh, &m.fetch.metatilePalette); err != nil {
		return fmt.Errorf(registerStateErrorFormat, err)
	}

	if err := archive.Values(&m.fetch.descriptorFetchesLeft, &m.fetch.tileOffset, &m.fetch.tileTable, &m.fetch.tileFineY); err != nil {
		return fmt.Errorf(registerStateErrorFormat, err)
	}
	return nil
}

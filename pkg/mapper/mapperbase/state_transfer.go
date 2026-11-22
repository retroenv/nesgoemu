package mapperbase

import (
	"errors"
	"fmt"

	"github.com/retroenv/nesgoemu/internal/stateio"
)

// TransferState transfers stopped Base state. The system owns the record version.
func (b *Base) TransferState(archive *stateio.Archive) error {
	if err := archive.Bytes(b.prgRAM); err != nil {
		return fmt.Errorf("transferring mapperbase state: %w", err)
	}
	if err := archive.Bytes(b.chrRAM); err != nil {
		return fmt.Errorf("transferring mapperbase state: %w", err)
	}

	if err := b.transferWindows(archive); err != nil {
		return err
	}
	if err := b.transferNameTables(archive); err != nil {
		return err
	}

	if err := archive.Devices(b.features); err != nil {
		return fmt.Errorf("transferring mapperbase state: %w", err)
	}
	return nil
}

// TransferFeatures transfers mapper feature counters while emulation is stopped.
func (b *Base) TransferFeatures(archive *stateio.Archive) error {
	if err := b.features.TransferState(archive); err != nil {
		return fmt.Errorf("transferring mapper features: %w", err)
	}
	return nil
}

func (b *Base) transferWindows(archive *stateio.Archive) error {
	chrCount := len(b.chrWindows)
	prgCount := len(b.prgWindows)

	if err := archive.Values(&b.chrWindows, &b.prgWindows); err != nil {
		return fmt.Errorf("transferring mapperbase state: %w", err)
	}
	if len(b.chrWindows) != chrCount || len(b.prgWindows) != prgCount {
		return errors.New("invalid mapper snapshot window count")
	}

	for _, bank := range b.chrWindows {
		if bank < 0 || (len(b.chrBanks) > 0 && bank >= len(b.chrBanks)) || (len(b.chrBanks) == 0 && bank != 0) {
			return errors.New("invalid CHR snapshot bank")
		}
	}

	for _, bank := range b.prgWindows {
		if bank < 0 || bank >= len(b.prgBanks) {
			return errors.New("invalid PRG snapshot bank")
		}
	}

	return nil
}

func (b *Base) transferNameTables(archive *stateio.Archive) error {
	selected := 0
	inspector, ok := b.bus.NameTable.(nametableBufferInspector)
	if !ok {
		return errors.New("nametable snapshot inspection is unavailable")
	}

	for i := range b.nameTableBanks {
		bank := &b.nameTableBanks[i]
		if inspector.UsesVRAM(bank.data) {
			selected = i
		}

		if err := archive.Bytes(bank.data); err != nil {
			return fmt.Errorf("transferring nametable bank: %w", err)
		}
	}

	if err := archive.Values(&selected); err != nil {
		return fmt.Errorf("transferring nametable selection: %w", err)
	}
	if selected < 0 || selected >= len(b.nameTableBanks) {
		return errors.New("invalid nametable snapshot bank")
	}
	if archive.Reading() {
		b.SetNameTableWindow(selected)
	}

	return nil
}

type nametableBufferInspector interface {
	UsesVRAM([]byte) bool
}

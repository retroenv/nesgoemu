package mapperbase

import (
	"encoding/json"
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/ppu/nametable"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

const (
	safetyTestCHRWindowSize = 0x2000
	safetyTestPRGROMSize    = 0x8000
	safetyTestCHRROMSize    = 0x4000
	safetyTestCHRRAMSize    = 0x4000
	safetyTestWriteValue    = 0x5a
	snapshotChangedBank     = 99
	stateSwitchIterations   = 1000
	stateSwitchBankCount    = 2
)

func TestStateOwnsBankWindows(t *testing.T) {
	base := newPPUMappingTestBase(t, safetyTestCHRWindowSize, false)
	state := base.State()
	base.SetChrWindow(0, 1)
	base.SetPrgWindow(0, 1)
	assert.Equal(t, 0, state.ChrWindows[0])
	assert.Equal(t, 0, state.PrgWindows[0])

	// A state result must not give the caller access to the live bank tables.
	state.ChrWindows[0] = snapshotChangedBank
	state.PrgWindows[0] = snapshotChangedBank
	assert.Equal(t, 1, base.State().ChrWindows[0])
	assert.Equal(t, 1, base.State().PrgWindows[0])
}

func TestStateDuringBankSwitches(t *testing.T) {
	base := newPPUMappingTestBase(t, safetyTestCHRWindowSize, false)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range stateSwitchIterations {
			base.SetChrWindow(0, i%stateSwitchBankCount)
			base.SetPrgWindow(0, i%stateSwitchBankCount)
		}
	}()
	for range stateSwitchIterations {
		_, err := json.Marshal(base.State())
		assert.NoError(t, err)
	}
	<-done
}

func TestCHRROMStaysReadOnlyWithRAMBuffer(t *testing.T) {
	base := New(&bus.Bus{
		Cartridge: &cartridge.Cartridge{
			PRG: make([]byte, safetyTestPRGROMSize),
			CHR: make([]byte, safetyTestCHRROMSize),
		},
		NameTable: nametable.New(cartridge.MirrorHorizontal),
	})
	base.SetChrRAM(make([]byte, safetyTestCHRRAMSize))
	assert.NoError(t, base.Initialize())
	var event bus.GraphicsEvent
	base.ObserveGraphics(func(value bus.GraphicsEvent) { event = value })
	base.SetChrWindow(0, 1)

	// The selected bank uses ROM even when a separate RAM buffer exists.
	base.Write(0, safetyTestWriteValue)
	assert.Equal(t, byte(0), base.Read(0))
	_, writable := base.PPUWriteMapping(0)
	assert.False(t, writable)
	assert.Equal(t, bus.GraphicsCHRROM, event.Page.Memory)
}

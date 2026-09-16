package rainbow

import (
	"bytes"
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

func TestStateRestoresMemoryAndPendingTransfers(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	m.prgROM[0] = 0xFF
	m.EnableBusTiming()
	m.Write(regESPControl, 3)
	m.Write(0x4800, 1)
	m.Write(0x4801, 0)
	m.Write(regESPStart, 0)
	m.Write(0x8AAA, 0xAA)
	m.Write(0x8555, 0x55)
	m.Write(0x8AAA, 0xA0)
	m.fpgaRAM[123] = 0xAB
	m.prgRAM[456] = 0xCD
	m.chrRAM[789] = 0xEF
	m.highBanks[1] = 0x1234
	m.scanIRQ.jitter = 42
	m.executeESP([]byte{byte(espServerSetSettings), 0x12, 0x34, 4, 't', 'e', 's', 't'})
	var saved bytes.Buffer
	assert.NoError(t, m.SaveState(&saved))
	m.ClockCPU(1)
	m.Write(0x8000, 0)
	m.Reset()
	m.fpgaRAM[123], m.prgRAM[456], m.chrRAM[789] = 0, 0, 0
	assert.NoError(t, m.LoadState(bytes.NewReader(saved.Bytes())))
	assert.Equal(t, byte(0xAB), m.fpgaRAM[123])
	assert.Equal(t, byte(0xCD), m.prgRAM[456])
	assert.Equal(t, byte(0xEF), m.chrRAM[789])
	assert.Equal(t, uint16(0x1234), m.highBanks[1])
	assert.Equal(t, byte(42), m.scanIRQ.jitter)
	assert.True(t, m.ppuBus.enabled)
	assert.Equal(t, byte(0xFF), m.prgROM[0])
	assert.Equal(t, flashProgram, m.prgFlash.Phase)
	assert.Equal(t, uint16(0x1234), m.esp.server.port)
	assert.Equal(t, []byte("test"), m.esp.server.hostname)
	m.Write(0x8000, 0)
	assert.Equal(t, byte(0), m.prgROM[0])
	m.ClockCPU(1)
	assert.Equal(t, byte(0x80), m.Read(regESPStatus))
	assert.True(t, m.cycleIRQ.lineActive)
}

func TestStateRejectsDataBeforeChangingMemory(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	var saved bytes.Buffer
	assert.NoError(t, m.SaveState(&saved))
	m.prgROM[0] = 0xAB
	assert.Error(t, m.LoadState(bytes.NewReader(saved.Bytes()[:saved.Len()-1])))
	assert.Equal(t, byte(0xAB), m.prgROM[0])
	wrong := newTestMapper(t, 0x8000, 0x2000)
	wrong.Cartridge().PRG[0] ^= 0xFF
	assert.Error(t, wrong.LoadState(bytes.NewReader(saved.Bytes())))
	m.fpgaAutoAddr = testUint16Maximum
	saved.Reset()
	assert.NoError(t, m.SaveState(&saved))
	m.fpgaAutoAddr = 1
	assert.Error(t, m.LoadState(bytes.NewReader(saved.Bytes())))
	assert.Equal(t, uint16(1), m.fpgaAutoAddr)
}

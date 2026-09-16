package rainbow

import (
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

func TestResetPreservesMemoryAndUnspecifiedRegisters(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	m.prgRAM[12], m.chrRAM[13], m.fpgaRAM[14] = 1, 2, 3
	m.highBanks[0], m.highBanks[1] = 7, 8
	m.chrBanks[0], m.chrBanks[1] = 9, 10
	m.oamLimit = 17
	m.nmiVectorEnabled, m.irqVectorEnabled = true, true
	m.Write(regCycleIRQReloadLower, 1)
	m.Write(regCycleIRQControl, 1)
	m.ClockCPU(1)
	m.Reset()
	assert.Equal(t, byte(1), m.prgRAM[12])
	assert.Equal(t, byte(2), m.chrRAM[13])
	assert.Equal(t, byte(3), m.fpgaRAM[14])
	assert.Equal(t, uint16(0), m.highBanks[0])
	assert.Equal(t, uint16(8), m.highBanks[1])
	assert.Equal(t, uint16(0), m.chrBanks[0])
	assert.Equal(t, uint16(10), m.chrBanks[1])
	assert.Equal(t, byte(17), m.oamLimit)
	assert.False(t, m.nmiVectorEnabled)
	assert.False(t, m.irqVectorEnabled)
	assert.False(t, m.cycleIRQ.lineActive)
	assert.Equal(t, byte(135), m.scanIRQ.offset)
}

// These expected values come from the board's power-up/reset table. Keep them
// independent of resetRegisters so an omitted reset write causes a failure.
func TestDocumentedPowerUpAndResetValues(t *testing.T) {
	for _, reset := range []bool{false, true} {
		m := newTestMapper(t, 0x8000, 0x2000)
		if reset {
			for _, address := range []uint16{
				regPRGControl, regHighBankUpperStart, regHighBankLowerStart, regCHRControl, regCHRBankUpperStart, regCHRBankLowerStart,
				regNTBankStart, regNTBankStart + 1, regNTBankStart + 2, regNTBankEnd, regNTWindowBank,
				regNTControlStart, regNTControlStart + 1, regNTControlStart + 2, regNTControlEnd, regNTWindowControl,
				regOAMSlowPage, regOAMExtendedPage, regScanIRQControl, regScanIRQOffset, regCycleIRQControl,
				regVectorControl, regESPControl,
			} {
				m.Write(address, 0xFF)
			}
			m.scanIRQ.pending, m.cycleIRQ.pending = true, true
			m.Reset()
		}

		assert.Equal(t, byte(0), m.Read(regPRGControl))
		assert.Equal(t, uint16(0), m.highBanks[0])
		assert.Equal(t, byte(0), m.Read(regCHRControl))
		assert.Equal(t, uint16(0), m.chrBanks[0])
		assert.Equal(t, [5]byte{0, 0, 1, 1, 0}, m.ntBank)
		assert.Equal(t, [5]byte{0, 0, 0, 0, 0x80}, m.ntControl)
		assert.Equal(t, byte(7), m.oamSlowPage)
		assert.Equal(t, byte(6), m.oamExtPage)
		assert.False(t, m.scanIRQ.enabled)
		assert.False(t, m.scanIRQ.pending)
		assert.Equal(t, byte(135), m.scanIRQ.offset)
		assert.False(t, m.cycleIRQ.enabled)
		assert.False(t, m.cycleIRQ.enableAfterAck)
		assert.False(t, m.cycleIRQ.ackOn4011)
		assert.False(t, m.cycleIRQ.pending)
		assert.False(t, m.nmiVectorEnabled)
		assert.False(t, m.irqVectorEnabled)
		assert.Equal(t, byte(0), m.Read(regESPControl))
	}
}

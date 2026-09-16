package rainbow

// Specification: https://github.com/BrokeStudio/rainbow-net/blob/master/NES/mapper-doc.md#power-up-and-reset-register-status

// Reset applies the register reset values and preserves RAM and flash contents.
// The board resets selected bank registers, not every bank or OAM limit register.
func (m *Mapper) Reset() {
	for _, register := range resetRegisters {
		m.Write(register.address, register.value)
	}
	m.oamCodeLocked = false
	m.bgExtActive = false
	m.activeSpriteIndex = -1
	m.scanIRQ.inFrame, m.scanIRQ.inHBlank = false, false
	m.scanIRQ.counter = 0
	m.ppuBus = ppuBusState{enabled: m.ppuBus.enabled}
	m.prgFlash, m.chrFlash = flash{}, flash{}
	m.updateIRQStatus()
}

var resetRegisters = [...]struct {
	address uint16
	value   byte
}{
	{regPRGControl, 0},
	{regHighBankUpperStart, 0},
	{regHighBankLowerStart, 0},
	{regCHRControl, 0},
	{regCHRBankUpperStart, 0},
	{regCHRBankLowerStart, 0},
	{regNTBankStart, 0},
	{regNTBankStart + 1, 0},
	{regNTBankStart + 2, 1},
	{regNTBankEnd, 1},
	{regNTWindowBank, 0},
	{regNTControlStart, 0},
	{regNTControlStart + 1, 0},
	{regNTControlStart + 2, 0},
	{regNTControlEnd, 0},
	{regNTWindowControl, ntWindowSource},
	{regOAMSlowPage, oamDefaultSlowPage},
	{regOAMExtendedPage, oamDefaultExtendedPage},
	{regScanIRQAcknowledge, 0},
	{regScanIRQOffset, scanIRQDefaultOffset},
	{regCycleIRQControl, 0},
	{regVectorControl, 0},
	{regESPControl, 0},
}

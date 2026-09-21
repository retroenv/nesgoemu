package rainbow

import "github.com/retroenv/nesgoemu/pkg/feature"

const (
	registerLowByteMask  = 0x00FF
	registerHighByteMask = 0xFF00
	registerByteShift    = 8

	prgModeMask     = 0x07
	prgRAMModeShift = 7
	prgRAMModeMask  = 0x01

	chrModeMask        = 0x07
	chrWindowEnableBit = 0x10
	chrSpriteExtBit    = 0x20
	chrSourceShift     = 6

	vectorNMIEnableBit = 0x01
	vectorIRQEnableBit = 0x02
)

func (m *Mapper) readRegister(address uint16) uint8 {
	switch {
	case address == regPRGControl:
		return m.prgMode | m.prgRAMMode<<7

	case address == regCHRControl:
		return m.readCHRControlReg()

	case address >= regScanIRQLatch && address <= regCycleIRQParity:
		return m.readIRQRegister(address)

	case address == regFPGAAutoData:
		return m.readAutoData()

	case address == regPlatformVersion:
		return platformEmulatorV

	case address == regIRQStatus:
		return m.readCombinedIRQStatus()

	case address >= regNTBankStart && address <= regNTWindowControl:
		return m.readNTRegister(address)

	case address >= regESPControl && address <= regESPStart:
		return m.readESPRegisters(address)

	default:
		return 0
	}
}

func (m *Mapper) writeRegister(address uint16, value uint8) {
	switch {
	case address <= regHighBankLowerEnd:
		m.writePRGRegister(address, value)

	case address <= regCHRBankLowerEnd:
		m.writeCHRAndNTRegister(address, value)

	case address <= regFPGAAutoData:
		m.writeIRQAndFPGARegister(address, value)

	case address >= regPlatformVersion:
		m.writeExtRegister(address, value)
	}
}

func (m *Mapper) writePRGRegister(address uint16, value uint8) {
	switch {
	case address == regPRGControl:
		m.writePRGControl(value)

	case address >= regLowBankUpperStart && address <= regLowBankUpperEnd:
		// Low bank upper bytes ($6000-$7FFF).
		m.MarkFeature(feature.PRGBanking)
		idx := address - regLowBankUpperStart
		m.lowBanks[idx] = (m.lowBanks[idx] & registerLowByteMask) | uint16(value)<<registerByteShift

	case address >= regHighBankUpperStart && address <= regHighBankUpperEnd:
		// High bank upper bytes ($8000-$FFFF).
		m.MarkFeature(feature.PRGBanking)
		idx := address - regHighBankUpperStart
		m.highBanks[idx] = (m.highBanks[idx] & registerLowByteMask) | uint16(value)<<registerByteShift

	case address == regFPGABankSelect:
		m.fpgaBankSelect = value & fpgaBankMask

	case address >= regLowBankLowerStart && address <= regLowBankLowerEnd:
		// Low bank lower bytes ($6000-$7FFF).
		m.MarkFeature(feature.PRGBanking)
		idx := address - regLowBankLowerStart
		m.lowBanks[idx] = (m.lowBanks[idx] & registerHighByteMask) | uint16(value)

	case address >= regHighBankLowerStart && address <= regHighBankLowerEnd:
		// High bank lower bytes ($8000-$FFFF).
		m.MarkFeature(feature.PRGBanking)
		idx := address - regHighBankLowerStart
		m.highBanks[idx] = (m.highBanks[idx] & registerHighByteMask) | uint16(value)
	}
}

func (m *Mapper) writeCHRAndNTRegister(address uint16, value uint8) {
	switch {
	case address == regCHRControl:
		m.writeCHRControl(value)

	case address == regBGExtModeOffset:
		m.bgExtModeOffset = value & bgExtModeMask

	case address == regFillTile:
		m.fillTile = value

	case address == regFillAttribute:
		m.fillAttr = value & ntPaletteMask

	case address >= regNTBankStart && address <= regNTBankEnd:
		m.MarkFeature(feature.NameTableControl)
		m.ntBank[address-regNTBankStart] = value

	case address >= regNTControlStart && address <= regNTControlEnd:
		m.MarkFeature(feature.NameTableControl)
		m.ntControl[address-regNTControlStart] = value

	case address == regNTWindowBank:
		m.MarkFeature(feature.NameTableControl)
		m.ntBank[ntWindowSlot] = value

	case address == regNTWindowControl:
		m.MarkFeature(feature.NameTableControl)
		m.ntControl[ntWindowSlot] = (value &^ ntSrcMask) | ntWindowSource

	case address >= regCHRBankUpperStart && address <= regCHRBankUpperEnd:
		// CHR bank upper bytes.
		m.MarkFeature(feature.CHRBanking)
		idx := address - regCHRBankUpperStart
		m.chrBanks[idx] = (m.chrBanks[idx] & registerLowByteMask) | uint16(value)<<registerByteShift

	case address >= regCHRBankLowerStart && address <= regCHRBankLowerEnd:
		// CHR bank lower bytes.
		m.MarkFeature(feature.CHRBanking)
		idx := address - regCHRBankLowerStart
		m.chrBanks[idx] = (m.chrBanks[idx] & registerHighByteMask) | uint16(value)
	}
}

func (m *Mapper) writeIRQAndFPGARegister(address uint16, value uint8) {
	switch {
	case address >= regScanIRQLatch && address <= regScanIRQOffset:
		m.writeScanlineIRQReg(address, value)

	case address == regCycleIRQParity:
		m.cycleIRQ.parityReset = true

	case address >= regCycleIRQReloadUpper && address <= regCycleIRQAcknowledge:
		m.writeCycleIRQReg(address, value)

	case address == regFPGAAutoAddressUpper:
		// FPGA auto-address high bits [12:8], masked to 5 bits.
		m.fpgaAutoAddr = (m.fpgaAutoAddr & registerLowByteMask) |
			uint16(value&fpgaAutoAddressUpperMask)<<registerByteShift

	case address == regFPGAAutoAddressLower:
		// FPGA auto-address low byte [7:0].
		m.fpgaAutoAddr = (m.fpgaAutoAddr & registerHighByteMask) | uint16(value)

	case address == regFPGAAutoIncrement:
		m.fpgaAutoInc = value

	case address == regFPGAAutoData:
		m.writeAutoData(value)
	}
}

func (m *Mapper) writeExtRegister(address uint16, value uint8) {
	if address >= regSpriteBankLowerStart && address <= regSpriteBankUpper {
		m.MarkFeature(feature.SpriteExtendedMode)
	}

	switch {
	case address >= regVectorControl && address <= regIRQVectorLower:
		m.writeVectorRegisters(address, value)

	case address >= regWindowSplitStart && address <= regWindowSplitEnd:
		m.writeWindowSplit(address, value)

	case address >= regESPControl && address <= regESPTransmitPage:
		m.writeESPRegister(address, value)

	case address >= regSpriteBankLowerStart && address <= regSpriteBankLowerEnd:
		m.spriteBankLower[address-regSpriteBankLowerStart] = value

	case address == regSpriteBankUpper:
		m.spriteBankUpper = value
	case address >= regOAMSlowPage && address <= regOAMLimit:
		m.writeOAMRegister(address, value)
	}
}

func (m *Mapper) writePRGControl(value uint8) {
	m.MarkFeature(feature.PRGBanking)

	m.prgMode = value & prgModeMask
	m.prgRAMMode = (value >> prgRAMModeShift) & prgRAMModeMask
}

func (m *Mapper) readCHRControlReg() uint8 {
	v := m.chrMode
	if m.windowEnabled {
		v |= chrWindowEnableBit
	}
	if m.spriteExtMode {
		v |= chrSpriteExtBit
	}
	return v | m.chrSource<<chrSourceShift
}

func (m *Mapper) writeCHRControl(value uint8) {
	m.MarkFeature(feature.CHRBanking)
	if value>>chrSourceShift&chrSourceMask != chrSourceROM {
		m.MarkFeature(feature.CHRSourceSelect)
	}
	if value&chrSpriteExtBit != 0 {
		m.MarkFeature(feature.SpriteExtendedMode)
	}

	m.chrMode = value & chrModeMask
	m.windowEnabled = value&chrWindowEnableBit != 0
	m.spriteExtMode = value&chrSpriteExtBit != 0
	m.chrSource = (value >> chrSourceShift) & chrSourceMask
}

func (m *Mapper) writeVectorRegisters(address uint16, value uint8) {
	m.MarkFeature(feature.VectorRedirection)

	switch address {
	case regVectorControl:
		m.nmiVectorEnabled = value&vectorNMIEnableBit != 0
		m.irqVectorEnabled = value&vectorIRQEnableBit != 0
	case regNMIVectorUpper:
		m.nmiAddr = (m.nmiAddr & registerLowByteMask) | uint16(value)<<registerByteShift
	case regNMIVectorLower:
		m.nmiAddr = (m.nmiAddr & registerHighByteMask) | uint16(value)
	case regIRQVectorUpper:
		m.irqAddr = (m.irqAddr & registerLowByteMask) | uint16(value)<<registerByteShift
	case regIRQVectorLower:
		m.irqAddr = (m.irqAddr & registerHighByteMask) | uint16(value)
	}
}

func (m *Mapper) writeWindowSplit(address uint16, value uint8) {
	m.MarkFeature(feature.WindowSplit)

	idx := address - regWindowSplitStart
	if idx == windowSplitXStartIndex || idx == windowSplitXEndIndex || idx == windowSplitXScrollIndex {
		value &= ntTileCols - 1
	}
	m.windowSplitRegs[idx] = value
}

// Scanline IRQ registers: $4150-$4154.
// $4150 W: Set target scanline (8-bit latch).
// $4151 W: Enable scanline IRQ.
// $4151 R: Status (bit 7=HBlank, bit 6=in-frame). Reading clears pending.
// $4152 W: Disable IRQ + acknowledge pending.
// $4153 W: Set dot offset (clamped to 1-170).
// $4154 R: Jitter counter.

func (m *Mapper) readScanlineIRQStatus() uint8 {
	var v byte
	if m.scanIRQ.inHBlank {
		v |= irqStatusHBlankBit
	}
	if m.scanIRQ.inFrame {
		v |= irqStatusInFrameBit
	}
	m.scanIRQ.pending = false
	m.updateIRQStatus()
	return v
}

func (m *Mapper) writeScanlineIRQReg(address uint16, value uint8) {
	m.MarkFeature(feature.ScanlineIRQ)

	switch address {
	case regScanIRQLatch:
		m.scanIRQ.latch = value

	case regScanIRQControl:
		m.scanIRQ.enabled = true
		m.updateIRQStatus()

	case regScanIRQAcknowledge:
		m.scanIRQ.enabled = false
		m.scanIRQ.pending = false
		m.scanIRQ.readyToFire = false
		m.updateIRQStatus()

	case regScanIRQOffset:
		m.scanIRQ.offset = max(scanIRQMinimumOffset, min(value, scanIRQMaximumOffset))
	}
}

// Latch writes do not change the counter. Enabling reloads it.
// Acknowledgement clears pending and copies the enable-after-ack bit to enable.
func (m *Mapper) writeCycleIRQReg(address uint16, value uint8) {
	m.MarkFeature(feature.CPUCycleIRQ)

	switch address {
	case regCycleIRQReloadUpper:
		m.cycleIRQ.reloadValue = (m.cycleIRQ.reloadValue & registerLowByteMask) |
			uint16(value)<<registerByteShift

	case regCycleIRQReloadLower:
		m.cycleIRQ.reloadValue = (m.cycleIRQ.reloadValue & registerHighByteMask) | uint16(value)

	case regCycleIRQControl:
		m.cycleIRQ.pending = false
		m.cycleIRQ.enabled = value&cycleIRQEnableBit != 0
		m.cycleIRQ.enableAfterAck = value&cycleIRQEnableAfterAckBit != 0
		m.cycleIRQ.ackOn4011 = value&cycleIRQAckOnPCMReadBit != 0
		if m.cycleIRQ.enabled {
			m.cycleIRQ.counter = m.cycleIRQ.reloadValue
		}
		m.updateIRQStatus()

	case regCycleIRQAcknowledge:
		m.ackCycleIRQ()
	}
}

func (m *Mapper) readCombinedIRQStatus() uint8 {
	var v byte
	if m.scanIRQ.pending {
		v |= irqStatusPendingScanlineBit
	}
	if m.cycleIRQ.pending {
		v |= irqStatusPendingCycleBit
	}
	if m.esp.received {
		v |= irqStatusReceivedESPBit
	}
	return v
}

func (m *Mapper) readESPConfig() uint8 {
	var v byte
	if m.espEnabled {
		v |= espEnableBit
	}
	if m.wifiIrqEnable {
		v |= espIRQEnableBit
	}
	return v
}

// readESPRegisters returns configuration and transfer status without acknowledgement.
func (m *Mapper) readESPRegisters(address uint16) uint8 {
	switch address {
	case regESPControl:
		return m.readESPConfig()
	case regESPStatus:
		var value byte
		if m.esp.received {
			value |= espStatusReceivedBit
		}
		if len(m.esp.queue) != 0 {
			value |= espStatusQueuedBit
		}
		return value
	case regESPStart:
		if m.esp.sent {
			return espStatusSentBit
		}
	}
	return 0
}

// readNTRegister handles reads from the nametable bank ($4126-$4129, $412E) and
// control ($412A-$412D, $412F) registers.
func (m *Mapper) readNTRegister(address uint16) uint8 {
	switch {
	case address >= regNTBankStart && address <= regNTBankEnd:
		return m.ntBank[address-regNTBankStart]
	case address == regNTWindowBank:
		return m.ntBank[ntWindowSlot]
	case address >= regNTControlStart && address <= regNTControlEnd:
		return m.ntControl[address-regNTControlStart]
	case address == regNTWindowControl:
		return m.ntControl[ntWindowSlot]
	}
	return 0
}

func (m *Mapper) readIRQRegister(address uint16) byte {
	switch address {
	case regScanIRQLatch:
		return byte(m.scanIRQ.counter)
	case regScanIRQControl:
		return m.readScanlineIRQStatus()
	case regScanIRQJitter:
		return m.scanIRQ.jitter
	case regCycleIRQParity:
		return m.cycleIRQ.parity
	default:
		return 0
	}
}

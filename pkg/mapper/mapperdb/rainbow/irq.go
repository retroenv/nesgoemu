package rainbow

// Scanline IRQ specification: https://github.com/BrokeStudio/rainbow-net/blob/master/NES/mapper-doc.md#scanlineppu-irq-4150-4154
// CPU IRQ specification: https://github.com/BrokeStudio/rainbow-net/blob/master/NES/mapper-doc.md#cpu-cycle-irq-4158-415b

const (
	ppuHBlankStartCycle  = ppuVisibleFetchEnd
	ppuLastVisibleLine   = ppuVisibleHeight - 1
	scanIRQMinimumOffset = 1
	scanIRQMaximumOffset = 170
	scanIRQDefaultOffset = 135
	cycleIRQParityBit    = 0x80

	irqStatusPendingScanlineBit = 0x80
	irqStatusPendingCycleBit    = 0x40
	irqStatusReceivedESPBit     = 0x01
	irqStatusHBlankBit          = 0x80
	irqStatusInFrameBit         = 0x40

	cycleIRQEnableBit         = 0x01
	cycleIRQEnableAfterAckBit = 0x02
	cycleIRQAckOnPCMReadBit   = 0x04
)

// ClockCPU advances the mapper by elapsed CPU clocks. The system calls it once
// per clock so PPU reads can occur between CPU clocks. Standalone callers can
// supply multiple clocks when no PPU transaction must occur between them.
func (m *Mapper) ClockCPU(cycles uint64) {
	for range cycles {
		m.clockPPUBus()
		m.cycleIRQ.parity ^= cycleIRQParityBit
		if m.cycleIRQ.parityReset {
			// Reset parity one M2 clock after the register write.
			m.cycleIRQ.parity = 0
			m.cycleIRQ.parityReset = false
		}
		m.scanIRQ.jitter++
		m.clockESP()

		// Zero holds. Underflow to $FFFF would create a spurious long IRQ period.
		if !m.cycleIRQ.enabled || m.cycleIRQ.counter == 0 {
			continue
		}

		m.cycleIRQ.counter--
		if m.cycleIRQ.counter != 0 {
			continue
		}

		m.cycleIRQ.pending = true

		m.cycleIRQ.counter = m.cycleIRQ.reloadValue

		m.updateIRQStatus()
	}
}

// TickPPU records position before the PPU fetches data. With bus timing enabled,
// read transactions determine scanline IRQs and HBlank. Otherwise, this callback
// supplies the earlier dot-based fallback for standalone mapper callers.
func (m *Mapper) TickPPU(cycle, scanLine int, rendering bool) {
	m.ppuCycle = cycle
	m.ppuScanLine = scanLine
	if m.ppuBus.enabled {
		if !rendering {
			m.ppuScanLine = ppuLastVisibleLine + 1
		}
		return
	}
	if !rendering {
		m.ppuScanLine = ppuLastVisibleLine + 1
		m.scanIRQ.inFrame = false
		m.scanIRQ.inHBlank = false
		m.scanIRQ.readyToFire = false
		m.bgExtActive = false
		return
	}

	switch {
	case cycle == 0:
		m.handleNewScanline(scanLine)
	case m.scanIRQ.readyToFire && cycle == int(m.scanIRQ.offset)*2-1:
		m.scanIRQ.readyToFire = false
		m.scanIRQ.pending = true
		m.updateIRQStatus()
	case cycle == ppuHBlankStartCycle && scanLine >= 0 && scanLine <= ppuLastVisibleLine:
		m.scanIRQ.inHBlank = true
	}
}

// ReadPCM acknowledges the cycle IRQ when enabled. Audio synthesis is not implemented.
func (m *Mapper) ReadPCM() byte {
	if m.cycleIRQ.ackOn4011 {
		m.ackCycleIRQ()
	}
	return 0
}

func (m *Mapper) ackCycleIRQ() {
	m.cycleIRQ.enabled = m.cycleIRQ.enableAfterAck
	m.cycleIRQ.pending = false
	m.updateIRQStatus()
}

func (m *Mapper) handleNewScanline(scanLine int) {
	m.scanIRQ.inHBlank = false
	m.scanIRQ.inFrame = scanLine >= 0 && scanLine <= ppuLastVisibleLine
	m.scanIRQ.counter = int16(scanLine)
	m.scanIRQ.readyToFire = false // clear any stale state from previous scanline

	if !m.scanIRQ.inFrame || m.scanIRQ.latch == 0 {
		return
	}

	if m.scanIRQ.counter == int16(m.scanIRQ.latch) {
		m.scanIRQ.readyToFire = true // fires at dot offset (TickPPU)
	}
}

// updateIRQStatus combines sources without acknowledging any of them. Jitter
// measures time since line assertion. A second pending source does not restart it.
func (m *Mapper) updateIRQStatus() {
	active := (m.cycleIRQ.enabled && m.cycleIRQ.pending) || (m.scanIRQ.enabled && m.scanIRQ.pending) ||
		(m.espEnabled && m.wifiIrqEnable && m.esp.received)
	if active && !m.cycleIRQ.lineActive {
		m.scanIRQ.jitter = 0
	}
	m.cycleIRQ.lineActive = active
	m.SetMapperIRQ(active)
}

type scanlineIRQState struct {
	enabled     bool
	pending     bool
	readyToFire bool  // latch matched; IRQ fires at dot offset
	latch       byte  // target scanline (8-bit)
	offset      byte  // dot offset (1-170)
	inFrame     bool  // true during rendering scanlines (0-239)
	inHBlank    bool  // true when dot >= 256
	jitter      byte  // M2 cycles since last IRQ
	counter     int16 // current rendering scanline
}

type cycleIRQState struct {
	lineActive     bool
	parityReset    bool
	parity         byte
	enabled        bool
	pending        bool
	counter        uint16
	reloadValue    uint16
	enableAfterAck bool // auto-reload after acknowledge
	ackOn4011      bool // reading $4011 auto-acknowledges
}

package rainbow

import "github.com/retroenv/nesgoemu/pkg/feature"

const (
	ppuBusRepeatedReadCount    = 2
	ppuBusIdleClockCount       = 3
	ppuBusHBlankTileCount      = 33
	ppuBusWindowFetchCount     = 50
	ppuBusLastCurrentLineFetch = 49
)

// EnableBusTiming selects scanline detection from PPU read transactions.
func (m *Mapper) EnableBusTiming() {
	m.ppuBus = ppuBusState{enabled: true}
	m.scanIRQ.inFrame = false
	m.scanIRQ.counter = -1
	m.scanIRQ.readyToFire = false
}

// observePPURead detects the three-read sequence across a scanline boundary.
// A different address breaks the sequence. The first match starts line zero.
func (m *Mapper) observePPURead(address uint16) {
	if !m.ppuBus.enabled {
		return
	}
	m.MarkFeature(feature.PPUBusTiming)
	m.ppuBus.reads++
	if address >= ntBaseAddress && address < ntBaseAddress+ntMirrorSize && address == m.ppuBus.lastAddress {
		m.ppuBus.repeated++
		if m.ppuBus.repeated == ppuBusRepeatedReadCount {
			m.startBusScanline()
		}
	} else {
		m.ppuBus.repeated = 0
	}
	if m.scanIRQ.inFrame && m.scanIRQ.counter == int16(m.scanIRQ.latch) && m.ppuBus.reads == uint16(m.scanIRQ.offset) {
		m.scanIRQ.pending = true
		m.updateIRQStatus()
	}
	m.ppuBus.idle = ppuBusIdleClockCount
	m.ppuBus.lastAddress = address
	if m.scanIRQ.inFrame && address >= ntBaseAddress && address < ntBaseAddress+ntMirrorSize &&
		address&(ntSlotSize-1) < ntAttrOffset {

		m.ppuBus.tiles++
		if m.ppuBus.tiles == ppuBusHBlankTileCount {
			m.scanIRQ.inHBlank = true
		}
	}
}

func (m *Mapper) startBusScanline() {
	if !m.scanIRQ.inFrame {
		m.scanIRQ.counter = 0
	} else {
		m.scanIRQ.counter++
	}
	m.scanIRQ.inFrame = true
	m.scanIRQ.inHBlank = false
	m.ppuBus.reads, m.ppuBus.tiles, m.ppuBus.repeated = 0, 0, 0
}

// clockPPUBus detects rendering stop from an absence of reads for three M2 clocks.
// It preserves pending IRQs. Stopping rendering does not acknowledge an IRQ.
func (m *Mapper) clockPPUBus() {
	if m.ppuBus.idle == 0 {
		return
	}
	m.ppuBus.idle--
	if m.ppuBus.idle == 0 {
		m.scanIRQ.inFrame, m.scanIRQ.inHBlank, m.bgExtActive = false, false, false
		m.scanIRQ.counter = -1
		m.ppuBus.tiles, m.ppuBus.repeated = 0, 0
	}
}

type ppuBusState struct {
	enabled     bool
	idle        byte
	repeated    byte
	lastAddress uint16
	reads       uint16
	tiles       int
}

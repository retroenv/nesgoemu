package memory

import "github.com/retroenv/retrogolib/arch/system/nes/register"

// AccessCounts counts the accesses that reach the CPU bus controller.
// Reads and Writes count accesses of the CPU. DMAReads counts reads that a
// DMA transfer makes, including the repeated reads of a halted CPU. The
// writes of an OAM transfer go to the PPU directly and are not counted.
type AccessCounts struct {
	Reads    uint64
	Writes   uint64
	DMAReads uint64

	// PPUWrites counts CPU writes to the PPU registers $2000-$2007 and their
	// mirrors, indexed by the register number 0-7.
	PPUWrites [8]uint64
	// OAMDMAWrites counts CPU writes to the OAM DMA register $4014.
	OAMDMAWrites uint64
}

// PPURegisterWrites returns the sum of PPUWrites and OAMDMAWrites.
func (c *AccessCounts) PPURegisterWrites() uint64 {
	total := c.OAMDMAWrites
	for _, count := range c.PPUWrites {
		total += count
	}
	return total
}

// CountAccesses replaces the optional counters for bus accesses.
// The emulation goroutine increments them. The caller reads and clears them
// on the same goroutine, for example in a frame callback. A nil pointer
// disables counting. The counters do not change emulation results.
func (m *Memory) CountAccesses(counts *AccessCounts) {
	m.counts = counts
}

// SetDMAAccess marks the reads that follow as DMA reads for CountAccesses.
// The system sets it around each DMA read and clears it afterward.
func (m *Memory) SetDMAAccess(active bool) {
	m.dmaAccess = active
}

func (m *Memory) countRead() {
	if m.dmaAccess {
		m.counts.DMAReads++
		return
	}
	m.counts.Reads++
}

func (m *Memory) countWrite(address uint16) {
	m.counts.Writes++
	switch {
	case address >= register.PPU_CTRL && address < register.APU_PL1_VOL:
		m.counts.PPUWrites[address&7]++
	case address == register.OAM_DMA:
		m.counts.OAMDMAWrites++
	}
}

package nes

import (
	"crypto/sha256"
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/memory"
	"github.com/retroenv/nesgoemu/pkg/ppu"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

// toggleProgram sets two palette entries, starts an OAM DMA from page 2 and
// then toggles PPUMASK between $1E and $00 in a loop of about 340 CPU cycles.
//
//	LDA #$3F / STA $2006 / LDA #$00 / STA $2006
//	LDA #$21 / STA $2007 / LDA #$16 / STA $2007
//	LDA #$02 / STA $4014
//	loop: LDA $10 / EOR #$1E / STA $2001 / STA $10
//	      LDX #$40 / DEX / BNE -3 / JMP loop
var toggleProgram = []byte{
	0xa9, 0x3f, 0x8d, 0x06, 0x20, 0xa9, 0x00, 0x8d, 0x06, 0x20,
	0xa9, 0x21, 0x8d, 0x07, 0x20, 0xa9, 0x16, 0x8d, 0x07, 0x20,
	0xa9, 0x02, 0x8d, 0x14, 0x40,
	0xa5, 0x10, 0x49, 0x1e, 0x8d, 0x01, 0x20, 0x85, 0x10,
	0xa2, 0x40, 0xca, 0xd0, 0xfd, 0x4c, 0x19, 0x80,
}

func newToggleSystem(t *testing.T) *System {
	t.Helper()

	cart := cartridge.New()
	cart.PRG = make([]byte, 32*1024)
	cart.CHR = make([]byte, 8*1024)
	for row := range 8 {
		cart.CHR[row] = 0xaa
	}
	copy(cart.PRG, toggleProgram)
	cart.PRG[0x7ffc] = 0x00
	cart.PRG[0x7ffd] = 0x80

	sys, err := NewSystem(NewOptions(WithCartridge(cart), WithSavePath("")))
	assert.NoError(t, err)
	return sys
}

// stepFrames runs whole instructions until frames more frames completed.
func stepFrames(t *testing.T, sys *System, frames uint64) {
	t.Helper()

	for done := uint64(0); done < frames; {
		step, err := sys.StepSystem()
		assert.NoError(t, err)
		if step.FrameCompleted {
			done++
		}
	}
}

func TestObserveFramesRecordsRealRun(t *testing.T) {
	sys := newToggleSystem(t)
	var records []FrameRecord
	assert.True(t, sys.ObserveFrames(func(record *FrameRecord) {
		records = append(records, record.Clone())
	}))

	stepFrames(t, sys, 6)

	assert.True(t, len(records) >= 5)
	assert.True(t, records[0].Partial)
	record := records[len(records)-1]
	assert.False(t, record.Partial)
	assert.Equal(t, records[len(records)-2].Frame+1, record.Frame)

	// The loop toggles PPUMASK every few scanlines inside the picture.
	assert.True(t, len(record.MaskChanges) > 50)
	assert.True(t, record.Rendered() > 0)
	assert.True(t, record.Rendered() < ppu.Width*ppu.Height)
	blank := record.OffSpans(ppu.MaskBoth)
	assert.True(t, len(blank) > 20)
	blankDots := 0
	for _, span := range blank {
		blankDots += span.End - span.Start
	}
	assert.True(t, blankDots > ppu.VisibleDots/4 && blankDots < ppu.VisibleDots*3/4)

	// Each loop writes PPUMASK once, and every write changes the enable
	// bits. It also writes $10 once. Every CPU cycle is one bus access, and
	// a frame period has about 29781 CPU cycles.
	accesses := record.Accesses
	assert.Equal(t, uint64(len(record.MaskChanges)), accesses.PPUWrites[1])
	assert.Equal(t, 2*accesses.PPUWrites[1], accesses.Writes)
	assert.Equal(t, accesses.PPUWrites[1], accesses.PPURegisterWrites())
	assert.Equal(t, uint64(0), accesses.DMAReads)
	total := accesses.Reads + accesses.Writes
	assert.True(t, total >= 29779 && total <= 29782)

	// The first period holds the setup: two palette writes, one DMA request.
	first := records[0]
	assert.Equal(t, uint64(1), first.Accesses.OAMDMAWrites)
	assert.Equal(t, uint64(2), first.Accesses.PPUWrites[7])
	assert.Equal(t, uint64(2), first.Accesses.PPUWrites[6])
	// The transfer reads 256 bytes, plus the halt and alignment reads.
	assert.True(t, first.Accesses.DMAReads >= 256 && first.Accesses.DMAReads <= 258)
}

func TestObserveFramesPictureMatchesEmulatorImage(t *testing.T) {
	sys := newToggleSystem(t)
	var record FrameRecord
	sys.ObserveFrames(func(r *FrameRecord) { record = r.Clone() })
	stepFrames(t, sys, 4)

	img := record.RGBA()
	shown := sys.Image()
	// The PPU swaps the buffers at the end of the picture, so the shown image
	// is the picture of the last record.
	checked := 0
	for y := range ppu.Height {
		if record.RenderedPixels[y] != ppu.Width {
			continue
		}
		checked++
		for x := range ppu.Width {
			assert.Equal(t, shown.RGBAAt(x, y), img.RGBAAt(x, y))
		}
	}
	assert.True(t, checked > 20)
	// The picture shows both palette entries.
	var seen [64]bool
	for _, index := range record.Indices {
		seen[index] = true
	}
	assert.True(t, seen[0x21])
	assert.True(t, seen[0x16])
}

func TestObserveFramesDoNotChangeEmulation(t *testing.T) {
	type result struct {
		cycles uint64
		frame  uint64
		pc     uint16
		image  [32]byte
		oam    [256]byte
		ram    [0x20]byte
	}
	run := func(observe bool) result {
		sys := newToggleSystem(t)
		var frames int
		if observe {
			sys.ObserveFrames(func(*FrameRecord) { frames++ })
		}
		stepFrames(t, sys, 8)
		if observe {
			assert.True(t, frames >= 7)
		}
		var res result
		res.cycles = sys.CPU.Cycles()
		res.frame = sys.Bus.PPU.Frame()
		res.pc = sys.PC
		res.image = sha256.Sum256(sys.Image().Pix)
		res.oam = sys.InspectOAM()
		for address := range res.ram {
			res.ram[address], _ = sys.InspectRAM(uint16(address))
		}
		return res
	}

	assert.Equal(t, run(false), run(true))
}

func TestObserveFramesRunKeepsResults(t *testing.T) {
	run := func(observe bool) (RunResult, [32]byte) {
		sys := newToggleSystem(t)
		if observe {
			sys.ObserveFrames(func(*FrameRecord) {})
		}
		result := sys.Run(t.Context(), RunOptions{
			Frames:    5,
			MaxCycles: 1_000_000,
		})
		hash := sha256.Sum256(result.Image.Pix)
		result.Image = nil
		return result, hash
	}
	plain, plainHash := run(false)
	observed, observedHash := run(true)
	assert.Equal(t, ExitSuccess, plain.Reason)
	assert.Equal(t, plain, observed)
	assert.Equal(t, plainHash, observedHash)
}

func TestObserveFramesDisable(t *testing.T) {
	sys := newToggleSystem(t)
	calls := 0
	sys.ObserveFrames(func(*FrameRecord) { calls++ })
	stepFrames(t, sys, 3)
	seen := calls
	assert.True(t, seen > 0)

	assert.True(t, sys.ObserveFrames(nil))
	stepFrames(t, sys, 3)
	assert.Equal(t, seen, calls)
}

func TestObserveFramesNativeCountsCallerAccesses(t *testing.T) {
	n, err := NewNative(NativeOptions{CHR: make([]byte, 8192)})
	assert.NoError(t, err)
	var records []FrameRecord
	assert.True(t, n.ObserveFrames(func(r *FrameRecord) { records = append(records, r.Clone()) }))

	n.Write(0x2001, 0x1e)
	n.Write(0x2007, 0)
	n.Read(0x2002)
	n.Read(0x0000)
	n.Advance(30000)
	n.Write(0x4014, 2)
	n.Advance(1)
	n.Advance(30000)

	assert.True(t, len(records) >= 2)
	first := records[0]
	assert.True(t, first.Partial)
	assert.Equal(t, uint64(2), first.Accesses.Writes)
	assert.Equal(t, uint64(2), first.Accesses.Reads)
	assert.Equal(t, uint64(1), first.Accesses.PPUWrites[1])
	assert.Equal(t, uint64(1), first.Accesses.PPUWrites[7])
	var dmaWrites, dmaReads uint64
	for _, record := range records {
		dmaWrites += record.Accesses.OAMDMAWrites
		dmaReads += record.Accesses.DMAReads
	}
	assert.Equal(t, uint64(1), dmaWrites)
	assert.True(t, dmaReads >= 256 && dmaReads <= 258)
}

func TestObserveFramesUnsupportedPPU(t *testing.T) {
	sys := &System{Bus: &bus.Bus{PPU: &clockPPU{}}}
	assert.False(t, sys.ObserveFrames(func(*FrameRecord) {}))
}

func TestObserveFramesCountsDMCReads(t *testing.T) {
	// Enable a one-byte DMC sample, then repeat NOP and JMP.
	sys := newAudioTestSystem(t, []byte{0xa9, 0x10, 0x8d, 0x15, 0x40, 0xea, 0x4c, 0x05, 0x80})
	var records []FrameRecord
	assert.True(t, sys.ObserveFrames(func(record *FrameRecord) {
		records = append(records, record.Clone())
	}))
	stepFrames(t, sys, 3)

	// The DMC load uses a halt read, a dummy read, and the sample read.
	// None of these accesses belongs in the CPU read count.
	assert.Len(t, records, 2)
	assert.Equal(t, uint64(3), records[0].Accesses.DMAReads)
	assert.Equal(t, uint64(1), records[0].Accesses.Writes)
	assert.Equal(t, uint64(0), records[1].Accesses.DMAReads)
}

func TestObserveFramesCountsBoundaryAccessInNextPeriod(t *testing.T) {
	n, err := NewNative(NativeOptions{})
	assert.NoError(t, err)
	var records []FrameRecord
	assert.True(t, n.ObserveFrames(func(record *FrameRecord) {
		records = append(records, record.Clone())
	}))

	// Position the PPU one dot before capture. The next device cycle ends
	// the period before the CPU write reaches memory.
	const dotsPerFrame = ppu.DotsPerScanline * 262
	n.bus.PPU.Step(1)
	assert.Empty(t, records)
	n.Advance(1)
	n.Write(0x2001, 0x1e)
	assert.Len(t, records, 1)
	assert.Equal(t, memory.AccessCounts{}, records[0].Accesses)

	n.bus.PPU.Step(dotsPerFrame)
	assert.Len(t, records, 2)
	assert.Equal(t, uint64(1), records[1].Accesses.Writes)
	assert.Equal(t, uint64(1), records[1].Accesses.PPUWrites[1])
	assert.Len(t, records[1].MaskChanges, 1)
}

package nes

import (
	"bytes"
	"fmt"
	"image"

	"github.com/retroenv/nesgoemu/pkg/apu"
	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/controller"
	"github.com/retroenv/nesgoemu/pkg/memory"
	"github.com/retroenv/nesgoemu/pkg/ppu"
	"github.com/retroenv/nesgoemu/pkg/ppu/screen"
	"github.com/retroenv/retrogolib/arch/cpu/cpu6502"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/audio"
	"github.com/retroenv/retrogolib/gui"
	"github.com/retroenv/retrogolib/input"
)

// nativeMinPRGData is the smallest PRG image, one full $8000-$FFFF window.
const nativeMinPRGData = 32 * 1024

// NativeOptions selects a source-mode board and source-derived assets.
// Any mapper that the emulator implements is accepted. PRG contains data only;
// it is padded to a power of two of at least 32 KiB. Empty CHR selects 8 KiB
// CHR-RAM unless NES2 sets the RAM sizes.
type NativeOptions struct {
	Mapper uint16
	// Mirror selects one of the named cartridge nametable layouts.
	Mirror  cartridge.MirrorMode
	CHR     []byte
	PRGData []byte
	// Battery marks nonvolatile PRG-RAM. Source mode keeps it in memory only.
	Battery byte
	// NES2 supplies board RAM sizes for mappers that read them.
	NES2 *cartridge.NES2Metadata
}

type nativeClock struct {
	cycles uint64
	stalls uint64
	nmi    bool
	irq    bool
}

// Native clocks the existing devices without a CPU interpreter or reset vector.
// All device access, input, and clock operations have one goroutine owner.
// Only AudioCallback can run on another goroutine.
type Native struct {
	bus    *bus.Bus
	memory *memory.Memory
	apu    *apu.APU
	dma    dmaController
	clock  nativeClock
}

// NewNative creates devices from a board and assets without loading a ROM file.
func NewNative(opts NativeOptions) (*Native, error) {
	if !supportedNativeMirror(opts.Mirror) {
		return nil, fmt.Errorf("unsupported native mirroring mode: %d", opts.Mirror)
	}

	// The mapper's data window has no program, header, or interrupt vectors.
	data := make([]byte, nativePRGSize(len(opts.PRGData)))
	copy(data, opts.PRGData)
	cart := &cartridge.Cartridge{
		Mapper:  opts.Mapper,
		Mirror:  opts.Mirror,
		CHR:     bytes.Clone(opts.CHR),
		PRG:     data,
		Battery: opts.Battery,
	}
	if opts.NES2 != nil {
		metadata := *opts.NES2
		cart.NES2 = &metadata
	}

	// The CPU-memory wrapper stays in bus.Memory; no CPU interpreter is created.
	b, m, _, err := initializeDevices(cart)
	if err != nil {
		return nil, fmt.Errorf("unsupported native board: %w", err)
	}

	n := &Native{
		bus:    b,
		memory: m,
	}
	b.CPU = &n.clock
	b.DMA = &n.dma
	n.apu = apu.New(b)
	b.APU = n.apu
	b.PPU = ppu.New(b)

	return n, nil
}

// Advance clocks devices and completes queued DMA before source execution resumes.
// The cycle count is a cooperative source-mode budget, not target Go timing.
func (n *Native) Advance(cycles uint64) {
	for cycles > 0 || n.dma.oamPending || n.dma.halted || n.clock.stalls > 0 {
		if cycles > 0 {
			cycles--
		}

		if n.clock.stalls > 0 {
			n.clock.stalls--
		}

		n.clock.cycles++
		clockDeviceCycle(n.bus, n.apu, &n.dma, n.memory, n.clock.cycles, cpu6502.BusCycle{})
	}
}

// Read accesses the emulated bus. The caller advances time between operations.
func (n *Native) Read(address uint16) byte { return n.bus.Memory.Read(address) }

// Write accesses the emulated bus. Advance completes queued DMA after the write.
func (n *Native) Write(address uint16, value byte) { n.bus.Memory.Write(address, value) }

// TakeNMI clears and returns one pending NMI edge. Multiple pending edges coalesce.
func (n *Native) TakeNMI() bool {
	pending := n.clock.nmi
	n.clock.nmi = false
	return pending
}

// IRQActive reports the IRQ line without acknowledging it or advancing clocks.
// Go IRQ delivery is not implemented; callers must not infer it from this query.
func (n *Native) IRQActive() bool { return n.clock.irq }

// Cycles returns the device clock count. No 6502 instructions contribute to it.
func (n *Native) Cycles() uint64 { return n.clock.cycles }

// Frame returns the device frame counter.
func (n *Native) Frame() uint64 { return n.bus.PPU.Frame() }

// Image returns the live framebuffer for the synchronous renderer.
func (n *Native) Image() *image.RGBA { return n.bus.PPU.Image() }

// Dimensions returns the NES display dimensions.
func (n *Native) Dimensions() gui.Dimensions {
	return gui.Dimensions{
		Width:       screen.Width,
		Height:      screen.Height,
		ScaleFactor: 2,
	}
}

// WindowTitle returns the native runtime title.
func (n *Native) WindowTitle() string { return "NES Go source mode" }

// KeyDown sets a controller button on the owner goroutine.
func (n *Native) KeyDown(key input.Key) {
	setControllerKey(n.bus.Controller1, key, true)
}

// KeyUp clears a controller button on the owner goroutine.
func (n *Native) KeyUp(key input.Key) {
	setControllerKey(n.bus.Controller1, key, false)
}

// SetButtons replaces controller-one state for source-mode replay.
func (n *Native) SetButtons(buttons byte) {
	n.bus.Controller1.SetButtonState(controller.Button(0xff), false)
	n.bus.Controller1.SetButtonState(controller.Button(buttons), true)
}

// InspectRAM reads RAM without bus side effects or clock progress.
func (n *Native) InspectRAM(address uint16) (byte, bool) { return n.memory.InspectRAM(address) }

// InspectPPU returns a byte of PPU address space without bus effects. It
// reports false when the board cannot supply it. See ppu.PPU.InspectMemory.
func (n *Native) InspectPPU(address uint16) (byte, bool) {
	inspector, ok := n.bus.PPU.(bus.PPUMemoryInspector)
	if !ok {
		return 0, false
	}

	return inspector.InspectMemory(address)
}

// InspectPRGRAM returns a byte of the physical PRG-RAM array without bus
// effects, volatile bytes first. It reports false when the mapper does not
// expose its PRG-RAM or the offset is outside the array.
func (n *Native) InspectPRGRAM(offset int) (byte, bool) {
	inspector, ok := n.bus.Mapper.(bus.MemoryInspector)
	if !ok {
		return 0, false
	}

	return inspector.InspectPRGRAM(offset)
}

// InspectOAM returns copied OAM for source-mode conformance checks.
func (n *Native) InspectOAM() [256]byte { return n.bus.PPU.OAM() }

// AudioFormat selects the existing APU output format.
func (n *Native) AudioFormat() audio.Format {
	return nesAudioFormat()
}

// AudioCallback drains the thread-safe APU output queue. It does not clock devices.
func (n *Native) AudioCallback(buffer []byte) { n.apu.FillSamples(buffer) }

// AudioPaused permits playback and silence when the queue is empty.
func (n *Native) AudioPaused() bool { return false }

// DrainAudio returns the number of queued PCM sample frames read without clocking.
func (n *Native) DrainAudio(buffer []byte) int { return n.apu.DrainSamples(buffer) }

// MustNewNative creates devices for fixed startup configuration and panics on error.
//
//nolint:funcorder // The two linters classify Must constructors differently.
func MustNewNative(opts NativeOptions) *Native {
	device, err := NewNative(opts)
	if err != nil {
		panic(fmt.Errorf("creating native NES devices: %w", err))
	}

	return device
}

func (c *nativeClock) Cycles() uint64            { return c.cycles }
func (c *nativeClock) StallCycles(cycles uint16) { c.stalls += uint64(cycles) }
func (c *nativeClock) State() cpu6502.State      { return cpu6502.State{Cycles: c.cycles} }
func (c *nativeClock) SetIRQ(active bool)        { c.irq = active }
func (c *nativeClock) TriggerIrq()               { c.irq = true }
func (c *nativeClock) TriggerNMI()               { c.nmi = true }

// nativePRGSize keeps bank masks valid for mappers that wrap PRG by size.
func nativePRGSize(size int) int {
	padded := nativeMinPRGData
	for padded < size {
		padded *= 2
	}

	return padded
}

func supportedNativeMirror(mode cartridge.MirrorMode) bool {
	switch mode {
	case cartridge.MirrorHorizontal, cartridge.MirrorVertical,
		cartridge.MirrorSingle0, cartridge.MirrorSingle1, cartridge.Mirror4,
		cartridge.MirrorDiagonal, cartridge.MirrorLShaped,
		cartridge.Mirror3Vertical, cartridge.Mirror3Horizontal, cartridge.Mirror3Diagonal:
		return true
	default:
		return false
	}
}

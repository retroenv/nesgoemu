package nes

import (
	"fmt"
	"io"
	"slices"
	"sync"

	"github.com/retroenv/nesgoemu/pkg/apu"
	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/ppu"
	"github.com/retroenv/retrogolib/arch/cpu/cpu6502"
	"github.com/retroenv/retrogolib/arch/system/nes"
	"github.com/retroenv/retrogolib/arch/system/nes/codedatalog"
)

// coverageSegmentSize is the PRG data size in one coverage segment.
const coverageSegmentSize = 0x2000

// CDLCoverage reports the code and data coverage of one 8 KiB PRG segment.
type CDLCoverage struct {
	Code        int     `json:"code"`
	CodePercent float64 `json:"code_percent"`
	Data        int     `json:"data"`
	DataPercent float64 `json:"data_percent"`
	Segment     int     `json:"segment"`
	Size        int     `json:"size"`
}

// CDLRecorder records FCEUX-compatible code and data logging flags for a
// system. The flags cover the PRG ROM and the CHR ROM. Start installs runtime
// observers. Stop removes them.
type CDLRecorder struct {
	sys *System

	mu sync.Mutex

	chr []codedatalog.ChrFlag
	prg []codedatalog.PrgFlag

	active bool
	remove []func()

	indirect bool
}

// NewCDLRecorder creates a recorder for the system in the stopped state.
func (sys *System) NewCDLRecorder() *CDLRecorder {
	cart := sys.Bus.Cartridge
	return &CDLRecorder{
		sys: sys,

		chr: make([]codedatalog.ChrFlag, len(cart.CHR)),
		prg: make([]codedatalog.PrgFlag, len(cart.PRG)),
	}
}

// Start begins recording. Repeated calls have no effect.
// Call Start while emulation is stopped.
func (r *CDLRecorder) Start() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.active {
		return
	}
	r.active = true
	r.remove = append(r.remove,
		r.sys.AddPreExecutionHook(r.observeInstruction),
		r.sys.AddBusCycleObserver(r.observeCycle),
	)
	r.sys.apu.ObserveDMCFetches(r.observeDMCFetch)
	if device, ok := r.sys.Bus.PPU.(*ppu.PPU); ok {
		device.ObserveCHRReads(r.observeCHRRead)
	}
}

// Stop ends recording and removes the runtime hooks.
// Call Stop while emulation is stopped.
func (r *CDLRecorder) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.active = false
	for _, remove := range r.remove {
		remove()
	}

	r.remove = nil
	r.sys.apu.ObserveDMCFetches(nil)
	if device, ok := r.sys.Bus.PPU.(*ppu.PPU); ok {
		device.ObserveCHRReads(nil)
	}
}

// Reset clears the recorded flags.
func (r *CDLRecorder) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.prg = make([]codedatalog.PrgFlag, len(r.prg))
	r.chr = make([]codedatalog.ChrFlag, len(r.chr))
}

// Coverage reports the code and data coverage in 8 KiB PRG segments.
func (r *CDLRecorder) Coverage() []CDLCoverage {
	r.mu.Lock()
	defer r.mu.Unlock()

	return coverage(r.prg)
}

// PRG returns a copy of the PRG flags.
func (r *CDLRecorder) PRG() []codedatalog.PrgFlag {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]codedatalog.PrgFlag(nil), r.prg...)
}

// CHR returns a copy of the CHR flags.
func (r *CDLRecorder) CHR() []codedatalog.ChrFlag {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]codedatalog.ChrFlag(nil), r.chr...)
}

// PRGRange returns an independent copy of a physical PRG flag range.
func (r *CDLRecorder) PRGRange(offset, length int) ([]codedatalog.PrgFlag, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return copyCDLRange(r.prg, offset, length)
}

// CHRRange returns an independent copy of a physical CHR flag range.
func (r *CDLRecorder) CHRRange(offset, length int) ([]codedatalog.ChrFlag, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return copyCDLRange(r.chr, offset, length)
}

// Write writes the flags in the FCEUX layout: PRG flags then CHR flags.
func (r *CDLRecorder) Write(destination io.Writer) error {
	r.mu.Lock()
	prg := append([]codedatalog.PrgFlag(nil), r.prg...)
	chr := append([]codedatalog.ChrFlag(nil), r.chr...)
	r.mu.Unlock()

	if err := codedatalog.Write(destination, prg, chr); err != nil {
		return fmt.Errorf("writing CDL: %w", err)
	}
	return nil
}

// observeInstruction records the addressing mode and instruction entry targets.
func (r *CDLRecorder) observeInstruction(_ *cpu6502.CPU, _ *cpu6502.Instruction, _ ...any) {
	if !r.enabled() {
		return
	}

	pc := r.sys.PC
	opcodeByte, ok := r.sys.Peek(pc)
	if !ok {
		return
	}
	opcode := cpu6502.Opcodes[opcodeByte]
	if opcode.Instruction == nil {
		return
	}
	r.markEntry(opcode, pc)
	r.mu.Lock()
	r.indirect = indirectAddressing(opcode.Addressing)
	r.mu.Unlock()
}

// observeCycle records instruction fetches and data reads separately.
// Dummy reads and mapper register writes do not add data flags.
func (r *CDLRecorder) observeCycle(cycle cpu6502.BusCycle) {
	if !r.enabled() || cycle.Write || cycle.Kind == cpu6502.BusCycleDummy {
		return
	}
	if cycle.Kind == cpu6502.BusCycleOpcode || cycle.Kind == cpu6502.BusCycleOperand {
		r.markPRG(cycle.Address, codedatalog.Code)
		return
	}
	if cycle.Address < nes.CodeBaseAddress {
		return
	}

	r.mu.Lock()
	indirect := r.indirect
	r.mu.Unlock()

	flag := codedatalog.Data
	if indirect && cycle.Kind == cpu6502.BusCycleData {
		flag |= codedatalog.IndirectData
	}

	r.markPRG(cycle.Address, flag)
}

// observeDMCFetch marks a DMC sample read as data and PCM audio.
func (r *CDLRecorder) observeDMCFetch(fetch apu.DMCFetch) {
	if r.enabled() {
		r.markPRG(fetch.Address, codedatalog.Data|codedatalog.PCMAudio)
	}
}

// observeCHRRead records a physical pattern read in the current mapper context.
func (r *CDLRecorder) observeCHRRead(event ppu.CHRRead) {
	if event.Mapping.Memory != bus.GraphicsCHRROM {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	offset := event.Mapping.Offset
	if !r.active || offset < 0 || offset >= len(r.chr) {
		return
	}
	flag := codedatalog.ReadProgrammatically
	if event.Rendering {
		flag = codedatalog.DrawnOnScreen
	}

	r.chr[offset] |= flag
}

// markEntry marks the JSR and indirect JMP targets of an instruction.
func (r *CDLRecorder) markEntry(opcode cpu6502.Opcode, pc uint16) {
	switch opcode.Instruction.Name {
	case cpu6502.JsrName:
		if target, ok := peekWord(r.sys.Peek, pc+1); ok {
			r.markPRG(target, codedatalog.SubEntryPoint)
		}

	case cpu6502.JmpName:
		if opcode.Addressing != cpu6502.IndirectAddressing {
			return
		}
		if target, ok := indirectTarget(r.sys.Peek, pc+1); ok {
			r.markPRG(target, codedatalog.IndirectCode)
		}
	}
}

// markPRG marks one PRG byte with a flag.
func (r *CDLRecorder) markPRG(address uint16, flag codedatalog.PrgFlag) {
	offset, ok := r.prgOffset(address)
	if !ok {
		return
	}

	r.mu.Lock()
	flags := r.prg[offset] | flag
	if flag&(codedatalog.Code|codedatalog.Data|codedatalog.PCMAudio) != 0 {
		flags = flags.WithMapping(address)
	}

	r.prg[offset] = flags
	r.mu.Unlock()
}

// prgOffset returns the physical PRG offset of a CPU address.
func (r *CDLRecorder) prgOffset(address uint16) (int, bool) {
	_, offset, ok := r.sys.PRGLocation(address)
	if !ok || offset < 0 || offset >= len(r.prg) {
		return 0, false
	}
	return offset, true
}

// enabled reports whether the recorder is active.
func (r *CDLRecorder) enabled() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.active
}

// coverage builds the per-bank coverage report.
func coverage(prg []codedatalog.PrgFlag) []CDLCoverage {
	segments := (len(prg) + coverageSegmentSize - 1) / coverageSegmentSize
	report := make([]CDLCoverage, 0, segments)

	for segment := range segments {
		start := segment * coverageSegmentSize
		end := min(start+coverageSegmentSize, len(prg))
		code := 0
		data := 0
		for index := start; index < end; index++ {
			if prg[index]&codedatalog.Code != 0 {
				code++
			}

			if prg[index]&codedatalog.Data != 0 {
				data++
			}
		}

		size := end - start
		report = append(report, CDLCoverage{
			Code:        code,
			CodePercent: percent(code, size),
			Data:        data,
			DataPercent: percent(data, size),
			Segment:     segment,
			Size:        size,
		})
	}

	return report
}

// indirectAddressing reports whether an addressing mode reads indirect data.
func indirectAddressing(mode cpu6502.AddressingMode) bool {
	switch mode {
	case cpu6502.IndirectXAddressing, cpu6502.IndirectYAddressing:
		return true
	default:
		return false
	}
}

// peekWord reads a 16-bit value from consecutive addresses. An instruction
// operand can cross a page boundary, and the CPU reads it without the
// page-wrap behavior of an indirect pointer.
func peekWord(peek memoryPeek, address uint16) (uint16, bool) {
	low, ok := peek(address)
	if !ok {
		return 0, false
	}
	high, ok := peek(address + 1)
	if !ok {
		return 0, false
	}
	return uint16(low) | uint16(high)<<8, true
}

// indirectTarget reads the target of an indirect JMP. The operand is a plain
// 16-bit value, and the pointer read has the page-wrap bug.
func indirectTarget(peek memoryPeek, address uint16) (uint16, bool) {
	pointer, ok := peekWord(peek, address)
	if !ok {
		return 0, false
	}
	return peekWordBug(peek, pointer)
}

// percent returns part/total as a percentage.
func percent(part, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total) * 100
}

// copyCDLRange checks the bounds before it copies the selected flags.
func copyCDLRange[T ~byte](flags []T, offset, length int) ([]T, error) {
	if offset < 0 || offset > len(flags) || length < 0 || length > len(flags)-offset {
		return nil, fmt.Errorf("cdl range offset %d length %d is outside %d bytes", offset, length, len(flags))
	}
	return slices.Clone(flags[offset : offset+length]), nil
}

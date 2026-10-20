package nes

import (
	"fmt"
	"io"
	"sync"

	"github.com/retroenv/nesgoemu/pkg/apu"
	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/ppu"
	"github.com/retroenv/retrogolib/arch/cpu/cpu6502"
	"github.com/retroenv/retrogolib/arch/system/nes/codedatalog"
)

// cdlBankSize is the PRG bank size used by the coverage report.
const cdlBankSize = 0x2000

// cdlCodeBase is the start address of the CPU PRG window.
const cdlCodeBase = 0x8000

// CDLCoverage reports the code and data coverage of one PRG bank.
type CDLCoverage struct {
	Bank        int     `json:"bank"`
	Size        int     `json:"size"`
	Code        int     `json:"code"`
	Data        int     `json:"data"`
	CodePercent float64 `json:"code_percent"`
	DataPercent float64 `json:"data_percent"`
}

// CDLRecorder records FCEUX-compatible code and data logging flags for a
// system. The flags cover the PRG ROM and the CHR ROM. The recorder feeds from
// the runtime hooks, so it adds no work while it is stopped.
type CDLRecorder struct {
	sys *System

	mu       sync.Mutex
	prg      []codedatalog.PrgFlag
	chr      []codedatalog.ChrFlag
	remove   []func()
	active   bool
	indirect bool
}

// NewCDLRecorder creates a recorder for the system in the stopped state.
func (sys *System) NewCDLRecorder() *CDLRecorder {
	cart := sys.Bus.Cartridge
	return &CDLRecorder{
		sys: sys,
		prg: make([]codedatalog.PrgFlag, len(cart.PRG)),
		chr: make([]codedatalog.ChrFlag, len(cart.CHR)),
	}
}

// Start begins recording. Repeated calls have no effect.
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
}

// Stop ends recording and removes the runtime hooks.
func (r *CDLRecorder) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.active = false
	for _, remove := range r.remove {
		remove()
	}
	r.remove = nil
	r.sys.apu.ObserveDMCFetches(nil)
}

// Reset clears the recorded flags.
func (r *CDLRecorder) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.prg = make([]codedatalog.PrgFlag, len(r.prg))
	r.chr = make([]codedatalog.ChrFlag, len(r.chr))
}

// Coverage reports the code and data coverage per PRG bank.
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

// Write writes the flags in the FCEUX layout: PRG flags then CHR flags.
func (r *CDLRecorder) Write(destination io.Writer) error {
	r.mu.Lock()
	prg := append([]codedatalog.PrgFlag(nil), r.prg...)
	chr := append([]codedatalog.ChrFlag(nil), r.chr...)
	r.mu.Unlock()

	if _, err := destination.Write(toBytes(prg)); err != nil {
		return fmt.Errorf("writing PRG flags: %w", err)
	}
	if _, err := destination.Write(chrBytes(chr)); err != nil {
		return fmt.Errorf("writing CHR flags: %w", err)
	}
	return nil
}

// observeInstruction marks the executed instruction and its entries.
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
	r.markInstruction(pc, addressingSize(opcode.Addressing))
	r.markEntry(opcode, pc)
	r.mu.Lock()
	r.indirect = indirectAddressing(opcode.Addressing)
	r.mu.Unlock()
}

// observeCycle marks the data flags for one CPU bus cycle. A write does not
// mark PRG data, because a write to the PRG range selects a mapper register.
// A read of the PPU data register marks the CHR byte that the program reads.
func (r *CDLRecorder) observeCycle(cycle cpu6502.BusCycle) {
	if !r.enabled() {
		return
	}
	if cycle.Write {
		return
	}
	if isPPUDataRegister(cycle.Address) {
		r.markProgrammaticCHR()
		return
	}
	if cycle.Address < cdlCodeBase {
		return
	}

	r.mu.Lock()
	indirect := r.indirect
	r.mu.Unlock()

	flag := codedatalog.Data
	if indirect {
		flag = codedatalog.IndirectData
	}
	r.markPRG(cycle.Address, flag)
}

// observeDMCFetch marks a DMC sample byte as PCM audio.
func (r *CDLRecorder) observeDMCFetch(fetch apu.DMCFetch) {
	if !r.enabled() {
		return
	}
	r.markPRG(fetch.Address, codedatalog.PCMAudio)
}

// markInstruction marks each byte of an executed instruction as code.
func (r *CDLRecorder) markInstruction(pc uint16, size int) {
	for offset := range size {
		r.markCode(pc + uint16(offset))
	}
}

// markEntry marks the JSR and indirect JMP targets of an instruction.
func (r *CDLRecorder) markEntry(opcode cpu6502.Opcode, pc uint16) {
	switch opcode.Instruction.Name {
	case cpu6502.JsrName:
		if target, ok := peekWordBug(r.sys.Peek, pc+1); ok {
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

// markCode marks one PRG byte as code and clears its data flags. The fetch of an
// instruction looks like a data read on the bus, so the flags are cleared here.
func (r *CDLRecorder) markCode(address uint16) {
	offset, ok := r.prgOffset(address)
	if !ok {
		return
	}

	r.mu.Lock()
	r.prg[offset] |= codedatalog.Code
	r.prg[offset] &^= codedatalog.Data | codedatalog.IndirectData
	r.mu.Unlock()
}

// markPRG marks one PRG byte with a flag.
func (r *CDLRecorder) markPRG(address uint16, flag codedatalog.PrgFlag) {
	offset, ok := r.prgOffset(address)
	if !ok {
		return
	}

	r.mu.Lock()
	r.prg[offset] |= flag
	r.mu.Unlock()
}

// markProgrammaticCHR marks the current CHR address as read by the program.
func (r *CDLRecorder) markProgrammaticCHR() {
	device, ok := r.sys.Bus.PPU.(*ppu.PPU)
	if !ok {
		return
	}
	address := device.Snapshot().V & 0x3FFF
	if address >= 0x2000 {
		return
	}

	offset := int(address)
	if inspector, ok := r.sys.Bus.Mapper.(bus.PPUMappingInspector); ok {
		if mapping, ok := inspector.PPUReadMapping(address); ok {
			offset = mapping.Offset
		}
	}
	if offset < 0 || offset >= len(r.chr) {
		return
	}

	r.mu.Lock()
	r.chr[offset] |= codedatalog.ReadProgrammatically
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
	banks := (len(prg) + cdlBankSize - 1) / cdlBankSize
	report := make([]CDLCoverage, 0, banks)

	for bank := range banks {
		start := bank * cdlBankSize
		end := min(start+cdlBankSize, len(prg))
		code, data := 0, 0
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
			Bank:        bank,
			Size:        size,
			Code:        code,
			Data:        data,
			CodePercent: percent(code, size),
			DataPercent: percent(data, size),
		})
	}
	return report
}

// isPPUDataRegister reports whether an address selects $2007 or one of its
// mirrors in $2008-$3FFF.
func isPPUDataRegister(address uint16) bool {
	return address >= 0x2000 && address < 0x4000 && address&7 == 7
}

// indirectAddressing reports whether an addressing mode reads indirect data.
func indirectAddressing(mode cpu6502.AddressingMode) bool {
	switch mode {
	case cpu6502.AbsoluteXAddressing, cpu6502.AbsoluteYAddressing,
		cpu6502.ZeroPageXAddressing, cpu6502.ZeroPageYAddressing,
		cpu6502.IndirectAddressing, cpu6502.IndirectXAddressing, cpu6502.IndirectYAddressing:
		return true
	default:
		return false
	}
}

// indirectTarget reads the target of an indirect JMP with the page-wrap bug.
func indirectTarget(peek memoryPeek, address uint16) (uint16, bool) {
	pointer, ok := peekWordBug(peek, address)
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

// toBytes converts PRG flags to bytes.
func toBytes(flags []codedatalog.PrgFlag) []byte {
	data := make([]byte, len(flags))
	for index, flag := range flags {
		data[index] = byte(flag)
	}
	return data
}

// chrBytes converts CHR flags to bytes.
func chrBytes(flags []codedatalog.ChrFlag) []byte {
	data := make([]byte, len(flags))
	for index, flag := range flags {
		data[index] = byte(flag)
	}
	return data
}

package mcpserver

import (
	"errors"
	"fmt"
	"strings"

	"github.com/retroenv/retrogolib/arch/cpu/cpu6502"
)

// Memory space names shared by the read and write tools.
const (
	spaceCPU = "cpu"
	spacePPU = "ppu"
	spaceOAM = "oam"
	spacePRG = "prg"
	spaceCHR = "chr"
	spaceRAM = "ram"
)

// flagOrder is the display order of the status flags, from bit 7 to bit 0. The
// unused bit 5 shows a dash.
const flagOrder = "NV-BDIZC"

// cpuState returns the current CPU state.
func (m *Machine) cpuState() cpuStateResult {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.cpuStateLocked()
}

// cpuStateLocked returns the current CPU state. The caller holds the machine mutex.
func (m *Machine) cpuStateLocked() cpuStateResult {
	state := m.sys.State()
	bank, offset, _ := m.sys.PRGLocation(state.PC)
	return stateResult(state, m.sys.Bus.PPU.Frame(), bank, offset)
}

// machineInfo returns the mapper, memory, and feature state.
func (m *Machine) machineInfo() machineInfoResult {
	m.mu.Lock()
	defer m.mu.Unlock()

	summary := m.sys.Summary()
	state := m.sys.Bus.Mapper.State()

	result := machineInfoResult{
		MachineID:  m.id,
		ROM:        m.rom,
		Frame:      m.sys.Bus.PPU.Frame(),
		MapperID:   summary.MapperID,
		MapperName: summary.MapperName,
		Mirroring:  mirrorName(summary.MirrorMode),
		Battery:    summary.Battery,
		PRGWindows: state.PrgWindows,
		CHRWindows: state.ChrWindows,
		PRGRAMSize: summary.PrgRAMSize,
		CHRRAMSize: summary.ChrRAMSize,
	}
	for _, group := range summary.Groups {
		for _, usage := range group.Features {
			result.Features = append(result.Features, featureUsage{
				Group: group.Name,
				Name:  usage.Name,
				Used:  usage.Used,
			})
		}
	}
	return result
}

// readMemory reads a side-effect-free memory range for the given space. The
// start is a CPU or PPU address for the address spaces and a physical offset for
// the prg and chr spaces. An address range wraps at 16 bits.
func (m *Machine) readMemory(space string, start, length int) readMemoryResult {
	m.mu.Lock()
	defer m.mu.Unlock()

	values := make([]int, length)
	valid := make([]bool, length)
	bytes := make([]int, length)
	available := 0

	for offset := range length {
		value, ok := m.readMemoryByte(space, start+offset)
		values[offset] = int(value)
		valid[offset] = ok
		bytes[offset] = -1
		if ok {
			available++
			bytes[offset] = int(value)
		}
	}

	return readMemoryResult{
		Space:     space,
		Address:   Address(start),
		Start:     start,
		Length:    length,
		Available: available,
		Bytes:     bytes,
		Hex:       formatHexDump(start, values, valid),
	}
}

// readMemoryByte reads one byte of the selected space.
func (m *Machine) readMemoryByte(space string, index int) (byte, bool) {
	sys := m.sys
	address := uint16(index)
	switch space {
	case spaceCPU:
		return sys.Peek(address)

	case spaceRAM:
		return sys.InspectRAM(address)

	case spacePPU:
		return sys.InspectPPU(address)

	case spaceOAM:
		if index < 0 || index >= 256 {
			return 0, false
		}
		oam := sys.InspectOAM()
		return oam[index], true

	case spacePRG:
		prg := sys.Bus.Cartridge.PRG
		if index < 0 || index >= len(prg) {
			return 0, false
		}
		return prg[index], true

	case spaceCHR:
		chr := sys.Bus.Cartridge.CHR
		if index < 0 || index >= len(chr) {
			return 0, false
		}
		return chr[index], true

	default:
		return 0, false
	}
}

// writeMemory writes bytes to CPU RAM or PRG-RAM. It refuses register and ROM
// ranges and checks the whole range before it writes the first byte.
func (m *Machine) writeMemory(space string, address Address, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if space != spaceCPU && space != spaceRAM {
		return fmt.Errorf("write_memory space %q is not writable; use cpu", space)
	}
	if len(data) == 0 {
		return errors.New("write_memory needs at least one byte")
	}
	if int(address)+len(data) > 0x10000 {
		return fmt.Errorf("write of %d bytes at %s passes the end of the address space", len(data), address)
	}
	for offset := range data {
		target := uint16(address) + uint16(offset)
		if !writableCPUAddress(target) {
			return fmt.Errorf("address 0x%04x is a register or ROM range and is not writable", target)
		}
	}

	for offset, value := range data {
		m.sys.Bus.Memory.Write(uint16(address)+uint16(offset), value)
	}
	m.recordPoke(address, data)
	return nil
}

// cpuStateResult contains the CPU registers and timing of a machine.
type cpuStateResult struct {
	A          byte    `json:"a"`
	X          byte    `json:"x"`
	Y          byte    `json:"y"`
	PC         Address `json:"pc"`
	SP         byte    `json:"sp"`
	P          byte    `json:"p"`
	Flags      string  `json:"flags"`
	Cycles     uint64  `json:"cycles"`
	Frame      uint64  `json:"frame"`
	NMIPending bool    `json:"nmi_pending"`
	NMIRunning bool    `json:"nmi_running"`
	IRQPending bool    `json:"irq_pending"`
	IRQRunning bool    `json:"irq_running"`
	PRGBank    int     `json:"prg_bank"`
	PRGOffset  int     `json:"prg_offset"`
}

// featureUsage is one reported hardware feature.
type featureUsage struct {
	Group string `json:"group"`
	Name  string `json:"name"`
	Used  bool   `json:"used"`
}

// machineInfoResult contains the mapper, memory, and feature state.
type machineInfoResult struct {
	MachineID  string         `json:"machine_id"`
	ROM        string         `json:"rom"`
	Frame      uint64         `json:"frame"`
	MapperID   uint16         `json:"mapper_id"`
	MapperName string         `json:"mapper_name"`
	Mirroring  string         `json:"mirroring"`
	Battery    bool           `json:"battery"`
	PRGWindows []int          `json:"prg_windows"`
	CHRWindows []int          `json:"chr_windows"`
	PRGRAMSize int            `json:"prg_ram_size"`
	CHRRAMSize int            `json:"chr_ram_size"`
	Features   []featureUsage `json:"features"`
}

// readMemoryResult contains a side-effect-free memory view. Start is the first
// address, or the first physical offset in the prg and chr spaces.
type readMemoryResult struct {
	Space     string  `json:"space"`
	Address   Address `json:"address"`
	Start     int     `json:"start"`
	Length    int     `json:"length"`
	Available int     `json:"available"`
	Bytes     []int   `json:"bytes"`
	Hex       string  `json:"hex"`
}

// stateResult converts a CPU snapshot into a tool result.
func stateResult(state cpu6502.State, frame uint64, bank, offset int) cpuStateResult {
	p := flagsByte(state.Flags)
	return cpuStateResult{
		A:          state.A,
		X:          state.X,
		Y:          state.Y,
		PC:         Address(state.PC),
		SP:         state.SP,
		P:          p,
		Flags:      flagLetters(p),
		Cycles:     state.Cycles,
		Frame:      frame,
		NMIPending: state.Interrupts.NMITriggered,
		NMIRunning: state.Interrupts.NMIRunning,
		IRQPending: state.Interrupts.IrqTriggered,
		IRQRunning: state.Interrupts.IrqRunning,
		PRGBank:    bank,
		PRGOffset:  offset,
	}
}

// writableCPUAddress reports whether a CPU address accepts a plain RAM write.
func writableCPUAddress(address uint16) bool {
	switch {
	case address < 0x2000:
		return true
	case address >= 0x6000 && address < 0x8000:
		return true
	default:
		return false
	}
}

// formatHexDump renders values as a 16-byte-per-row hex dump. The row labels
// grow to six digits for a start beyond the 16-bit range.
func formatHexDump(start int, values []int, valid []bool) string {
	const perRow = 16
	var builder strings.Builder

	width := 4
	if start+len(values) > 0x10000 {
		width = 6
	}
	for rowStart := 0; rowStart < len(values); rowStart += perRow {
		end := min(rowStart+perRow, len(values))
		fmt.Fprintf(&builder, "%0*X: ", width, start+rowStart)

		for index := rowStart; index < end; index++ {
			if valid[index] {
				fmt.Fprintf(&builder, "%02X ", values[index])
			} else {
				builder.WriteString("-- ")
			}
		}
		for index := end; index < rowStart+perRow; index++ {
			builder.WriteString("   ")
		}

		builder.WriteString(" ")
		for index := rowStart; index < end; index++ {
			if !valid[index] {
				builder.WriteByte('.')
				continue
			}
			printable := values[index]
			if printable < 0x20 || printable > 0x7E {
				builder.WriteByte('.')
			} else {
				builder.WriteByte(byte(printable))
			}
		}
		builder.WriteByte('\n')
	}
	return builder.String()
}

// flagsByte encodes the CPU flags as one byte.
func flagsByte(flags cpu6502.Flags) byte {
	return flags.C |
		flags.Z<<1 |
		flags.I<<2 |
		flags.D<<3 |
		flags.B<<4 |
		flags.U<<5 |
		flags.V<<6 |
		flags.N<<7
}

// flagLetters renders the status flags in "NV-BDIZC" order.
func flagLetters(p byte) string {
	out := []byte(flagOrder)
	for index, name := range flagOrder {
		if name == '-' {
			continue
		}
		if p&(1<<uint(7-index)) == 0 {
			out[index] = '-'
		}
	}
	return string(out)
}

package rainbow

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/nesgoemu/pkg/ppu/nametable"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

const testUint16Maximum = 1<<16 - 1

func newTestMapper(t *testing.T, prgSize, chrSize int) *Mapper {
	t.Helper()
	m, _ := newTestMapperWithBus(t, prgSize, chrSize)
	return m
}

func newTestMapperWithBus(t *testing.T, prgSize, chrSize int) (*Mapper, *bus.Bus) {
	t.Helper()

	prg := make([]byte, prgSize)
	chr := make([]byte, chrSize)

	system := &bus.Bus{
		Cartridge: &cartridge.Cartridge{
			CHR: chr,
			PRG: prg,
		},
		NameTable: nametable.New(cartridge.MirrorHorizontal),
	}
	base := mapperbase.New(system)
	m, err := New(base)
	assert.NoError(t, err)

	return m.(*Mapper), system
}

// --- PRG Mode Tests ---

func TestPRGMode0_32KWindow(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.prgROM[0x0010] = 0xAA
	m.prgROM[0x4010] = 0xBB

	assert.Equal(t, 0xAA, m.Read(0x8010))
	assert.Equal(t, 0xBB, m.Read(0xC010))
}

func TestPRGMode1_16KWindows(t *testing.T) {
	m := newTestMapper(t, 0x10000, 0x2000)
	m.prgMode = 1

	// highBanks[0] → $8000-$BFFF, highBanks[4] → $C000-$FFFF.
	m.highBanks[0] = 0
	m.highBanks[4] = 1

	m.prgROM[0x0010] = 0xAA      // bank 0 offset 0x10
	m.prgROM[0x4000+0x10] = 0xBB // bank 1 offset 0x10

	assert.Equal(t, 0xAA, m.Read(0x8010))
	assert.Equal(t, 0xBB, m.Read(0xC010))

	// Switch window 0 to bank 1.
	m.highBanks[0] = 1
	assert.Equal(t, 0xBB, m.Read(0x8010))
}

func TestPRGMode2_MixedWindows(t *testing.T) {
	m := newTestMapper(t, 0x10000, 0x2000)
	m.prgMode = 2

	// highBanks[0]: 16K window at $8000-$BFFF (bank 0).
	m.highBanks[0] = 0
	// highBanks[4]: 8K window at $C000-$DFFF (bank 0).
	m.highBanks[4] = 0
	// highBanks[6]: 8K window at $E000-$FFFF (bank 1).
	m.highBanks[6] = 1

	m.prgROM[0x0010] = 0xAA      // 16K bank 0
	m.prgROM[0x0000] = 0xCC      // 8K bank 0
	m.prgROM[0x2000+0x10] = 0xBB // 8K bank 1

	assert.Equal(t, 0xAA, m.Read(0x8010))
	assert.Equal(t, 0xCC, m.Read(0xC000))
	assert.Equal(t, 0xBB, m.Read(0xE010))
}

func TestPRGMode3_8KWindows(t *testing.T) {
	m := newTestMapper(t, 0x10000, 0x2000)
	m.prgMode = 3

	// highBanks[0], [2], [4], [6] → 4 × 8K windows.
	m.highBanks[0] = 0
	m.highBanks[2] = 1
	m.highBanks[4] = 2
	m.highBanks[6] = 3

	m.prgROM[0x0010] = 0xAA
	m.prgROM[0x2010] = 0xBB
	m.prgROM[0x4010] = 0xCC
	m.prgROM[0x6010] = 0xDD

	assert.Equal(t, 0xAA, m.Read(0x8010))
	assert.Equal(t, 0xBB, m.Read(0xA010))
	assert.Equal(t, 0xCC, m.Read(0xC010))
	assert.Equal(t, 0xDD, m.Read(0xE010))
}

func TestPRGMode4_4KWindows(t *testing.T) {
	m := newTestMapper(t, 0x10000, 0x2000)
	m.prgMode = 4

	// highBanks[0..7] → 8 × 4K windows.
	for i := range 8 {
		m.highBanks[i] = uint16(i)
	}

	m.prgROM[0x0010] = 0xAA
	m.prgROM[0x1010] = 0xBB

	assert.Equal(t, 0xAA, m.Read(0x8010))
	assert.Equal(t, 0xBB, m.Read(0x9010))
}

// --- PRG RAM Tests ---

func TestPRGRAMMode0_8KWindow(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	m.prgRAMMode = 0
	// lowBanks[0] with chip=PRG-RAM (bits [15:14]=10), bank 0.
	m.lowBanks[0] = 0x8000 // bit 15 set = chip select 10

	m.prgRAM[0x0010] = 0xAA
	assert.Equal(t, 0xAA, m.Read(0x6010))

	m.Write(0x6020, 0xBB)
	assert.Equal(t, 0xBB, m.prgRAM[0x0020])
}

func TestPRGRAMMode1_4KWindows(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	m.prgRAMMode = 1
	// lowBanks[0] → $6000-$6FFF, bank 0, chip=PRG-RAM.
	m.lowBanks[0] = 0x8000
	// lowBanks[1] → $7000-$7FFF, bank 1, chip=PRG-RAM.
	m.lowBanks[1] = 0x8001

	m.prgRAM[0x0010] = 0xAA      // bank 0, offset 0x10
	m.prgRAM[0x1000+0x10] = 0xBB // bank 1, offset 0x10

	assert.Equal(t, 0xAA, m.Read(0x6010))
	assert.Equal(t, 0xBB, m.Read(0x7010))
}

func TestPRGLowBank_FPGARAM(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	m.prgRAMMode = 0
	// lowBanks[0] with chip=FPGA-RAM (bits [15:14]=11), bank 0.
	m.lowBanks[0] = 0xC000

	m.fpgaRAM[0x0010] = 0xDD
	assert.Equal(t, 0xDD, m.Read(0x6010))

	m.Write(0x6020, 0xEE)
	assert.Equal(t, 0xEE, m.fpgaRAM[0x0020])
}

func TestPRGLowBank_ROM(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	m.prgRAMMode = 0
	// lowBanks[0] with chip=PRG-ROM (bit 15=0), bank 0.
	m.lowBanks[0] = 0x0000

	m.prgROM[0x0010] = 0xCC
	assert.Equal(t, 0xCC, m.Read(0x6010))
}

func TestPRGHighBank_RAM(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	m.prgMode = 0
	// highBanks[0] with bit 15 set = RAM.
	m.highBanks[0] = 0x8000

	m.prgRAM[0x0010] = 0xEE
	assert.Equal(t, 0xEE, m.Read(0x8010))
}

// --- CHR Tests ---

func TestCHRMode0_8KWindow(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.chrROM[0x0100] = 0xAA
	m.chrROM[0x1100] = 0xBB

	assert.Equal(t, 0xAA, m.Read(0x0100))
	assert.Equal(t, 0xBB, m.Read(0x1100))
}

func TestCHRMode3_1KWindows(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x4000)
	m.chrMode = 3

	// Map window 0 to bank 0, window 1 to bank 1.
	m.chrBanks[0] = 0
	m.chrBanks[1] = 1

	m.chrROM[0x0010] = 0xAA      // 1K bank 0
	m.chrROM[0x0400+0x10] = 0xBB // 1K bank 1

	assert.Equal(t, 0xAA, m.Read(0x0010))
	assert.Equal(t, 0xBB, m.Read(0x0410))
}

func TestCHRRAMSource(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	m.chrSource = chrSourceRAM

	m.chrRAM[0x0100] = 0xAA
	assert.Equal(t, 0xAA, m.Read(0x0100))

	m.Write(0x0100, 0xBB)
	assert.Equal(t, 0xBB, m.chrRAM[0x0100])
}

// --- FPGA RAM Tests ---

func TestFPGARAMFixedWindow(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.fpgaRAM[0x1800] = 0xAA
	m.fpgaRAM[0x1900] = 0xBB

	assert.Equal(t, 0xAA, m.Read(0x4800))
	assert.Equal(t, 0xBB, m.Read(0x4900))

	m.Write(0x4800, 0xCC)
	assert.Equal(t, 0xCC, m.fpgaRAM[0x1800])
}

func TestFPGARAMBankedWindow(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.fpgaRAM[0x0010] = 0xAA // page 0, offset 0x10
	m.fpgaRAM[0x1010] = 0xBB // page 1, offset 0x10

	m.fpgaBankSelect = 0
	assert.Equal(t, 0xAA, m.Read(0x5010))

	m.fpgaBankSelect = 1
	assert.Equal(t, 0xBB, m.Read(0x5010))

	m.fpgaBankSelect = 0
	m.Write(0x5010, 0xCC)
	assert.Equal(t, 0xCC, m.fpgaRAM[0x0010])
}

// --- Register Tests ---

func TestRegisterPRGControl(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	// $4100: bit 7 = PRG-RAM mode, bits [2:0] = PRG-ROM mode.
	m.Write(registerStart, 0x83) // prgMode=3, prgRAMMode=1 (bit 7)
	assert.Equal(t, byte(3), m.prgMode)
	assert.Equal(t, byte(1), m.prgRAMMode)

	val := m.Read(regPRGControl)
	assert.Equal(t, 3|1<<7, val) // bit 7 for RAM mode
}

func TestRegisterCHRControl(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	// $4120: bits [7:6]=source, bit 5=sprite ext, bit 4=window, bits [2:0]=mode.
	m.Write(regCHRControl, 0xB2) // source=2 (FPGA), spriteExt=1, window=1, mode=2
	assert.Equal(t, byte(2), m.chrMode)
	assert.Equal(t, byte(2), m.chrSource)
	assert.True(t, m.spriteExtMode)
	assert.True(t, m.windowEnabled)

	val := m.Read(regCHRControl)
	assert.Equal(t, 2|1<<4|1<<5|2<<6, val)
}

func TestRegisterCHRControl_WindowAndSpriteExtDisabled(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.Write(regCHRControl, 0x82) // source=2 (FPGA), no window, no sprite ext, mode=2
	assert.Equal(t, byte(2), m.chrMode)
	assert.Equal(t, byte(2), m.chrSource)
	assert.False(t, m.spriteExtMode)
	assert.False(t, m.windowEnabled)
}

func TestRegisterPlatform(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	assert.Equal(t, byte(0x21), m.Read(regPlatformVersion))
}

func TestRegisterBGExtOffset(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.Write(regBGExtModeOffset, 0x1F)
	assert.Equal(t, byte(0x1F), m.bgExtModeOffset)

	m.Write(regBGExtModeOffset, 0xFF) // should mask to 5 bits
	assert.Equal(t, byte(0x1F), m.bgExtModeOffset)
}

// --- PRG Bank Register Mapping Tests ---

func TestPRGBankRegistersHighBanks(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	// Write high byte via $4108, low byte via $4118 → highBanks[0].
	m.Write(regHighBankUpperStart, 0x02)
	m.Write(regHighBankLowerStart, 0x10)

	assert.Equal(t, uint16(0x0210), m.highBanks[0])
}

func TestPRGBankRegistersLowBanks(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	// Write high byte via $4106, low byte via $4116 → lowBanks[0].
	m.Write(regLowBankUpperStart, 0x80) // chip select = PRG-RAM
	m.Write(regLowBankLowerStart, 0x05)

	assert.Equal(t, uint16(0x8005), m.lowBanks[0])
}

func TestCHRBankRegisters(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	// Write high byte via $4130, low byte via $4140 → chrBanks[0].
	m.Write(regCHRBankUpperStart, 0x05)
	m.Write(regCHRBankLowerStart, 0x20)

	assert.Equal(t, uint16(0x0520), m.chrBanks[0])
}

// --- Scanline IRQ Register Tests ---

func TestScanlineIRQRegisters(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	// $4150: set target scanline to 100.
	m.Write(regScanIRQLatch, 100)
	assert.Equal(t, byte(100), m.scanIRQ.latch)

	// $4151: enable scanline IRQ.
	m.Write(regScanIRQControl, 0x00)
	assert.True(t, m.scanIRQ.enabled)

	// $4152: disable and acknowledge.
	m.Write(regScanIRQAcknowledge, 0x00)
	assert.False(t, m.scanIRQ.enabled)
	assert.False(t, m.scanIRQ.pending)
}

func TestScanlineIRQStatusRead(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.scanIRQ.enabled = true
	m.scanIRQ.pending = true
	m.scanIRQ.inFrame = true
	m.scanIRQ.inHBlank = true

	// Reading $4151 returns status and clears pending.
	val := m.Read(regScanIRQControl)
	assert.Equal(t, 0x80|0x40, val)    // HBlank and in-frame.
	assert.False(t, m.scanIRQ.pending) // cleared by read
}

func TestScanlineIRQOffset(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.Write(regScanIRQOffset, 135)
	assert.Equal(t, byte(135), m.scanIRQ.offset)

	// Clamp to 1-170.
	m.Write(regScanIRQOffset, 0)
	assert.Equal(t, byte(1), m.scanIRQ.offset)

	m.Write(regScanIRQOffset, 200)
	assert.Equal(t, byte(170), m.scanIRQ.offset)
}

func TestScanlineIRQJitter(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	m.scanIRQ.jitter = 42
	assert.Equal(t, byte(42), m.Read(regScanIRQJitter))
}

// --- Cycle IRQ Register Tests ---

func TestCycleIRQRegisters(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	// $4158: latch high byte (does not reload the counter).
	m.Write(regCycleIRQReloadUpper, 0x01)
	assert.Equal(t, uint16(0x0100), m.cycleIRQ.reloadValue)
	assert.Equal(t, uint16(0), m.cycleIRQ.counter)

	// $4159: latch low byte (does not reload the counter).
	m.Write(regCycleIRQReloadLower, 0x10)
	assert.Equal(t, uint16(0x0110), m.cycleIRQ.reloadValue)
	assert.Equal(t, uint16(0), m.cycleIRQ.counter)

	// $415A: control - enable + auto-reload.
	m.Write(regCycleIRQControl, 0x03) // enable + auto-reload after ack
	assert.True(t, m.cycleIRQ.enabled)
	assert.True(t, m.cycleIRQ.enableAfterAck)
	assert.Equal(t, uint16(0x0110), m.cycleIRQ.counter) // reloaded on enable
}

func TestCycleIRQAcknowledge(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.cycleIRQ.reloadValue = 0x0050
	m.cycleIRQ.enabled = true
	m.cycleIRQ.pending = true
	m.cycleIRQ.enableAfterAck = true

	// $415B: acknowledge.
	m.Write(regCycleIRQAcknowledge, 0x00)
	assert.False(t, m.cycleIRQ.pending)
	assert.True(t, m.cycleIRQ.enabled)             // re-enabled via enableAfterAck
	assert.Equal(t, uint16(0), m.cycleIRQ.counter) // Counter is unchanged.
}

func TestCycleIRQZPCMAckFlag(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.Write(regCycleIRQControl, 0x05) // enable + ZPCM ack
	assert.True(t, m.cycleIRQ.enabled)
	assert.True(t, m.cycleIRQ.ackOn4011)
	assert.False(t, m.cycleIRQ.enableAfterAck)
}

// --- Combined IRQ Status ---

func TestCombinedIRQStatus(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.scanIRQ.pending = true
	m.cycleIRQ.pending = true

	val := m.Read(regIRQStatus)
	assert.Equal(t, 0x80|0x40, val) // scanline|cycle
}

// --- FPGA Auto R/W Tests ---

func TestFPGAAutoReadWrite(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.fpgaAutoAddr = 0x0000
	m.fpgaAutoInc = 1

	// Write via auto-writer.
	m.Write(regFPGAAutoData, 0xAA)
	m.Write(regFPGAAutoData, 0xBB)

	assert.Equal(t, 0xAA, m.fpgaRAM[0])
	assert.Equal(t, 0xBB, m.fpgaRAM[1])

	// Read via auto-reader.
	m.fpgaAutoAddr = 0x0000
	assert.Equal(t, 0xAA, m.Read(regFPGAAutoData))
	assert.Equal(t, 0xBB, m.Read(regFPGAAutoData))
}

func TestFPGAAutoAddressRegisters(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	// $415C: high bits [12:8], masked to 5 bits.
	m.Write(regFPGAAutoAddressUpper, 0xFF) // should mask to 0x1F
	assert.Equal(t, uint16(0x1F00), m.fpgaAutoAddr)

	// $415D: low byte [7:0].
	m.Write(regFPGAAutoAddressLower, 0xAB)
	assert.Equal(t, uint16(0x1FAB), m.fpgaAutoAddr)
}

func TestFPGAAutoAddressWraps(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.fpgaAutoAddr = 0x1FFF
	m.fpgaAutoInc = 1

	// Read should wrap around within 8K FPGA-RAM.
	_ = m.Read(regFPGAAutoData) // reads from 0x1FFF, advances to 0x0000
	assert.Equal(t, uint16(0x0000), m.fpgaAutoAddr)
}

// --- Vector Redirection Tests ---

func TestVectorRedirection(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	// $416B: enable NMI+IRQ redirection.
	m.Write(regVectorControl, 0x03)
	assert.True(t, m.nmiVectorEnabled)
	assert.True(t, m.irqVectorEnabled)

	// $416C=high, $416D=low for NMI address.
	m.Write(regNMIVectorUpper, 0x80) // NMI high byte
	m.Write(regNMIVectorLower, 0x10) // NMI low byte
	assert.Equal(t, uint16(0x8010), m.nmiAddr)

	// $416E=high, $416F=low for IRQ address.
	m.Write(regIRQVectorUpper, 0xC0) // IRQ high byte
	m.Write(regIRQVectorLower, 0x20) // IRQ low byte
	assert.Equal(t, uint16(0xC020), m.irqAddr)

	// Read vectors: $FFFA/$FFFB should return redirected NMI address.
	assert.Equal(t, byte(0x10), m.Read(nmiVectorLower)) // low byte
	assert.Equal(t, byte(0x80), m.Read(nmiVectorUpper)) // high byte

	// $FFFE/$FFFF should return redirected IRQ address.
	assert.Equal(t, byte(0x20), m.Read(irqVectorLower)) // low byte
	assert.Equal(t, byte(0xC0), m.Read(irqVectorUpper)) // high byte
}

func TestVectorRedirectionDisabled(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.Write(regVectorControl, 0x00) // redirection disabled
	m.Write(regNMIVectorUpper, 0x80)
	m.Write(regNMIVectorLower, 0x10)

	// Should fall through to normal PRG read (not redirected).
	expected := m.readPRG(nmiVectorLower)
	assert.Equal(t, expected, m.Read(nmiVectorLower))
}

// --- Window Split Tests ---

func TestWindowSplitRegisters(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	// Window split is now controlled via $4120 bit 4, not $4170.
	m.Write(regCHRControl, 0x10) // enable window split via CHR control
	assert.True(t, m.windowEnabled)

	m.Write(regCHRControl, 0x00) // disable
	assert.False(t, m.windowEnabled)

	// $4170-$4175 store window parameters.
	m.Write(regWindowSplitStart, 0x05)   // X start
	m.Write(regWindowSplitStart+1, 0x1F) // X end
	assert.Equal(t, byte(0x05), m.windowSplitRegs[0])
	assert.Equal(t, byte(0x1F), m.windowSplitRegs[1])
}

func TestWifiRegisters(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	// $4190: ESP config.
	m.Write(regESPControl, 0x03) // enable ESP + IRQ
	assert.True(t, m.espEnabled)
	assert.True(t, m.wifiIrqEnable)

	// Read back config.
	val := m.Read(regESPControl)
	assert.Equal(t, 0x03, val)

	// Disable.
	m.Write(regESPControl, 0x00)
	assert.False(t, m.espEnabled)
	assert.False(t, m.wifiIrqEnable)
}

// --- Nametable Register Tests ---

func TestNametableRegisters(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.Write(regFillTile, 0xAA)      // fill tile
	m.Write(regFillAttribute, 0x55) // fill attr (masked to 2 bits)
	assert.Equal(t, byte(0xAA), m.fillTile)
	assert.Equal(t, byte(0x01), m.fillAttr) // 0x55 & 0x03 = 0x01

	m.Write(regNTBankStart, 0x01)    // NT bank 0
	m.Write(regNTControlStart, 0x02) // NT control 0
	assert.Equal(t, byte(0x01), m.ntBank[0])
	assert.Equal(t, byte(0x02), m.ntControl[0])

	// Read NT control back.
	assert.Equal(t, 0x02, m.Read(regNTControlStart))
}

// --- Sprite Ext Register Tests ---

func TestSpriteExtRegisters(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.Write(regSpriteBankLowerStart, 0x10) // sprite bank lower[0]
	m.Write(regSpriteBankUpper, 0x03)      // sprite bank upper

	assert.Equal(t, byte(0x10), m.spriteBankLower[0])
	assert.Equal(t, byte(0x03), m.spriteBankUpper)
}

// --- ClockCPU / CPU Cycle IRQ Tests ---

func TestClockCPU_DisabledNoOp(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.cycleIRQ.counter = 5
	m.cycleIRQ.enabled = false

	m.ClockCPU(10)

	assert.Equal(t, uint16(5), m.cycleIRQ.counter) // unchanged
	assert.False(t, m.cycleIRQ.pending)
}

func TestClockCPU_Decrement(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.cycleIRQ.enabled = true
	m.cycleIRQ.counter = 10

	m.ClockCPU(3)

	assert.Equal(t, uint16(7), m.cycleIRQ.counter)
	assert.False(t, m.cycleIRQ.pending)
}

func TestClockCPU_FiresAtZero(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.cycleIRQ.enabled = true
	m.cycleIRQ.counter = 3

	m.ClockCPU(3)

	assert.True(t, m.cycleIRQ.pending)
	assert.True(t, m.cycleIRQ.enabled) // Counting continues until acknowledgement.
}

func TestClockCPU_AutoReload(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.cycleIRQ.enabled = true
	m.cycleIRQ.enableAfterAck = true
	m.cycleIRQ.reloadValue = 5
	m.cycleIRQ.counter = 3

	m.ClockCPU(3)

	assert.True(t, m.cycleIRQ.pending)
	assert.True(t, m.cycleIRQ.enabled)             // re-enabled via auto-reload
	assert.Equal(t, uint16(5), m.cycleIRQ.counter) // reloaded
}

func TestClockCPU_MultipleFiresAutoReload(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.cycleIRQ.enabled = true
	m.cycleIRQ.enableAfterAck = true
	m.cycleIRQ.reloadValue = 2
	m.cycleIRQ.counter = 2

	// 7 cycles with period=2: fires at cycle 2, 4, 6 → pending=true, counter=1 remaining.
	m.ClockCPU(7)

	assert.True(t, m.cycleIRQ.pending)
	assert.Equal(t, uint16(1), m.cycleIRQ.counter)
}

func TestClockCPU_JitterIncrement(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.scanIRQ.jitter = 0

	m.ClockCPU(5)

	assert.Equal(t, byte(5), m.scanIRQ.jitter)
}

func TestClockCPU_JitterWrap(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.scanIRQ.jitter = 0xFE

	m.ClockCPU(5)

	assert.Equal(t, byte(3), m.scanIRQ.jitter)
}

func TestClockCPU_JitterResetOnFire(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.cycleIRQ.enabled = true
	m.cycleIRQ.counter = 2
	m.scanIRQ.jitter = 100

	// 4 cycles: IRQ fires at cycle 2, jitter resets to 0, then counts 2 more.
	m.ClockCPU(4)

	assert.Equal(t, byte(2), m.scanIRQ.jitter)
}

// --- TickPPU / Scanline IRQ Tests ---

func TestTickPPU_ScanlineMatchFires(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.scanIRQ.enabled = true
	m.scanIRQ.latch = 50

	m.TickPPU(0, 50, true) // cycle=0: arms the IRQ (readyToFire=true)
	assert.True(t, m.scanIRQ.inFrame)
	assert.False(t, m.scanIRQ.pending) // not yet; fires at dot offset

	m.TickPPU(269, 50, true) // cycle=269 (default offset): fires the IRQ
	assert.True(t, m.scanIRQ.pending)
}

func TestTickPPU_WrongScanlineNoFire(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.scanIRQ.enabled = true
	m.scanIRQ.latch = 50

	m.TickPPU(0, 51, true)

	assert.False(t, m.scanIRQ.pending)
}

func TestTickPPU_DisabledNoFire(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.scanIRQ.enabled = false
	m.scanIRQ.latch = 50

	m.TickPPU(0, 50, true)

	assert.False(t, m.scanIRQ.pending)
}

func TestTickPPU_OutOfFrameNoFire(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.scanIRQ.enabled = true
	m.scanIRQ.latch = 240

	m.TickPPU(0, 240, true) // scanLine 240 is post-render, not in-frame

	assert.False(t, m.scanIRQ.pending)
	assert.False(t, m.scanIRQ.inFrame)
}

func TestTickPPU_InFrameTracking(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.TickPPU(0, 0, true)
	assert.True(t, m.scanIRQ.inFrame)

	m.TickPPU(0, 239, true)
	assert.True(t, m.scanIRQ.inFrame)

	m.TickPPU(0, 240, true)
	assert.False(t, m.scanIRQ.inFrame)
}

func TestTickPPU_HBlankTracking(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	// New scanline clears HBlank.
	m.scanIRQ.inHBlank = true
	m.TickPPU(0, 100, true)
	assert.False(t, m.scanIRQ.inHBlank)

	// HBlank starts at cycle 257 on a visible scanline.
	m.TickPPU(ppuHBlankStartCycle, 100, true)
	assert.True(t, m.scanIRQ.inHBlank)
}

func TestTickPPU_HBlankNotSetOnVblankLine(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	// Cycle 257 during vblank (scanLine 241) should NOT set HBlank.
	m.TickPPU(ppuHBlankStartCycle, 241, true)
	assert.False(t, m.scanIRQ.inHBlank)
}

func TestTickPPU_CounterTracked(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.TickPPU(0, 75, true)
	assert.Equal(t, int16(75), m.scanIRQ.counter)
}

func TestTickPPU_JitterResetOnFire(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.scanIRQ.enabled = true
	m.scanIRQ.latch = 50
	m.scanIRQ.jitter = 99

	m.TickPPU(0, 50, true)                      // arms the IRQ; jitter unchanged
	assert.Equal(t, byte(99), m.scanIRQ.jitter) // not reset yet

	m.TickPPU(269, 50, true)                   // fires at dot offset; jitter reset
	assert.Equal(t, byte(0), m.scanIRQ.jitter) // reset when IRQ fires
}

func TestNMIVectorRead_ClearsInFrame(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.nmiVectorEnabled = true
	m.nmiAddr = 0x8050
	m.scanIRQ.inFrame = true
	m.scanIRQ.counter = 42
	m.scanIRQ.pending = true

	_ = m.Read(nmiVectorLower)

	assert.False(t, m.scanIRQ.inFrame)
	assert.Equal(t, int16(0), m.scanIRQ.counter)
	assert.False(t, m.scanIRQ.pending)
}

// --- Power-up Defaults ---

func TestPowerUpDefaults(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	assert.Equal(t, byte(0), m.prgMode)
	assert.Equal(t, byte(0), m.prgRAMMode)
	assert.Equal(t, byte(0), m.chrMode)
	assert.Equal(t, byte(0), m.chrSource)
	assert.Equal(t, byte(135), m.scanIRQ.offset)
	assert.Equal(t, byte(1), m.fpgaAutoInc)
}

// --- Nametable Routing Tests (Phase 4) ---

// TestNametableCIRAMFallthrough verifies that with the default CIRAM source the hook
// returns (0, false), allowing the standard CIRAM read/write path to work normally.
func TestNametableCIRAMFallthrough(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	// ntControl[0] == 0 → CIRAM source (default).
	m.NameTableMemory().Write(0x2000, 0xAB)
	assert.Equal(t, byte(0xAB), m.NameTableMemory().Read(0x2000))
}

// TestNametableCHRRAMSource verifies that when a slot is configured for CHR-RAM,
// reads are fetched from the correct 1 KB bank within chrRAM.
func TestNametableCHRRAMSource(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x8000) // 32 KB CHR (large enough for bank 2)

	m.ntBank[1] = 0x02                            // bank 2 → byte offset 0x800
	m.ntControl[1] = ntSourceCHRRAM << ntSrcShift // slot 1 = CHR-RAM
	m.chrRAM[2*ntSlotSize+0x10] = 0x77

	result := m.NameTableMemory().Read(0x2400 + 0x10) // slot 1
	assert.Equal(t, byte(0x77), result)
}

// TestNametableFPGARAMSource verifies that FPGA-RAM source routing uses the
// 2-bit bank field to select a 1 KB page from fpgaRAM.
func TestNametableFPGARAMSource(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.ntBank[2] = 0x01                             // bank 1 → byte offset 0x400
	m.ntControl[2] = ntSourceFPGARAM << ntSrcShift // slot 2 = FPGA-RAM
	m.fpgaRAM[1*ntSlotSize+0x20] = 0x55

	result := m.NameTableMemory().Read(0x2800 + 0x20) // slot 2
	assert.Equal(t, byte(0x55), result)
}

// TestNametableCHRROMSource verifies that CHR-ROM source routing uses the
// 8-bit bank register to select a 1 KB page from chrROM.
func TestNametableCHRROMSource(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000) // 8 KB CHR-ROM

	m.ntBank[3] = 0x01                            // bank 1 → byte offset 0x400
	m.ntControl[3] = ntSourceCHRROM << ntSrcShift // slot 3 = CHR-ROM
	m.chrROM[1*ntSlotSize+0x05] = 0x33

	result := m.NameTableMemory().Read(0x2C00 + 0x05) // slot 3
	assert.Equal(t, byte(0x33), result)
}

// TestNametableCHRRAMBankMask verifies that only 5 bits of ntBank are used for
// CHR-RAM banks (maximum 31 × 1 KB pages addressable).
func TestNametableCHRRAMBankMask(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x8000)

	m.ntBank[0] = 0xFF // high bits should be masked to 0x1F = 31
	m.ntControl[0] = ntSourceCHRRAM << ntSrcShift
	m.chrRAM[31*ntSlotSize+0x00] = 0xBB

	result := m.NameTableMemory().Read(0x2000)
	assert.Equal(t, byte(0xBB), result)
}

// TestNametableFPGARAMBankMask verifies that only 2 bits of ntBank are used for
// FPGA-RAM banks (maximum 3 × 1 KB pages addressable within 4 KB window).
func TestNametableFPGARAMBankMask(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.ntBank[0] = 0xFF // high bits should be masked to 0x03 = 3
	m.ntControl[0] = ntSourceFPGARAM << ntSrcShift
	m.fpgaRAM[3*ntSlotSize+0x00] = 0xCC

	result := m.NameTableMemory().Read(0x2000)
	assert.Equal(t, byte(0xCC), result)
}

// --- Fill Mode Tests (Phase 5) ---

// TestNametableFillModeTile verifies that fill mode returns fillTile for all
// tile-area offsets (< $3C0) regardless of slot source.
func TestNametableFillModeTile(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.fillTile = 0x42
	m.ntControl[0] = ntFillBit // bit 5 only → CIRAM source + fill mode

	// Tile area: offsets 0x000–0x3BF.
	assert.Equal(t, byte(0x42), m.NameTableMemory().Read(0x2000))
	assert.Equal(t, byte(0x42), m.NameTableMemory().Read(0x23BF))
}

// TestNametableFillModeAttr verifies that fill mode returns the replicated 2-bit
// fillAttr value for attribute-area offsets (>= $3C0).
func TestNametableFillModeAttr(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.fillAttr = 0x02 // 2-bit value: 0b10
	m.ntControl[0] = ntFillBit

	// Expected: 0b10 | 0b10<<2 | 0b10<<4 | 0b10<<6 = 0x02|0x08|0x20|0x80 = 0xAA.
	want := byte(0x02 | 0x08 | 0x20 | 0x80)
	assert.Equal(t, want, m.NameTableMemory().Read(0x23C0)) // first attr byte
	assert.Equal(t, want, m.NameTableMemory().Read(0x23FF)) // last attr byte
}

// TestNametableFillModeAttrAllValues verifies fillAttr replication for all
// four possible 2-bit values.
func TestNametableFillModeAttrAllValues(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	m.ntControl[0] = ntFillBit

	tests := []struct {
		attr byte
		want byte
	}{
		{0x00, 0x00},
		{0x01, 0x55},
		{0x02, 0xAA},
		{0x03, 0xFF},
	}
	for _, tt := range tests {
		m.fillAttr = tt.attr
		assert.Equal(t, tt.want, m.NameTableMemory().Read(0x23C0))
	}
}

// TestNametableFillModeOverridesSource verifies that fill mode takes priority
// even when a non-CIRAM source is also configured.
func TestNametableFillModeOverridesSource(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x8000)

	m.fillTile = 0x99
	// Both fill mode (bit 5) and CHR-RAM source (bits [7:6]=01) set simultaneously.
	m.ntControl[0] = ntFillBit | (ntSourceCHRRAM << ntSrcShift)
	m.chrRAM[0x10] = 0x77 // CHR-RAM data that should NOT be returned

	result := m.NameTableMemory().Read(0x2000)
	assert.Equal(t, byte(0x99), result) // fill mode wins
}

// TestNametableNormalizedMirror verifies that addresses in the $3000–$3EFF mirror
// range are handled identically to their $2000–$2EFF counterparts.
func TestNametableNormalizedMirror(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.fillTile = 0x55
	m.ntControl[0] = ntFillBit

	// $3000 mirrors $2000 (slot 0, offset 0).
	assert.Equal(t, byte(0x55), m.NameTableMemory().Read(0x3000))
}

// --- Sprite Extended Mode Tests (Phase 7) ---

// TestSpriteExtMode8x8 verifies that when spriteExtMode is active and
// activeSpriteIndex is set, an 8×8 sprite CHR fetch uses per-sprite banking.
// addr = (spriteBankUpper<<20) | (spriteBankLower[i]<<12) | (address & 0xFFF)
func TestSpriteExtMode8x8(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x8000) // 32 KB CHR-ROM
	m.spriteExtMode = true
	m.activeSpriteIndex = 2  // OAM sprite 2
	m.activeSpriteSize = 8   // 8×8
	m.spriteBankUpper = 0    // upper=0
	m.spriteBankLower[2] = 1 // lower=1 → bank offset 0x1000

	// addr = (0<<20) | (1<<12) | (0x080) = 0x1080
	m.chrROM[0x1080] = 0xAB

	assert.Equal(t, byte(0xAB), m.Read(0x0080))
}

// TestSpriteExtMode8x8UpperBits verifies that spriteBankUpper contributes the high
// address bits in 8×8 mode: addr = (spriteBankUpper<<20) | (lower<<12) | (addr&0xFFF).
func TestSpriteExtMode8x8UpperBits(t *testing.T) {
	chrSize := 4 * 1024 * 1024 // 4 MB CHR-ROM to reach upper address bits
	m := newTestMapper(t, 0x8000, chrSize)
	m.spriteExtMode = true
	m.activeSpriteIndex = 0
	m.activeSpriteSize = 8
	m.spriteBankUpper = 1    // upper=1 → bit 20 set
	m.spriteBankLower[0] = 0 // lower=0

	// addr = (1<<20) | (0<<12) | 0x010 = 0x100010
	m.chrROM[0x100010] = 0xCC

	assert.Equal(t, byte(0xCC), m.Read(0x0010))
}

// TestSpriteExtMode8x16 verifies per-sprite banking in 8×16 mode.
// addr = (spriteBankUpper<<21) | (spriteBankLower[i]<<13) | (address & 0x1FFF)
func TestSpriteExtMode8x16(t *testing.T) {
	chrSize := 4 * 1024 * 1024 // 4 MB CHR-ROM
	m := newTestMapper(t, 0x8000, chrSize)
	m.spriteExtMode = true
	m.activeSpriteIndex = 5  // OAM sprite 5
	m.activeSpriteSize = 16  // 8×16
	m.spriteBankUpper = 0    // upper=0
	m.spriteBankLower[5] = 2 // lower=2 → bank offset 0x4000

	// addr = (0<<21) | (2<<13) | (0x010) = 0x4010
	m.chrROM[0x4010] = 0xDD

	assert.Equal(t, byte(0xDD), m.Read(0x0010))
}

// TestSpriteExtModeDisabled verifies that normal CHR banking is used when
// spriteExtMode is false, even if activeSpriteIndex is set.
func TestSpriteExtModeDisabled(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	m.spriteExtMode = false
	m.activeSpriteIndex = 0 // would trigger sprite ext if mode were enabled
	m.chrROM[0x0010] = 0x55

	assert.Equal(t, byte(0x55), m.Read(0x0010))
}

// TestSpriteExtModeNoActiveSprite verifies that normal CHR banking is used when
// activeSpriteIndex is -1 (no active sprite fetch).
func TestSpriteExtModeNoActiveSprite(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	m.spriteExtMode = true
	m.activeSpriteIndex = -1 // power-up default
	m.chrROM[0x0020] = 0x77

	assert.Equal(t, byte(0x77), m.Read(0x0020))
}

// TestSetActiveSpriteExt verifies the SetActiveSpriteExt method stores index/size
// and that -1 resets the index.
func TestSetActiveSpriteExt(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.SetActiveSpriteExt(7, 16)
	assert.Equal(t, 7, m.activeSpriteIndex)
	assert.Equal(t, 16, m.activeSpriteSize)

	m.SetActiveSpriteExt(-1, 0)
	assert.Equal(t, -1, m.activeSpriteIndex)
}

// --- BG Extended Mode Tests (Phase 11) ---

// TestBGExtMode_CHRBankOverride verifies that when BG ext mode is active
// (ntControl bit 1), the CHR address is remapped using the ext data byte from
// FPGA-RAM: fetchAddr = (addr & 0xFFF) | (extData[5:0] << 12) | (bgExtModeOffset << 18).
func TestBGExtMode_CHRBankOverride(t *testing.T) {
	chrSize := 2 * 1024 * 1024 // 2 MB CHR-ROM to reach high banks
	m := newTestMapper(t, 0x8000, chrSize)

	// Enable BG ext mode on slot 0: bit 1 = BG ext, bits [3:2] = FPGA page 0.
	m.ntControl[0] = 0x02 // bit 1 set, FPGA page 0
	m.bgExtModeOffset = 0 // offset=0: high bits = 0

	// extData at FPGA page 0, tile offset 0 = 0x03 → CHR bank 3.
	m.fpgaRAM[0] = 0x03 // extData[5:0] = 3

	// Simulate a tile index fetch at slot 0, offset 0 to populate bgExtData.
	m.ppuCycle = 1
	m.ppuScanLine = 0
	_ = m.NameTableMemory().Read(0x2000) // triggers updateBGExtState

	// Now a CHR read: fetchAddr = (0x010 & 0xFFF) | (3 << 12) | 0 = 0x3010.
	m.chrROM[0x3010] = 0xEE

	assert.Equal(t, byte(0xEE), m.Read(0x0010))
}

// TestBGExtMode_OffsetBits verifies that bgExtModeOffset contributes the
// high address bits: fetchAddr bits [22:18] = bgExtModeOffset[4:0].
func TestBGExtMode_OffsetBits(t *testing.T) {
	chrSize := 4 * 1024 * 1024 // 4 MB CHR-ROM
	m := newTestMapper(t, 0x8000, chrSize)

	m.ntControl[0] = 0x02 // BG ext mode, FPGA page 0
	m.bgExtModeOffset = 1 // offset=1 → bits [22:18] = 1 = adds 0x40000 to addr

	m.fpgaRAM[0] = 0x00 // extData = 0 → CHR bank 0

	// Trigger tile fetch at slot 0, offset 0.
	m.ppuCycle = 1
	m.ppuScanLine = 0
	_ = m.NameTableMemory().Read(0x2000)

	// fetchAddr = (0x008 & 0xFFF) | (0 << 12) | (1 << 18) = 0x40008
	m.chrROM[0x40008] = 0xBB

	assert.Equal(t, byte(0xBB), m.Read(0x0008))
}

// TestBGExtMode_FPGAPage verifies that bits [3:2] of ntControl select the FPGA
// source page for ext data: page 1 starts at FPGA-RAM offset 0x400.
func TestBGExtMode_FPGAPage(t *testing.T) {
	chrSize := 2 * 1024 * 1024
	m := newTestMapper(t, 0x8000, chrSize)

	// BG ext mode on slot 0, FPGA page 1 (bits [3:2] = 01 → ctrl = 0b00000110 = 0x06).
	m.ntControl[0] = 0x06 // bit 1 (BG ext) + bits [3:2]=01 (FPGA page 1)
	m.bgExtModeOffset = 0

	// Ext data for tile 0 is at FPGA page 1, offset 0 = FPGA-RAM[0x400].
	m.fpgaRAM[0x400] = 0x05 // extData = 5 → CHR bank 5

	m.ppuCycle = 1
	m.ppuScanLine = 0
	_ = m.NameTableMemory().Read(0x2000)

	// fetchAddr = (0x010 & 0xFFF) | (5 << 12) | 0 = 0x5010
	m.chrROM[0x5010] = 0x99

	assert.Equal(t, byte(0x99), m.Read(0x0010))
}

// TestBGExtMode_Disabled verifies that normal CHR banking applies when BG ext
// mode bit is clear in ntControl.
func TestBGExtMode_Disabled(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.ntControl[0] = 0x00 // BG ext mode disabled
	m.chrROM[0x0030] = 0x44

	m.ppuCycle = 1
	m.ppuScanLine = 0
	_ = m.NameTableMemory().Read(0x2000) // triggers updateBGExtState (bgExtActive = false)

	assert.Equal(t, byte(0x44), m.Read(0x0030))
}

// --- Attribute Extended Mode Tests (Phase 12) ---

// TestAttrExtMode_PerTilePalette verifies that when attribute ext mode is active
// (ntControl bit 0), an attribute fetch returns the per-tile palette from ext data
// bits [7:6] replicated across all 4 attribute positions.
func TestAttrExtMode_PerTilePalette(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	// Attr ext mode on slot 0, FPGA page 0 (ctrl = 0x01).
	m.ntControl[0] = 0x01 // bit 0 = attr ext mode, FPGA page 0
	m.fpgaRAM[0] = 0xC0   // extData[7:6] = 0b11 → palette = 3 → 0xFF

	// Simulate a tile fetch first to populate bgTileSlotIdx/bgTileSlotOffset.
	m.ppuCycle = 1
	m.ppuScanLine = 0
	_ = m.NameTableMemory().Read(0x2000) // tile fetch at offset 0

	// Now read the attribute for the same tile.
	val := m.NameTableMemory().Read(0x23C0)
	assert.Equal(t, byte(0xFF), val) // 3|3<<2|3<<4|3<<6 = 0xFF
}

// TestAttrExtMode_AllPalettes verifies palette replication for all four 2-bit
// palette values: 0→0x00, 1→0x55, 2→0xAA, 3→0xFF.
func TestAttrExtMode_AllPalettes(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	m.ntControl[0] = 0x01 // attr ext mode, FPGA page 0

	tests := []struct {
		extData byte
		want    byte
	}{
		{0x00, 0x00}, // palette 0
		{0x40, 0x55}, // palette 1
		{0x80, 0xAA}, // palette 2
		{0xC0, 0xFF}, // palette 3
	}

	for _, tt := range tests {
		// Set ext data for tile at offset 0.
		m.fpgaRAM[0] = tt.extData

		// Simulate tile fetch to cache slot/offset.
		m.ppuCycle = 1
		m.ppuScanLine = 0
		_ = m.NameTableMemory().Read(0x2000)

		val := m.NameTableMemory().Read(0x23C0)
		assert.Equal(t, tt.want, val)
	}
}

// TestAttrExtMode_DisabledFallsThrough verifies that when attr ext bit is clear,
// the normal attribute source routing is used instead.
func TestAttrExtMode_DisabledFallsThrough(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	// Fill mode only (no attr ext), fill attr = 2 → 0xAA.
	m.ntControl[0] = ntFillBit
	m.fillAttr = 0x02

	val := m.NameTableMemory().Read(0x23C0)
	assert.Equal(t, byte(0xAA), val)
}

// TestAttrExtMode_SlotMismatch verifies that attr ext mode only applies to the
// slot that last performed a tile fetch: reads from a different slot fall through.
func TestAttrExtMode_SlotMismatch(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	// Attr ext mode on slot 0.
	m.ntControl[0] = 0x01
	m.fpgaRAM[0] = 0xC0 // palette 3

	// Simulate tile fetch on slot 1 to set bgTileSlotIdx = 1.
	m.ppuCycle = 1
	m.ppuScanLine = 0
	m.ntControl[1] = 0x00                // normal CIRAM on slot 1
	_ = m.NameTableMemory().Read(0x2400) // slot 1 tile fetch

	// Attr read on slot 0 should NOT use ext mode (bgTileSlotIdx = 1 ≠ 0).
	// Slot 0 has no fill mode, CIRAM source → falls through to CIRAM.
	m.NameTableMemory().Write(0x23C0, 0x33)
	val := m.NameTableMemory().Read(0x23C0)
	assert.Equal(t, byte(0x33), val) // normal CIRAM value, not ext palette
}

// TestAttrExtMode_FPGAPage verifies that bits [3:2] of ntControl select the correct
// FPGA page for the attr ext data: page 2 starts at FPGA-RAM offset 0x800.
func TestAttrExtMode_FPGAPage(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	// Attr ext mode on slot 0, FPGA page 2 (bits [3:2] = 10 → ctrl = 0b00001001 = 0x09).
	m.ntControl[0] = 0x09   // bit 0 (attr ext) + bits [3:2]=10 (FPGA page 2)
	m.fpgaRAM[0x800] = 0x40 // page 2, offset 0: palette 1 → 0x55

	m.ppuCycle = 1
	m.ppuScanLine = 0
	_ = m.NameTableMemory().Read(0x2000) // tile fetch at offset 0

	val := m.NameTableMemory().Read(0x23C0)
	assert.Equal(t, byte(0x55), val)
}

// --- ntBank Readback Tests (Issue #6) ---

// TestNtBankReadback verifies that ntBank[0-3] are readable at $4126-$4129 and
// ntBank[4] (window slot) is readable at $412E.
func TestNtBankReadback(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.ntBank[0] = 0x01
	m.ntBank[1] = 0x02
	m.ntBank[2] = 0x03
	m.ntBank[3] = 0x04
	m.ntBank[4] = 0x05

	assert.Equal(t, byte(0x01), m.Read(regNTBankStart))
	assert.Equal(t, byte(0x02), m.Read(regNTBankStart+1))
	assert.Equal(t, byte(0x03), m.Read(regNTBankStart+2))
	assert.Equal(t, byte(0x04), m.Read(regNTBankEnd))
	assert.Equal(t, byte(0x05), m.Read(regNTWindowBank))
}

// TestNtBankAndControlRoundtrip verifies that both ntBank and ntControl registers
// round-trip independently (write then read back the same value).
func TestNtBankAndControlRoundtrip(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.Write(regNTBankStart, 0xAA)    // ntBank[0]
	m.Write(regNTControlStart, 0xBB) // ntControl[0]

	assert.Equal(t, byte(0xAA), m.Read(regNTBankStart))
	assert.Equal(t, byte(0xBB), m.Read(regNTControlStart))
}

// TestESPTXStatusRequiresSend checks that power-up is not a completed transfer.
func TestESPTXStatusRequiresSend(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	assert.Equal(t, byte(0), m.Read(regESPStart))
}

// TestESPRXStatusEmpty verifies that $4191 returns 0x00 (no incoming message)
// so games don't attempt to receive non-existent data.
func TestESPRXStatusEmpty(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	assert.Equal(t, byte(0x00), m.Read(regESPStatus))
}

func TestOAMRoutineEntryPoints(t *testing.T) {
	for _, address := range []uint16{oamRoutineStart, oamSpriteRoutineStart} {
		m := newTestMapper(t, 0x8000, 0x2000)
		assert.Equal(t, byte(0xA9), m.Read(address))
	}
}

// --- Dot-level Scanline IRQ Timing Tests (Issue #5) ---

// TestScanlineIRQFiresAtDotOffset verifies the two-phase fire model: cycle 0 arms
// the IRQ (readyToFire=true) and the IRQ fires at cycle 2*offset-1.
func TestScanlineIRQFiresAtDotOffset(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.scanIRQ.enabled = true
	m.scanIRQ.latch = 10
	m.scanIRQ.offset = 50

	m.TickPPU(0, 10, true) // arm
	assert.False(t, m.scanIRQ.pending)
	assert.True(t, m.scanIRQ.readyToFire)

	m.TickPPU(98, 10, true) // just before offset — should not fire yet
	assert.False(t, m.scanIRQ.pending)

	m.TickPPU(99, 10, true) // at offset — fires
	assert.True(t, m.scanIRQ.pending)
	assert.False(t, m.scanIRQ.readyToFire)
}

// TestScanlineIRQAckClearsReadyToFire verifies that writing $4152 (disable+ack)
// cancels a queued IRQ even if readyToFire was already set.
func TestScanlineIRQAckClearsReadyToFire(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.scanIRQ.enabled = true
	m.scanIRQ.latch = 20
	m.scanIRQ.offset = 80

	m.TickPPU(0, 20, true) // arm: readyToFire=true
	assert.True(t, m.scanIRQ.readyToFire)

	m.Write(regScanIRQAcknowledge, 0x00) // disable + ack
	assert.False(t, m.scanIRQ.readyToFire)
	assert.False(t, m.scanIRQ.pending)

	m.TickPPU(159, 20, true) // would have fired — but readyToFire was cleared
	assert.False(t, m.scanIRQ.pending)
}

// TestScanlineIRQReadyToFireClearedOnNewScanline verifies that readyToFire is cleared
// at the start of each new scanline, preventing stale fires on subsequent scanlines.
func TestScanlineIRQReadyToFireClearedOnNewScanline(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)

	m.scanIRQ.enabled = true
	m.scanIRQ.latch = 30
	m.scanIRQ.offset = 100

	m.TickPPU(0, 30, true) // arm on scanline 30
	assert.True(t, m.scanIRQ.readyToFire)

	m.TickPPU(0, 31, true) // new scanline: clears readyToFire (latch=30 != 31)
	assert.False(t, m.scanIRQ.readyToFire)
	assert.False(t, m.scanIRQ.pending)
}

// --- PRG-RAM Size from Header Tests (Issue #6) ---

// TestPRGRAMSizeFromHeader verifies that a non-zero cart.RAM field sets the
// PRG-RAM buffer to the correct size (8 KB units, aligned to 32 KB).
func TestPRGRAMSizeFromHeader(t *testing.T) {
	t.Helper()

	base := mapperbase.New(&bus.Bus{
		Cartridge: &cartridge.Cartridge{
			PRG: make([]byte, 0x8000),
			CHR: make([]byte, 0x2000),
			RAM: 8, // 8 × 8 KB = 64 KB
		},
		NameTable: nametable.New(cartridge.MirrorHorizontal),
	})
	m, err := New(base)
	assert.NoError(t, err)

	assert.Len(t, m.(*Mapper).prgRAM, 64*1024)
}

// TestPRGRAMSizeDefaultsTo32K verifies that cart.RAM==0 (iNES default) gives 32 KB.
func TestPRGRAMSizeDefaultsTo32K(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000) // cart.RAM = 0 (field zero value)
	assert.Len(t, m.prgRAM, 32*1024)
}

// TestPRGRAMSizeAlignedTo32K verifies that a non-32K-aligned value (e.g. 5 × 8 KB = 40 KB)
// is rounded up to the next 32 KB boundary (64 KB).
func TestPRGRAMSizeAlignedTo32K(t *testing.T) {
	base := mapperbase.New(&bus.Bus{
		Cartridge: &cartridge.Cartridge{
			PRG: make([]byte, 0x8000),
			CHR: make([]byte, 0x2000),
			RAM: 5, // 5 × 8 KB = 40 KB → rounds up to 64 KB
		},
		NameTable: nametable.New(cartridge.MirrorHorizontal),
	})
	m, err := New(base)
	assert.NoError(t, err)

	assert.Len(t, m.(*Mapper).prgRAM, 64*1024)
}

// TestPRGRAMSizeCappedAt512K verifies that cart.RAM values exceeding 512 KB are clamped.
func TestPRGRAMSizeCappedAt512K(t *testing.T) {
	base := mapperbase.New(&bus.Bus{
		Cartridge: &cartridge.Cartridge{
			PRG: make([]byte, 0x8000),
			CHR: make([]byte, 0x2000),
			RAM: 100, // 100 × 8 KB = 800 KB → clamped to 512 KB
		},
		NameTable: nametable.New(cartridge.MirrorHorizontal),
	})
	m, err := New(base)
	assert.NoError(t, err)

	assert.Len(t, m.(*Mapper).prgRAM, 512*1024)
}

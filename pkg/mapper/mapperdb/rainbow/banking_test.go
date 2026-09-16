package rainbow

import (
	"strconv"
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

func TestHighPRGRAMWrites(t *testing.T) {
	for mode := byte(0); mode < 8; mode++ {
		m := newTestMapper(t, 0x8000, 0x2000)
		m.Write(regPRGControl, mode)
		for i := range uint16(8) {
			m.Write(regHighBankUpperStart+i, 0x80)
			m.Write(regHighBankLowerStart+i, byte(i))
		}
		for offset := 0x8000; offset < 0x10000; offset += 0x1000 {
			address := uint16(offset)
			m.Write(address+7, byte(address>>8))
			assert.Equal(t, byte(address>>8), m.Read(address+7))
		}
		m.Write(regHighBankUpperStart, 0)
		m.Write(0x8007, 0xCC)
		assert.Equal(t, byte(0), m.prgROM[7])
	}
}

// The bank registers select units of the current window size. Test every
// window boundary, both chips, upper bank bits, and physical memory wrapping.
func TestPRGAllWindowMappings(t *testing.T) {
	m := newTestMapper(t, 8*1024*1024, 0x2000)
	fillBankTestData(m.prgROM, 0x35)
	fillBankTestData(m.prgRAM, 0xCA)
	windows := [][]int{
		{0x8000}, {0x4000, 0x4000}, {0x4000, 0x2000, 0x2000},
		{0x2000, 0x2000, 0x2000, 0x2000},
		{0x1000, 0x1000, 0x1000, 0x1000, 0x1000, 0x1000, 0x1000, 0x1000},
	}

	for mode := range 8 {
		t.Run(strconv.Itoa(mode), func(t *testing.T) {
			m.Write(regPRGControl, byte(mode))
			for source, data := range [][]byte{m.prgROM, m.prgRAM} {
				address := 0x8000
				for _, size := range windows[min(mode, 4)] {
					register := uint16((address - 0x8000) / 0x1000)
					bank := uint16(0x103) + register
					m.Write(regHighBankUpperStart+register, byte(bank>>8)|byte(source)<<7|0x78)
					m.Write(regHighBankLowerStart+register, byte(bank))
					for _, offset := range []int{0, 1, size - 1} {
						physical := (int(bank)*size + offset) % len(data)
						assert.Equal(t, data[physical], m.Read(uint16(address+offset)))
					}
					address += size
				}
			}
		})
	}
}

func TestPRGLowAllSourcesAndWindows(t *testing.T) {
	m := newTestMapper(t, 8*1024*1024, 0x2000)
	fillBankTestData(m.prgROM, 0x35)
	fillBankTestData(m.prgRAM, 0xCA)
	fillBankTestData(m.fpgaRAM[:], 0x69)

	for mode := range 2 {
		m.Write(regPRGControl, byte(mode)<<7)
		size := 0x2000 >> mode
		for source, data := range [][]byte{m.prgROM, m.prgROM, m.prgRAM, m.fpgaRAM[:]} {
			for window := range 1 << mode {
				bank := uint16(0x103 + window)
				m.Write(regLowBankUpperStart+uint16(window), byte(source)<<6|byte(bank>>8)|0x30)
				m.Write(regLowBankLowerStart+uint16(window), byte(bank))
				for _, offset := range []int{0, 1, size - 1} {
					address := uint16(0x6000 + window*size + offset)
					physical := (int(bank)*size + offset) % len(data)
					assert.Equal(t, data[physical], m.Read(address))
					if source >= 2 {
						m.Write(address, 0x56)
						assert.Equal(t, byte(0x56), data[physical])
					}
				}
			}
		}
	}
}

// The specification defines five CHR modes. Values 5 through 7 select the
// same sixteen 512-byte windows as mode 4.
func TestCHRAllWindowMappings(t *testing.T) {
	m := newTestMapper(t, 0x8000, 8*1024*1024)
	fillBankTestData(m.chrROM, 0x35)
	fillBankTestData(m.chrRAM, 0xCA)

	for mode := range 8 {
		t.Run(strconv.Itoa(mode), func(t *testing.T) {
			size := 0x2000 >> min(mode, 4)
			for source, data := range [][]byte{m.chrROM, m.chrRAM} {
				m.Write(regCHRControl, byte(source)<<6|byte(mode))
				for window := range 0x2000 / size {
					bank := uint16(0x103 + window)
					m.Write(regCHRBankUpperStart+uint16(window), byte(bank>>8))
					m.Write(regCHRBankLowerStart+uint16(window), byte(bank))
					for _, offset := range []int{0, 1, size - 1} {
						address := uint16(window*size + offset)
						physical := (int(bank)*size + offset) % len(data)
						expected := data[physical]
						assert.Equal(t, expected, m.Read(address))
						m.Write(address, 0x56)
						if source == 1 {
							assert.Equal(t, byte(0x56), data[physical])
						} else {
							assert.Equal(t, expected, m.Read(address))
						}
					}
				}
			}
		})
	}
}

func TestBankModeAliases(t *testing.T) {
	for mode := byte(4); mode < 8; mode++ {
		m := newTestMapper(t, 0x8000, 0x2000)
		m.Write(regPRGControl, mode)
		m.Write(regHighBankLowerStart+1, 2)
		m.prgROM[0x2000] = 0xAB
		assert.Equal(t, byte(0xAB), m.Read(0x9000))
		m.Write(regCHRControl, mode)
		m.Write(regCHRBankLowerStart+1, 3)
		m.chrROM[0x600] = 0xCD
		assert.Equal(t, byte(0xCD), m.Read(0x200))
	}
}

func TestFPGAPatternMirroring(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	for mode := byte(0); mode < 8; mode++ {
		m.Write(regCHRControl, 0x80|mode)
		for i := range uint16(16) {
			m.Write(regCHRBankLowerStart+i, 0xFF)
		}
		m.Write(0x1123, 0xAB)
		assert.Equal(t, byte(0xAB), m.fpgaRAM[0x123])
		assert.Equal(t, byte(0xAB), m.Read(0x123))
		assert.Equal(t, byte(0xAB), m.Read(0x1123))
		assert.Equal(t, byte(0), m.fpgaRAM[0x1123])
	}
}

func TestFPGAAutoIncrementValues(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	for increment := range 256 {
		m.Write(regFPGAAutoAddressUpper, 0xFF)
		m.Write(regFPGAAutoAddressLower, 0xFE)
		m.Write(regFPGAAutoIncrement, byte(increment))
		m.Write(regFPGAAutoData, 0xA5)
		assert.Equal(t, byte(0xA5), m.fpgaRAM[0x1FFE])
		next := (0x1FFE + increment) & 0x1FFF
		m.fpgaRAM[next] = 0x5A
		assert.Equal(t, byte(0x5A), m.Read(regFPGAAutoData))
		assert.Equal(t, uint16((next+increment)&0x1FFF), m.fpgaAutoAddr)
	}
}

func fillBankTestData(data []byte, seed byte) {
	for i := range data {
		data[i] = seed ^ byte(i) ^ byte(i>>8) ^ byte(i>>16)
	}
}

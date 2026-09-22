package apu_test

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/retroenv/nesgoemu/pkg/nes"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

const maxROMCycles = 30_000_000

// TestAPU runs the unmodified hardware test programs. The manifest fixes the
// fixture set and its bytes. A missing file or a changed ROM fails the test.
func TestAPU(t *testing.T) {
	t.Parallel()
	manifest, err := os.ReadFile("SHA256SUMS")
	assert.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(manifest)), "\n")
	assert.Len(t, lines, 25)
	for _, line := range lines {
		fields := strings.Fields(line)
		assert.Len(t, fields, 2)
		t.Run(fields[1], func(t *testing.T) {
			t.Parallel()
			data, err := os.ReadFile(fields[1])
			assert.NoError(t, err)
			assert.Equal(t, fields[0], fmt.Sprintf("%x", sha256.Sum256(data)))
			cart, err := cartridge.LoadFile(bytes.NewReader(data))
			assert.NoError(t, err)
			sys, err := nes.NewSystem(nes.NewOptions(nes.WithCartridge(cart), nes.WithSavePath("")))
			assert.NoError(t, err)
			if strings.HasPrefix(fields[1], "apu_2005/") {
				runOlderROM(t, sys)
			} else {
				runROM(t, sys)
			}
		})
	}
}

// runROM checks Blargg's $6000 result protocol. Power-on RAM is not a result.
// Reset requests require at least 100 ms of emulated time before reset.
func runROM(t *testing.T, sys *nes.System) {
	t.Helper()
	running, resetHandled := false, false
	var resetAt uint64
	for sys.CPU.Cycles() < maxROMCycles {
		assert.NoError(t, t.Context().Err())
		stepROM(t, sys, 1024)
		if sys.Bus.Mapper.Read(0x6001) != 0xde || sys.Bus.Mapper.Read(0x6002) != 0xb0 ||
			sys.Bus.Mapper.Read(0x6003) != 0x61 {

			continue
		}
		status := sys.Bus.Mapper.Read(0x6000)
		running = running || status == 0x80 || status == 0x81
		if running && status < 0x80 {
			assert.Equal(t, byte(0), status, "ROM result: "+romText(sys))
			t.Logf("Passed after %d CPU cycles", sys.CPU.Cycles())
			return
		}
		if status != 0x81 {
			resetAt, resetHandled = 0, false
			continue
		}
		if !resetHandled {
			if resetAt == 0 {
				resetAt = sys.CPU.Cycles() + 200_000
			}
			if sys.CPU.Cycles() >= resetAt {
				sys.Reset()
				resetHandled = true
			}
		}
	}
	t.Fatalf("ROM exceeded %d CPU cycles at PC=$%04X; running=%t; text=%q",
		maxROMCycles, sys.PC, running, romText(sys))
}

// The 2005 tests stop in a JMP loop and print their result in the nametable.
// Their success code is $01. The later $6000 protocol uses $00 instead.
func runOlderROM(t *testing.T, sys *nes.System) {
	t.Helper()
	var samePC int
	for sys.CPU.Cycles() < maxROMCycles {
		previous := sys.PC
		_, err := sys.StepSystem()
		assert.NoError(t, err)
		if sys.PC == previous {
			samePC++
		} else {
			samePC = 0
		}
		if samePC == 100 {
			text := sys.Bus.NameTable.Data()[0][:960]
			result := bytes.Trim(text, "\x00 ")
			assert.Equal(t, "$01", string(result), "ROM screen result")
			t.Logf("Passed after %d CPU cycles", sys.CPU.Cycles())
			return
		}
	}
	t.Fatalf("ROM exceeded %d CPU cycles at PC=$%04X", maxROMCycles, sys.PC)
}

func stepROM(t *testing.T, sys *nes.System, cycles uint64) {
	t.Helper()
	end := sys.CPU.Cycles() + cycles
	for sys.CPU.Cycles() < end {
		_, err := sys.StepSystem()
		assert.NoError(t, err)
	}
}

func romText(sys *nes.System) string {
	var text strings.Builder
	for address := uint16(0x6004); address < 0x6800; address++ {
		value := sys.Bus.Mapper.Read(address)
		if value == 0 {
			break
		}
		text.WriteByte(value)
	}
	return text.String()
}

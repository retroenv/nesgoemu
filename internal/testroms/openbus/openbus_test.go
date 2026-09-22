// Package openbus runs the PPU open-bus test ROM by blargg. The ROM tests the
// decay register and the open bus bits of the PPU I/O bus.
package openbus

import (
	"os"
	"testing"

	"github.com/retroenv/nesgoemu/pkg/nes"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

const (
	// resultAddress holds the result of the test. The ROM writes $80 there while
	// it runs and the result code when it finishes, zero means passed.
	// https://github.com/christopherpow/nes-test-roms/blob/master/ppu_open_bus/readme.txt
	resultAddress = 0x6000
	textAddress   = 0x6004

	running   = 0x80
	maxFrames = 60 * 10 // allow twice the expected five-second run time
)

func TestPPUOpenBus(t *testing.T) {
	file, err := os.Open("ppu_open_bus.nes")
	assert.NoError(t, err)

	cart, err := cartridge.LoadFile(file)
	assert.NoError(t, err)
	assert.NoError(t, file.Close())

	sys, err := nes.NewSystem(nes.NewOptions(nes.WithCartridge(cart), nes.WithDisabledGUI()))
	assert.NoError(t, err)

	runUntilResult(t, sys)

	text := readText(sys)
	result := sys.Bus.Mapper.Read(resultAddress)
	assert.Equal(t, byte(0), result, "the test failed: %s", text)
	assert.Contains(t, text, "Passed")
}

// runUntilResult runs the system until the ROM reports its result.
func runUntilResult(t *testing.T, sys *nes.System) {
	t.Helper()

	frames := 0
	started := false

	for frames < maxFrames {
		step, err := sys.StepSystem()
		assert.NoError(t, err)

		if step.FrameCompleted {
			frames++
		}

		result := sys.Bus.Mapper.Read(resultAddress)
		switch {
		case !started:
			started = result == running
		case result != running:
			return
		}
	}

	t.Fatal("the test ROM did not report a result")
}

// readText returns the output text of the ROM, which starts at $6004.
func readText(sys *nes.System) string {
	text := make([]byte, 0, 128)

	for address := uint16(textAddress); address < 0x6200; address++ {
		value := sys.Bus.Mapper.Read(address)
		if value == 0 {
			break
		}
		text = append(text, value)
	}

	return string(text)
}

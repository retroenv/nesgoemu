package nes

import (
	"math"
	"strconv"
	"testing"

	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/nesgoemu/pkg/ppu/mask"
	"github.com/retroenv/retrogolib/arch/system/nes"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/arch/system/nes/register"
	"github.com/retroenv/retrogolib/assert"
)

const (
	snapshotCamericaPRGSize  = 0x20000
	snapshotMXM0CHRSize      = 0x6000
	snapshotNametableAddress = 0x2000
)

func TestSystemStateRoundTrip(t *testing.T) {
	const (
		allAudioChannels  = 0x1f
		stepsAfterRestore = 1000
		stepsBeforeSave   = 700
	)

	for _, mapper := range []uint16{0, 1, 2, 3, 4, 7, 30, 34, 66, 71, 79, 94, 111, 180, 220, 232, rainbowMapperID, 800} {
		t.Run(strconv.Itoa(int(mapper)), func(t *testing.T) {
			sys := newSnapshotTestSystem(t, mapper)
			mutateSnapshotMapper(sys, mapper)
			sys.Bus.PPU.Write(register.PPU_MASK, mask.MASK_BG|mask.MASK_SPR)
			sys.Bus.APU.Write(register.APU_SND_CHN, allAudioChannels)

			for range stepsBeforeSave {
				_, err := sys.StepSystem()
				assert.NoError(t, err)
			}

			data, err := sys.SaveState()
			assert.NoError(t, err)

			next := newSnapshotTestSystem(t, mapper)
			assert.NoError(t, next.LoadState(data))

			restored, err := next.SaveState()
			assert.NoError(t, err)
			assert.Equal(t, data, restored)

			mutateSnapshotMapper(sys, mapper)
			mutateSnapshotMapper(next, mapper)

			for range stepsAfterRestore {
				_, err = sys.StepSystem()
				assert.NoError(t, err)
				_, err = next.StepSystem()
				assert.NoError(t, err)
			}

			before, err := sys.SaveState()
			assert.NoError(t, err)

			after, err := next.SaveState()
			assert.NoError(t, err)
			assert.Equal(t, before, after)
			assert.Error(t, newSnapshotTestSystem(t, mapper).LoadState(data[:len(data)/2]))
		})
	}
}

func newSnapshotTestSystem(t *testing.T, mapper uint16) *System {
	t.Helper()

	const (
		incZeroPageOpcode  = 0xe6
		jmpAbsoluteOpcode  = 0x4c
		loopCounterAddress = 0x10
	)

	cart := cartridge.New()
	cart.Mapper = mapper
	switch mapper {
	case 7, 30, 71, 111, 232:
		cart.CHR = nil
	case 220:
		cart.CHR = make([]byte, snapshotMXM0CHRSize)
	}

	if mapper == 71 || mapper == 232 {
		cart.PRG = make([]byte, snapshotCamericaPRGSize)
	}

	copy(cart.PRG, []byte{incZeroPageOpcode, loopCounterAddress, jmpAbsoluteOpcode, nes.CodeBaseAddress & math.MaxUint8, nes.CodeBaseAddress >> 8})
	sys, err := NewSystem(NewOptions(WithCartridge(cart)))
	assert.NoError(t, err)
	sys.PC = nes.CodeBaseAddress

	return sys
}

func mutateSnapshotMapper(sys *System, mapper uint16) {
	const (
		gtromControlAddress        = 0x5000
		gtromSecondNametable       = 0x20
		mmc3RAMValue               = 0x42
		mmc3RAMWriteProtected      = 0xc0
		nametableFirstValue        = 0xaa
		nametableSecondValue       = 0x55
		rainbowLowBankLowerAddress = 0x4116
	)

	switch mapper {
	case 1:
		sys.Bus.Mapper.Write(nes.CodeBaseAddress, 1)
		sys.Bus.Mapper.Write(nes.CodeBaseAddress, 0)

	case 4:
		sys.Bus.Mapper.Write(mmc3RAMProtectionRegister, mmc3RAMWriteProtected)
		sys.Bus.Memory.Write(mapperbase.PRGRAMStart, mmc3RAMValue)

	case 111:
		sys.Bus.Mapper.Write(gtromControlAddress, gtromSecondNametable)
		sys.Bus.NameTable.Write(snapshotNametableAddress, nametableSecondValue)
		sys.Bus.Mapper.Write(gtromControlAddress, 0)
		sys.Bus.NameTable.Write(snapshotNametableAddress, nametableFirstValue)

	case rainbowMapperID:
		sys.Bus.Mapper.Write(rainbowLowBankLowerAddress, 2)

	default:
		sys.Bus.Mapper.Write(nes.CodeBaseAddress, 1)
	}
}

func TestSystemStateRejectsDifferentHeader(t *testing.T) {
	cart := cartridge.New()
	sys, err := NewSystem(NewOptions(WithCartridge(cart)))
	assert.NoError(t, err)

	data, err := sys.SaveState()
	assert.NoError(t, err)

	cart.Mirror = cartridge.MirrorHorizontal
	other, err := NewSystem(NewOptions(WithCartridge(cart)))
	assert.NoError(t, err)
	assert.ErrorContains(t, other.LoadState(data), "ROM differs")
}

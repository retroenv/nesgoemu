package mapperbase

import (
	"strconv"
	"testing"

	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

const (
	validationMemoryName    = "test memory"
	validationMinimumSize   = 0x2000
	validationMaximumSize   = 0x8000
	validationBankAlignment = 0x2000
	validationRAMSize       = 0x2000
	validationTrainerSize   = 512
)

func TestValidateSize(t *testing.T) {
	for _, size := range []int{-1, 0, 0x1000, 0x2000, 0x3000, 0x4000, 0x6000, 0x8000, 0xa000} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			err := ValidateSize(validationMemoryName, size, validationMinimumSize, validationMaximumSize, validationBankAlignment)
			valid := size == 0x2000 || size == 0x4000 || size == 0x6000 || size == 0x8000
			assert.Equal(t, valid, err == nil)
		})
	}
	assert.Error(t, ValidateSize(validationMemoryName, 1, 0, 2, 0))
	assert.NoError(t, ValidateSize(validationMemoryName, 0, 0, validationMinimumSize, validationBankAlignment))
}

func TestValidatePowerOfTwoSize(t *testing.T) {
	for _, size := range []int{-1, 0, 0x1000, 0x2000, 0x4000, 0x6000, 0x8000, 0x10000} {
		err := ValidatePowerOfTwoSize(validationMemoryName, size, validationMinimumSize, validationMaximumSize)
		valid := size == 0x2000 || size == 0x4000 || size == 0x8000
		assert.Equal(t, valid, err == nil, "size %d", size)
	}
}

func TestValidateSubmapper(t *testing.T) {
	assert.NoError(t, ValidateSubmapper(&cartridge.Cartridge{}, validationMemoryName, 1))
	for _, submapper := range []byte{0, 1, 2, 255} {
		cart := &cartridge.Cartridge{
			NES2: &cartridge.NES2Metadata{
				Submapper: submapper,
			},
		}
		err := ValidateSubmapper(cart, validationMemoryName, 0, 1)
		assert.Equal(t, submapper < 2, err == nil)
	}
}

func TestValidateFixedMirroring(t *testing.T) {
	for _, mode := range []cartridge.MirrorMode{
		cartridge.MirrorHorizontal, cartridge.MirrorVertical, cartridge.Mirror4, cartridge.MirrorSingle0,
	} {
		cart := &cartridge.Cartridge{
			Mirror: mode,
		}
		err := ValidateFixedMirroring(cart, validationMemoryName)
		assert.Equal(t, mode == cartridge.MirrorHorizontal || mode == cartridge.MirrorVertical, err == nil)
	}
}

func TestValidateNoBatteryTrainer(t *testing.T) {
	assert.NoError(t, ValidateNoBatteryTrainer(&cartridge.Cartridge{}, validationMemoryName))
	assert.Error(t, ValidateNoBatteryTrainer(&cartridge.Cartridge{
		Battery: 1,
	}, validationMemoryName))
	assert.Error(t, ValidateNoBatteryTrainer(&cartridge.Cartridge{
		Trainer: make([]byte, validationTrainerSize),
	}, validationMemoryName))
}

func TestValidateROMOnly(t *testing.T) {
	cart := &cartridge.Cartridge{
		Mirror: cartridge.MirrorHorizontal,
	}
	assert.NoError(t, ValidateROMOnly(cart, validationMemoryName))
	cart.NES2 = &cartridge.NES2Metadata{}
	assert.NoError(t, ValidateROMOnly(cart, validationMemoryName))
	cart.NES2.RAMSizes.CHRNonvolatile = validationRAMSize
	assert.Error(t, ValidateROMOnly(cart, validationMemoryName))
	cart.NES2.RAMSizes = cartridge.RAMSizes{}
	cart.NES2.Submapper = 1
	assert.Error(t, ValidateROMOnly(cart, validationMemoryName))
	cart.NES2.Submapper = 0
	cart.Battery = 1
	assert.Error(t, ValidateROMOnly(cart, validationMemoryName))
	cart.Battery = 0
	cart.Mirror = cartridge.Mirror4
	assert.Error(t, ValidateROMOnly(cart, validationMemoryName))
}

func TestValidateRAMSizes(t *testing.T) {
	assert.NoError(t, ValidateRAMSizes(&cartridge.Cartridge{}))
	maximum := int(^uint(0) >> 1)
	for _, sizes := range []cartridge.RAMSizes{
		{
			PRGVolatile: -1,
		},
		{
			PRGNonvolatile: -1,
		},
		{
			CHRVolatile: -1,
		},
		{
			CHRNonvolatile: -1,
		},
		{
			PRGVolatile:    maximum,
			PRGNonvolatile: 1,
		},
		{
			CHRVolatile:    maximum,
			CHRNonvolatile: 1,
		},
	} {
		cart := &cartridge.Cartridge{
			NES2: &cartridge.NES2Metadata{
				RAMSizes: sizes,
			},
		}
		assert.Error(t, ValidateRAMSizes(cart))
	}
}

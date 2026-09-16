package rainbow

import (
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

func TestFlashCommands(t *testing.T) {
	data := make([]byte, 0x100000)
	var fl flash
	flashCommandBytes(&fl, data, 0x80)
	flashUnlockBytes(&fl, data)
	fl.write(data, 0xAAA, 0x10)
	assert.Equal(t, byte(0xFF), data[0])
	for _, value := range []byte{0xF0, 0x0F} {
		flashCommandBytes(&fl, data, 0xA0)
		fl.write(data, 123, value)
	}
	assert.Equal(t, byte(0), data[123])
	flashCommandBytes(&fl, data, 0x90)
	assert.Equal(t, byte(1), fl.read(data, 0))
	assert.Equal(t, byte(0x5B), fl.read(data, 2))
	fl.write(data, 0, 0xF0)
	assert.Equal(t, byte(0xFF), fl.read(data, 0))
	flashCommandBytes(&fl, data, 0x20)
	fl.write(data, 0, 0xA0)
	fl.write(data, 124, 0x55)
	assert.Equal(t, byte(0x55), data[124])
	fl.write(data, 0, 0x90)
	fl.write(data, 0, 0)
	assert.False(t, fl.Bypass)
	fl.write(data, 0xAAA, 0xAA)
	fl.write(data, 0x556, 0x55)
	fl.write(data, 0xAAA, 0xA0)
	fl.write(data, 125, 0)
	assert.Equal(t, byte(0xFF), data[125])
}

func TestFlashSoftwareIDs(t *testing.T) {
	tests := []struct {
		size      int
		deviceID1 byte
		deviceID2 byte
		deviceID3 byte
	}{
		{flashSize1MB, flashDeviceID1MB, flashErasedValue, flashErasedValue},
		{flashSize2MB, flashDeviceID2MB, flashErasedValue, flashErasedValue},
		{flashSize4MB, flashDeviceID4To8MB, flashDeviceID2For4MB, 0},
		{flashSize8MB, flashDeviceID4To8MB, flashDeviceID2For8MB, 0},
	}

	for _, test := range tests {
		data := make([]byte, test.size)
		var fl flash
		flashCommandBytes(&fl, data, flashIDCommand)
		assert.Equal(t, byte(flashManufacturerID), fl.read(data, flashManufacturerIDOffset))
		assert.Equal(t, test.deviceID1, fl.read(data, flashDeviceIDOffset))
		assert.Equal(t, test.deviceID2, fl.read(data, flashDeviceID2Offset))
		assert.Equal(t, test.deviceID3, fl.read(data, flashDeviceID3Offset))
		assert.Equal(t, byte(0), fl.read(data, flashProtectionIDOffset))
		assert.Equal(t, byte(flashErasedValue), fl.read(data, 6))
	}
}

func TestFlashSectorBoundaries(t *testing.T) {
	for _, size := range []int{0x100000, 0x200000, 0x400000, 0x800000} {
		for _, offset := range []int{0, size - 0x10000, size - 0x8000, size - 0x6000, size - 0x4000, size - 1} {
			data := make([]byte, size)
			var fl flash
			flashCommandBytes(&fl, data, 0x80)
			flashUnlockBytes(&fl, data)
			fl.write(data, offset, 0x30)
			start, end := flashSector(size, offset)
			assert.Equal(t, byte(0xFF), data[start])
			assert.Equal(t, byte(0xFF), data[end-1])
			if start > 0 {
				assert.Equal(t, byte(0), data[start-1])
			}
			if end < size {
				assert.Equal(t, byte(0), data[end])
			}
		}
	}
	start, end := flashSector(0x100000, 0xFA123)
	assert.Equal(t, 0xFA000, start)
	assert.Equal(t, 0xFC000, end)
}

func TestMapperFlashIsolation(t *testing.T) {
	m := newTestMapper(t, 0x100000, 0x100000)
	originalPRG := m.Cartridge().PRG[0]
	for _, address := range []uint16{0x8AAA, 0x8555, 0x8AAA} {
		value := byte(0xAA)
		if address == 0x8555 {
			value = 0x55
		}
		m.Write(address, value)
	}
	m.Write(0x8AAA, 0xF0)
	for _, base := range []uint16{0x8000, 0} {
		m.Write(base+0xAAA, 0xAA)
		m.Write(base+0x555, 0x55)
		m.Write(base+0xAAA, 0x90)
		assert.Equal(t, byte(1), m.Read(base))
	}
	m.Write(0x8000, 0xF0)
	assert.Equal(t, originalPRG, m.Read(0x8000))
	assert.Equal(t, byte(1), m.Read(0))
	m.Write(0x8AAA, 0xAA)
	m.Write(0x8555, 0x55)
	m.Write(0x8AAA, 0x80)
	m.Write(0x8AAA, 0xAA)
	m.Write(0x8555, 0x55)
	m.Write(0x8AAA, 0x10)
	assert.Equal(t, byte(0xFF), m.Read(0x8000))
	assert.Equal(t, originalPRG, m.Cartridge().PRG[0])
}

func flashUnlockBytes(fl *flash, data []byte) {
	fl.write(data, 0xAAA, 0xAA)
	fl.write(data, 0x555, 0x55)
}

func flashCommandBytes(fl *flash, data []byte, value byte) {
	flashUnlockBytes(fl, data)
	fl.write(data, 0xAAA, value)
}

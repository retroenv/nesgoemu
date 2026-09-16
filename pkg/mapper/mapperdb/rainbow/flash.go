package rainbow

// PRG flash commands: https://github.com/BrokeStudio/rainbow-net/blob/master/NES/prg-rom-self-flashing.md
// CHR flash commands: https://github.com/BrokeStudio/rainbow-net/blob/master/NES/chr-rom-self-flashing.md

// flashPhase identifies the next byte in a flash command sequence.
type flashPhase byte

const (
	flashIdle flashPhase = iota
	flashUnlock
	flashCommand
	flashProgram
	flashEraseUnlock
	flashEraseConfirm
	flashErase
	flashBypassExit
)

const (
	flashCommandAddressMask  = 0x0FFF
	flashIDAddressMask       = 0x01FF
	flashFirstUnlockAddress  = 0x0AAA
	flashSecondUnlockAddress = 0x0555
	flashFirstUnlockValue    = 0xAA
	flashSecondUnlockValue   = 0x55

	flashReadArrayCommand   = 0xF0
	flashBypassCommand      = 0x20
	flashEraseCommand       = 0x80
	flashIDCommand          = 0x90
	flashProgramCommand     = 0xA0
	flashChipEraseCommand   = 0x10
	flashSectorEraseCommand = 0x30
	flashErasedValue        = 0xFF

	flashManufacturerIDOffset = 0x00
	flashDeviceIDOffset       = 0x02
	flashDeviceID2Offset      = 0x1C
	flashDeviceID3Offset      = 0x1E
	flashProtectionIDOffset   = 0x04
	flashManufacturerID       = 0x01
	flashDeviceID1MB          = 0x5B
	flashDeviceID2MB          = 0x49
	flashDeviceID4To8MB       = 0x7E
	flashDeviceID2For4MB      = 0x0A
	flashDeviceID2For8MB      = 0x10

	flashSize1MB = 0x100000
	flashSize2MB = 0x200000
	flashSize4MB = 0x400000
	flashSize8MB = 0x800000

	flashSectorSize64K = 0x10000
	flashSectorSize32K = 0x8000
	flashSectorSize16K = 0x4000
	flashSectorSize8K  = 0x2000
)

// flash stores command state. The mapper owns the memory.
// The 1/2 MB models use 32/8/8/16 KB top boot sectors.
// The 4/8 MB models use eight 8 KB top boot sectors.
type flash struct {
	Phase  flashPhase
	ID     bool
	Bypass bool
}

func (fl *flash) read(data []byte, offset int) byte {
	if len(data) == 0 {
		return 0
	}
	offset %= len(data)
	if !fl.ID {
		return data[offset]
	}
	switch offset & flashIDAddressMask {
	case flashManufacturerIDOffset:
		return flashManufacturerID
	case flashDeviceIDOffset:
		switch len(data) {
		case flashSize1MB:
			return flashDeviceID1MB
		case flashSize2MB:
			return flashDeviceID2MB
		case flashSize4MB, flashSize8MB:
			return flashDeviceID4To8MB
		}
	case flashDeviceID2Offset:
		switch len(data) {
		case flashSize4MB:
			return flashDeviceID2For4MB
		case flashSize8MB:
			return flashDeviceID2For8MB
		}
	case flashDeviceID3Offset:
		if len(data) == flashSize4MB || len(data) == flashSize8MB {
			return 0
		}
	case flashProtectionIDOffset:
		return 0
	}
	return flashErasedValue
}

func (fl *flash) write(data []byte, offset int, value byte) {
	if len(data) == 0 {
		return
	}
	offset %= len(data)
	if fl.Phase == flashProgram {
		data[offset] &= value
		fl.Phase = flashIdle
		return
	}
	if value == flashReadArrayCommand && !fl.Bypass {
		*fl = flash{}
		return
	}
	fl.advance(data, offset, value)
}

func (fl *flash) advance(data []byte, offset int, value byte) {
	address := offset & flashCommandAddressMask
	phase := fl.Phase
	fl.Phase = flashIdle
	switch phase {
	case flashIdle:
		fl.unlock(address, value)
	case flashUnlock:
		if flashUnlockByte(address, value) {
			fl.Phase = flashCommand
		}
	case flashCommand:
		if address == flashFirstUnlockAddress {
			fl.command(value)
		}
	case flashEraseUnlock:
		if address == flashFirstUnlockAddress && value == flashFirstUnlockValue {
			fl.Phase = flashEraseConfirm
		}
	case flashEraseConfirm:
		if flashUnlockByte(address, value) {
			fl.Phase = flashErase
		}
	case flashErase:
		fl.erase(data, offset, value)
	case flashBypassExit:
		if value == 0 {
			fl.Bypass = false
		}
	}
}

func (fl *flash) unlock(address int, value byte) {
	if fl.Bypass {
		switch value {
		case flashProgramCommand:
			fl.Phase = flashProgram
		case flashIDCommand:
			fl.Phase = flashBypassExit
		}
	} else if address == flashFirstUnlockAddress && value == flashFirstUnlockValue {
		fl.Phase = flashUnlock
	}
}

func (fl *flash) erase(data []byte, offset int, value byte) {
	if offset&flashCommandAddressMask == flashFirstUnlockAddress && value == flashChipEraseCommand {
		eraseFlash(data)
	} else if value == flashSectorEraseCommand {
		start, end := flashSector(len(data), offset)
		eraseFlash(data[start:end])
	}
}

func (fl *flash) command(value byte) {
	switch value {
	case flashBypassCommand:
		fl.Bypass = true
	case flashEraseCommand:
		fl.Phase = flashEraseUnlock
	case flashIDCommand:
		fl.ID = true
	case flashProgramCommand:
		fl.Phase = flashProgram
	}
}

func flashSector(size, offset int) (int, int) {
	start := offset &^ (flashSectorSize64K - 1)
	if start == size-flashSectorSize64K {
		switch size {
		case flashSize1MB, flashSize2MB:
			for _, length := range []int{flashSectorSize32K, flashSectorSize8K, flashSectorSize8K, flashSectorSize16K} {
				if offset < start+length {
					return start, start + length
				}
				start += length
			}
		case flashSize4MB, flashSize8MB:
			start = offset &^ (flashSectorSize8K - 1)
			return start, start + flashSectorSize8K
		}
	}
	return start, min(start+flashSectorSize64K, size)
}

func eraseFlash(data []byte) {
	for i := range data {
		data[i] = flashErasedValue
	}
}

func flashUnlockByte(address int, value byte) bool {
	return address == flashSecondUnlockAddress && value == flashSecondUnlockValue
}

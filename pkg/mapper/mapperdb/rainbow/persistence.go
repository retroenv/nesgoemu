package rainbow

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

var errSaveData = errors.New("invalid Rainbow save data")

const (
	batteryFormatVersion = 1
	serializedDataLimit  = 32 * 1024 * 1024
	romHashSize          = sha256.Size
)

// BatteryBacked reports whether the mapper has data that must persist.
func (m *Mapper) BatteryBacked() bool {
	return true
}

// SaveBattery writes flash and nonvolatile RAM. It does not write the input ROM.
// Call this method while emulation is stopped.
func (m *Mapper) SaveBattery(writer io.Writer) error {
	prg, chr := m.nonvolatileRAM()
	data := batteryData{
		Version: batteryFormatVersion,
		ROM:     m.romHash(),
		PRG:     m.prgROM,
		CHR:     m.chrROM,
		PRGRAM:  prg,
		CHRRAM:  chr,
	}
	if err := json.NewEncoder(writer).Encode(data); err != nil {
		return fmt.Errorf("writing Rainbow save: %w", err)
	}
	return nil
}

// LoadBattery validates all save data before it changes memory.
// Call this method while emulation is stopped.
func (m *Mapper) LoadBattery(reader io.Reader) error {
	var data batteryData
	if err := json.NewDecoder(io.LimitReader(reader, serializedDataLimit)).Decode(&data); err != nil {
		return fmt.Errorf("reading Rainbow save: %w", err)
	}
	prg, chr := m.nonvolatileRAM()
	if data.Version != batteryFormatVersion || data.ROM != m.romHash() || len(data.PRG) != len(m.prgROM) ||
		len(data.CHR) != len(m.chrROM) || len(data.PRGRAM) != len(prg) || len(data.CHRRAM) != len(chr) {

		return errSaveData
	}
	copy(m.prgROM, data.PRG)
	copy(m.chrROM, data.CHR)
	copy(prg, data.PRGRAM)
	copy(chr, data.CHRRAM)
	return nil
}

func (m *Mapper) nonvolatileRAM() ([]byte, []byte) {
	if metadata := m.Cartridge().NES2; metadata != nil {
		sizes := metadata.RAMSizes
		return m.prgRAM[sizes.PRGVolatile:], m.chrRAM[sizes.CHRVolatile:]
	}
	if m.Cartridge().Battery != 0 {
		return m.prgRAM, nil
	}
	return nil, nil
}

func (m *Mapper) romHash() [romHashSize]byte {
	hash := sha256.New()
	_, _ = hash.Write(m.Cartridge().PRG)
	_, _ = hash.Write(m.Cartridge().CHR)
	return [romHashSize]byte(hash.Sum(nil))
}

type batteryData struct {
	Version int
	ROM     [romHashSize]byte
	PRG     []byte
	CHR     []byte
	PRGRAM  []byte
	CHRRAM  []byte
}

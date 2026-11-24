package mxm1_test

import (
	"bytes"
	"encoding/binary"
	"io"
	"io/fs"
	"strings"
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/mapper"
	"github.com/retroenv/nesgoemu/pkg/mapper/internal/mappertest"
	"github.com/retroenv/nesgoemu/pkg/ppu/renderstate"
	"github.com/retroenv/retrogolib/arch/cpu/cpu6502"
	"github.com/retroenv/retrogolib/arch/system/nes"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

const (
	reg128FIFOData       = 0x40f0
	reg128FIFOStatus     = 0x40f1
	reg128PRGSelect      = 0x5011
	reg128PRGBankLow     = 0x5013
	reg128WriteBankLow   = 0x5093
	reg128TransferStatus = 0x5096
	reg128TransferEnable = 0x5097
	reg128MediaSlot      = 0x5098

	fifoSync          = 0x2b
	fileOpenCommand   = 0xc9
	fileReadCommand   = 0xca
	fileReadOnlyMode  = 1
	fileStatusCommand = 0x10

	fileStatusError = 1
	fileStatusOK    = 0

	mediaBlockSize   = 8 * 1024
	missingMediaPath = "missing"

	mapper128CHRSize = 256 * 1024
	mapper128PRGSize = 2 * 1024 * 1024
)

func TestMapper128RequiresMediaOpener(t *testing.T) {
	_, err := mapper.New(mappertest.NewBus(mapper128Cartridge()))
	assert.ErrorContains(t, err, "requires a cartridge media opener")
}

func TestMapper128IdentityReadback(t *testing.T) {
	const identityRegisterStart = 0x5000

	b := mappertest.NewBus(mapper128Cartridge())
	b.OpenCartridgeMedia = func(string) (io.ReaderAt, error) {
		return bytes.NewReader(nil), nil
	}
	m, err := mapper.New(b)
	assert.NoError(t, err)

	inspector := m.(bus.CPUMemoryInspector)
	for index, want := range [6]byte{0x8c} {
		address := identityRegisterStart + uint16(index)
		assert.Equal(t, want, m.Read(address))
		value, ok := inspector.InspectCPU(address)
		assert.True(t, ok)
		assert.Equal(t, want, value)
	}
}

func TestMapper128Transfer(t *testing.T) {
	const (
		blocksPerFrame  = 4
		fineY           = 2
		mediaPath       = "video/frames.dat"
		streamedPattern = 0xe1
		tileColumn      = 1

		descriptorBank         = 8
		descriptorPaletteByte  = 0xc2
		descriptorPaletteIndex = 2*mediaBlockSize + 3

		bankedPRGByte  = 0x41
		firstPRGSlot   = 1
		prgBank        = 0x21
		resetVectorLow = 0x85
	)

	cart := mapper128Cartridge()
	b := mappertest.NewBus(cart)

	media := make([]byte, blocksPerFrame*mediaBlockSize)
	media[tileColumn*tilePatternBytes+fineY] = streamedPattern
	media[descriptorPaletteIndex] = descriptorPaletteByte
	cart.PRG[len(cart.PRG)-(0x10000-cpu6502.ResetAddress)] = resetVectorLow
	cart.PRG[prgBank*prgBankSize] = bankedPRGByte

	b.OpenCartridgeMedia = func(request string) (io.ReaderAt, error) {
		assert.Equal(t, mediaPath, request)
		return bytes.NewReader(media), nil
	}
	m, err := mapper.New(b)
	assert.NoError(t, err)

	m.Write(reg128PRGSelect, firstPRGSlot)
	m.Write(reg128PRGBankLow, prgBank)
	assert.Equal(t, byte(bankedPRGByte), m.Read(nes.CodeBaseAddress))
	assert.Equal(t, byte(resetVectorLow), m.Read(cpu6502.ResetAddress))

	m.Write(reg128TransferEnable, 1)
	writeFileOpen(m, fileReadOnlyMode, mediaPath)
	assertMediaStatus(t, m, fileStatusOK)

	for range blocksPerFrame {
		transferNextMediaBlock(m)
	}
	transfer := m.(bus.MapperErrorSource)
	assert.NoError(t, transfer.MapperError())

	m.Write(regDescriptorMode, extendedModeEnable|attributeModePatternOnly)
	m.Write(regDescriptorBankLow, descriptorBank)
	m.Write(reg128MediaSlot, 0)
	m.(bus.PPUTicker).TickPPU(renderstate.FirstVisibleDot, 0, true)
	_ = b.NameTable.Read(nameTableBase + tileColumn)
	assert.Equal(t, byte(0xff), b.NameTable.Read(attributeTableBase))
	assert.Equal(t, byte(streamedPattern), m.Read(patternPageSize+fineY))

	transferNextMediaBlock(m)
	assert.ErrorIs(t, transfer.MapperError(), io.EOF)
	m.(bus.MapperResetter).Reset()
	assert.NoError(t, transfer.MapperError())
}

func TestMapper128OpenClearsPreviousStatus(t *testing.T) {
	b := mappertest.NewBus(mapper128Cartridge())
	var opened []string
	b.OpenCartridgeMedia = func(request string) (io.ReaderAt, error) {
		opened = append(opened, request)
		if request == missingMediaPath {
			return nil, fs.ErrNotExist
		}
		return bytes.NewReader(nil), nil
	}
	m, err := mapper.New(b)
	assert.NoError(t, err)
	longPath := strings.Repeat("directory/", 29) + "second.bin"

	for _, test := range []struct {
		mode   byte
		path   string
		status byte
	}{
		{mode: 0, path: "ignored", status: fileStatusError},
		{mode: fileReadOnlyMode, path: missingMediaPath, status: fileStatusError},
		{mode: fileReadOnlyMode, path: "first", status: fileStatusOK},
		{mode: fileReadOnlyMode, path: longPath, status: fileStatusOK},
	} {
		writeFileOpen(m, test.mode, test.path)
		assertMediaStatus(t, m, test.status)
	}
	assert.Equal(t, []string{missingMediaPath, "first", longPath}, opened)
}

func TestMapper128ReadReportsOpenError(t *testing.T) {
	b := mappertest.NewBus(mapper128Cartridge())
	b.OpenCartridgeMedia = func(string) (io.ReaderAt, error) {
		return nil, fs.ErrNotExist
	}
	m, err := mapper.New(b)
	assert.NoError(t, err)
	writeFileOpen(m, fileReadOnlyMode, missingMediaPath)
	source := m.(bus.MapperErrorSource)
	assert.NoError(t, source.MapperError())
	requestMediaBlock(m)
	assert.ErrorIs(t, source.MapperError(), fs.ErrNotExist)
	assert.ErrorContains(t, source.MapperError(), missingMediaPath)
}

func mapper128Cartridge() *cartridge.Cartridge {
	return &cartridge.Cartridge{
		PRG: make([]byte, mapper128PRGSize),
		CHR: make([]byte, mapper128CHRSize),

		NES2: &cartridge.NES2Metadata{},

		Mapper: 128,
		Mirror: cartridge.Mirror4,
	}
}

func assertMediaStatus(t *testing.T, m bus.Mapper, status byte) {
	t.Helper()
	const (
		responseEmpty   = 0xc1
		responsePending = 0x41
		statusPrefix    = 0xa5
	)

	writeMediaCommand(m, fileStatusCommand, nil)
	assert.Equal(t, byte(responsePending), m.Read(reg128FIFOStatus))
	assert.Equal(t, byte(statusPrefix), m.Read(reg128FIFOData))
	assert.Equal(t, status, m.Read(reg128FIFOData))
	assert.Equal(t, byte(responseEmpty), m.Read(reg128FIFOStatus))
}

func writeMediaCommand(m bus.Mapper, command byte, payload []byte) {
	for _, value := range []byte{fifoSync, ^byte(fifoSync), command, ^command} {
		m.Write(reg128FIFOData, value)
	}
	for _, value := range payload {
		m.Write(reg128FIFOData, value)
	}
}

func writeFileOpen(m bus.Mapper, mode byte, path string) {
	payload := binary.LittleEndian.AppendUint16([]byte{mode}, uint16(len(path)))
	writeMediaCommand(m, fileOpenCommand, append(payload, path...))
}

func transferNextMediaBlock(m bus.Mapper) {
	m.Write(reg128WriteBankLow, m.Read(reg128TransferStatus))
	requestMediaBlock(m)
}

func requestMediaBlock(m bus.Mapper) {
	writeMediaCommand(m, fileReadCommand, binary.LittleEndian.AppendUint32(nil, mediaBlockSize))
}

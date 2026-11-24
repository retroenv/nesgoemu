package mxm1

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	// The release ROM writes the file commands to $40F0. It sends $2B, $D4,
	// the command byte, and its complement. The ROM requests and reads a
	// status reply after an open or seek command.
	fifoHeaderSize           = 4
	fifoPrefixSize           = 2 // The command byte follows the sync bytes.
	fifoSync                 = 0x2b
	fileOpenCommand          = 0xc9
	fileOpenFieldsSize       = 3 // One mode byte and two path length bytes.
	fileOpenPathLengthOffset = 1
	fileReadCommand          = 0xca
	fileReadOnlyMode         = 1
	fileSeekCommand          = 0xcf
	fileStatusCommand        = 0x10
	fileTransferArgumentSize = 4 // Read length or seek offset.

	fifoResponseEmptyBit = 0x80
	fifoStatusBase       = 0x41
	fileStatusError      = 1
	fileStatusOK         = 0
	fileStatusPrefix     = 0xa5
	fileStatusReplySize  = 2

	mediaBlockSize = 0x2000
)

type mediaFIFO struct {
	command []byte

	response       [fileStatusReplySize]byte
	responseLength int
	status         byte

	armed           bool
	block           [mediaBlockSize]byte
	directFIFO      bool
	transferEnabled bool
	transferErr     error

	openErr  error
	path     string
	position int64
	reader   io.ReaderAt
}

func (m *mapper128) writeMedia(address uint16, value byte) {
	switch address {
	case reg128FIFO:
		m.writeFIFO(value)

	case reg128TransferEnable:
		m.fifo.transferEnabled = value&controlEnable != 0

	case reg128TransferMode:
		m.fifo.directFIFO = value&controlEnable != 0

	case regWriteBankLow:
		m.fifo.armed = m.fifo.transferEnabled && !m.fifo.directFIFO && value == transferReady

	case reg128MediaSlot:
		if m.slot != value {
			m.slot = value
			m.pcm.cursor = 0
		}

	case reg128PCMControl:
		m.pcm.enabled = value&pcmCaptureEnable != 0
		if !m.pcm.enabled {
			m.pcm.irqPhase = pcmIRQWidth
			m.pcm.irqActive = false
			m.SetMapperIRQ(false)
		}
	}
}

func (m *mapper128) writeFIFO(value byte) {
	f := &m.fifo
	if len(f.command) == 0 && value != fifoSync {
		return
	}
	f.command = append(f.command, value)

	switch len(f.command) {
	case fifoPrefixSize:
		if value != ^byte(fifoSync) {
			f.command = f.command[:0]
		}

	case fifoHeaderSize:
		if value != ^f.command[fifoPrefixSize] {
			f.command = f.command[:0]
		}
	}

	if len(f.command) < fifoHeaderSize || len(f.command) != f.commandSize() {
		return
	}

	cmd := f.command[fifoPrefixSize]
	payload := f.command[fifoHeaderSize:]
	f.command = f.command[:0]
	switch cmd {
	case fileOpenCommand:
		m.openMediaFile(payload[0], string(payload[fileOpenFieldsSize:]))

	case fileSeekCommand:
		f.position = int64(binary.LittleEndian.Uint32(payload))

	case fileReadCommand:
		m.readMediaBlock(binary.LittleEndian.Uint32(payload))

	case fileStatusCommand:
		f.response = [fileStatusReplySize]byte{fileStatusPrefix, f.status}
		f.responseLength = len(f.response)
	}
}

func (m *mapper128) openMediaFile(mode byte, path string) {
	f := &m.fifo
	f.reader = nil
	f.path = path
	f.position = 0
	f.status = fileStatusError
	if mode != fileReadOnlyMode || path == "" {
		f.openErr = errors.New("file open requires read mode and a path")
		return
	}

	reader, err := m.openMedia(path)
	if err == nil && reader == nil {
		err = errors.New("media opener returned no reader")
	}

	f.openErr = err
	if err == nil {
		f.status = fileStatusOK
		f.reader = reader
	}
}

func (m *mapper128) readMediaBlock(size uint32) {
	f := &m.fifo
	if f.openErr != nil {
		f.transferErr = fmt.Errorf("opening cartridge media %q: %w", f.path, f.openErr)
		return
	}
	if f.reader == nil || !f.armed || size != mediaBlockSize || f.position%mediaBlockSize != 0 {
		f.transferErr = fmt.Errorf("invalid mapper 128 media block request at offset %d, size %d", f.position, size)
		return
	}

	f.armed = false
	offset := int(f.position % (frameSlotCount * frameSize))
	n, err := f.reader.ReadAt(f.block[:], f.position)
	if n != mediaBlockSize {
		if err == nil {
			err = io.ErrUnexpectedEOF
		}

		f.transferErr = fmt.Errorf("reading cartridge media %q at offset %d: %w", f.path, f.position, err)
		return
	}

	copy(m.work[offset:], f.block[:])
	f.position += mediaBlockSize
}

func (f *mediaFIFO) commandSize() int {
	switch f.command[fifoPrefixSize] {
	case fileReadCommand, fileSeekCommand:
		return fifoHeaderSize + fileTransferArgumentSize

	case fileOpenCommand:
		const (
			pathLengthStart = fifoHeaderSize + fileOpenPathLengthOffset
			pathStart       = fifoHeaderSize + fileOpenFieldsSize
		)

		if len(f.command) < pathStart {
			return pathStart
		}
		return pathStart + int(binary.LittleEndian.Uint16(f.command[pathLengthStart:]))

	default:
		return fifoHeaderSize
	}
}

func (f *mediaFIFO) readResponse() byte {
	if f.responseLength == 0 {
		return 0
	}
	value := f.response[0]
	f.response[0] = f.response[1]
	f.response[1] = 0
	f.responseLength--

	return value
}

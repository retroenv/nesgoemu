package rainbow

import (
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

func TestESPTransferOwnership(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	m.Write(regESPControl, 3)
	m.Write(regESPReceivePage, 0xFF)
	m.Write(regESPTransmitPage, 0xFE)
	m.Write(0x4E00, 1)
	m.Write(0x4E01, 0)
	m.Write(regESPStart, 0)
	assert.Equal(t, byte(0), m.Read(regESPStart))
	m.Write(0x4E01, 6)
	m.ClockCPU(1)
	assert.Equal(t, byte(0x80), m.Read(regESPStart))
	assert.Equal(t, byte(0x80), m.Read(regESPStatus))
	assert.Equal(t, []byte{2, 0, 0}, m.fpgaRAM[0x1F00:0x1F03])
	assert.Equal(t, byte(1), m.Read(regIRQStatus))
	assert.True(t, m.cycleIRQ.lineActive)
	m.Write(regESPStart, 0)
	m.ClockCPU(1)
	assert.Equal(t, byte(0xC0), m.Read(regESPStatus))
	assert.Equal(t, byte(0), m.Read(0x4F01))
	m.Write(regESPStatus, 0)
	assert.False(t, m.cycleIRQ.lineActive)
	assert.Equal(t, byte(0x40), m.Read(regESPStatus))
	m.ClockCPU(1)
	assert.Equal(t, byte(2), m.Read(0x4F01))
	assert.True(t, m.cycleIRQ.lineActive)
}

func TestESPDisabledAndMalformed(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	m.Write(0x4800, 1)
	m.Write(0x4801, 0)
	m.Write(regESPStart, 0)
	m.ClockCPU(2)
	assert.Equal(t, byte(0), m.Read(regESPStatus))
	assert.Equal(t, byte(0), m.Read(regESPStart))
	m.Write(regESPControl, 3)
	m.Write(0x4800, 0)
	m.Write(regESPStart, 0)
	m.ClockCPU(1)
	assert.Nil(t, m.esp.pending)
	assert.Equal(t, byte(0), m.Read(regESPStatus))
	m.replyESP(0, 0)
	m.ClockCPU(1)
	m.Write(regESPControl, 0)
	assert.False(t, m.cycleIRQ.lineActive)
	assert.Equal(t, byte(0), m.Read(regESPStatus))
}

func TestESPQueueCommands(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	m.executeESP([]byte{2, 7})
	m.executeESP([]byte{1})
	m.replyESP(13, 1)
	m.replyESP(13, 2)
	m.replyESP(13, 3)
	m.executeESP([]byte{5, 13, 1})
	assert.Equal(t, [][]byte{{1, 7}, {13, 3}}, m.esp.queue)
	m.executeESP([]byte{4})
	assert.Empty(t, m.esp.queue)
	m.executeESP([]byte{8})
	m.executeESP([]byte{1})
	assert.Equal(t, [][]byte{{1, 0}}, m.esp.queue)
}

func TestESPServerSettingsAndStatus(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	m.executeESP([]byte{byte(espServerSetProtocol), byte(espProtocolUDP)})
	m.executeESP([]byte{byte(espServerSetSettings), 0x12, 0x34, 4, 't', 'e', 's', 't'})
	m.executeESP([]byte{byte(espServerGetSettings)})
	m.executeESP([]byte{byte(espServerConnect)})
	m.executeESP([]byte{byte(espServerGetStatus)})
	assert.Equal(t, espProtocolUDP, m.esp.server.protocol)
	assert.Equal(t, uint16(0x1234), m.esp.server.port)
	assert.Equal(t, []byte("test"), m.esp.server.hostname)
	assert.Equal(t, [][]byte{{byte(espServerSettings), 0x12, 0x34, 4, 't', 'e', 's', 't'}, {byte(espServerStatus), 0}}, m.esp.queue)
	m.executeESP([]byte{byte(espServerSetSettings)})
	m.executeESP([]byte{byte(espServerGetSettings)})
	assert.Equal(t, []byte{byte(espServerSettings)}, m.esp.queue[len(m.esp.queue)-1])
}

func TestESPRandomRanges(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	for _, test := range []struct {
		command, expected []byte
	}{
		{[]byte{17, 42, 42}, []byte{8, 42}},
		{[]byte{19, 0xAB, 0xCD, 0xAB, 0xCD}, []byte{9, 0xAB, 0xCD}},
		{[]byte{17, 1}, []byte{8, 0}},
		{[]byte{19, 1}, []byte{9, 0, 0}},
	} {
		m.executeESP(test.command)
		assert.Equal(t, test.expected, m.esp.queue[len(m.esp.queue)-1])
	}
	low, high, valid := espRandomRange([]byte{17, 9, 3}, false)
	assert.True(t, valid)
	assert.Equal(t, uint32(3), low)
	assert.Equal(t, uint32(9), high)
}

func TestESPMaximumMessageAndIRQSources(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x2000)
	m.Write(regESPControl, 3)
	m.Write(regESPReceivePage, 7)
	payload := make([]byte, 254)
	for index := range payload {
		payload[index] = byte(index)
	}
	m.fpgaRAM[0x1EFF] = 0xAB
	m.replyESP(13, payload...)
	m.ClockCPU(1)
	assert.Equal(t, byte(255), m.Read(0x4F00))
	assert.Equal(t, byte(0xAB), m.Read(0x4EFF))
	assert.Equal(t, payload, m.fpgaRAM[0x1F02:])
	assert.Equal(t, byte(0), m.Read(regScanIRQJitter))
	m.cycleIRQ.enabled, m.cycleIRQ.pending = true, true
	m.Write(regESPStatus, 0)
	assert.True(t, m.cycleIRQ.lineActive)
	m.ackCycleIRQ()
	assert.False(t, m.cycleIRQ.lineActive)
}

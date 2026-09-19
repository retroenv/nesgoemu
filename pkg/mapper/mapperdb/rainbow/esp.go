package rainbow

import "github.com/retroenv/nesgoemu/pkg/feature"

// Specification: https://github.com/BrokeStudio/rainbow-net/blob/master/NES/mapper-doc.md#wi-fi-4190-4194
// Configuration example: https://github.com/BrokeStudio/rainbow-net/blob/master/NES/rainbow-net-code-example.md#configuration
// Transfer example: https://github.com/BrokeStudio/rainbow-net/blob/master/NES/rainbow-net-code-example.md#send-and-receive-data
// The transfer example writes $4192 for its final receive acknowledgement.
// The register specification assigns receive acknowledgement to $4191.

const (
	espRegisterCount       = 5
	espPageMask            = 0x07
	espDebugMask           = 0x07
	espEnableBit           = 0x01
	espIRQEnableBit        = 0x02
	espStatusReceivedBit   = 0x80
	espStatusQueuedBit     = 0x40
	espStatusSentBit       = 0x80
	espMaximumMessageSize  = 255
	espMaximumHostnameSize = 64

	espServerSettingsHeaderSize = 4
	espServerHostnameSizeIndex  = 3
	espRandomSeed               = 0x682
	espRandomByteMaximum        = 1<<8 - 1
	espRandomWordMaximum        = 1<<16 - 1
)

// ESP messages have one length byte, one command byte, and optional payload.
// Length excludes itself, so a maximum-length message occupies one 256-byte page.
// RX ownership passes to the CPU until a write to $4191 acknowledges the message.
// Queued messages must not overwrite that page before acknowledgement.
func (m *Mapper) writeESPRegister(address uint16, value byte) {
	m.MarkFeature(feature.ESPMessages)

	switch address {
	case regESPControl:
		m.espEnabled = value&espEnableBit != 0
		m.wifiIrqEnable = value&espIRQEnableBit != 0
		if !m.espEnabled {
			m.esp.pending, m.esp.queue = nil, nil
			m.esp.received, m.esp.sent = false, false
		}
		m.updateIRQStatus()
	case regESPStatus:
		m.esp.received = false
		m.updateIRQStatus()
	case regESPStart:
		m.startESPTransfer()
	case regESPReceivePage, regESPTransmitPage:
		m.wifiRegs[address-regESPControl] = value & espPageMask
	}
}

func (m *Mapper) startESPTransfer() {
	if !m.espEnabled || m.esp.pending != nil {
		return
	}
	m.esp.sent = false
	start := fpgaFixedRAMOffset + int(m.wifiRegs[regESPTransmitPage-regESPControl])*fpgaPageSize
	length := int(m.fpgaRAM[start])
	// The CPU can reuse its page after this snapshot. A zero length is invalid.
	if length != 0 {
		m.esp.pending = append([]byte(nil), m.fpgaRAM[start+1:start+length+1]...)
	}
}

// clockESP services transfers on the emulation thread. One clock is a scheduling
// approximation, not an ESP UART baud rate or a measured hardware transfer time.
func (m *Mapper) clockESP() {
	if !m.espEnabled {
		return
	}
	if m.esp.pending != nil {
		message := m.esp.pending
		m.esp.pending = nil
		m.executeESP(message)
		m.esp.sent = true
	}
	if !m.esp.received && len(m.esp.queue) != 0 {
		message := m.esp.queue[0]
		m.esp.queue[0] = nil
		m.esp.queue = m.esp.queue[1:]
		start := fpgaFixedRAMOffset + int(m.wifiRegs[regESPReceivePage-regESPControl])*fpgaPageSize
		m.fpgaRAM[start] = byte(len(message))
		copy(m.fpgaRAM[start+1:], message)
		m.esp.received = true
		m.updateIRQStatus()
	}
}

func (m *Mapper) executeESP(message []byte) {
	if len(message) == 0 {
		return
	}
	switch espCommand(message[0]) {
	case espGetStatus:
		m.replyESP(espReady, 0)
	case espGetDebug:
		m.replyESP(espDebugLevel, m.esp.debug)
	case espSetDebug:
		if len(message) == 2 {
			m.esp.debug = message[1] & espDebugMask
		}
	case espClearBuffers:
		m.esp.queue = nil
	case espDropMessages:
		if len(message) == 3 {
			m.dropESPMessages(message[1], int(message[2]))
		}
	case espGetVersion:
		version := "nesgoemu"
		m.replyESP(espFirmwareVersion, append([]byte{byte(len(version))}, version...)...)
	case espRestart:
		m.esp.queue = nil
		m.esp.debug = 0
	case espGetRandomByte, espGetByteRange, espGetRandomWord, espGetWordRange:
		m.replyESPRandom(message)
	case espServerGetStatus, espServerSetProtocol, espServerGetSettings, espServerSetSettings,
		espServerConnect, espServerDisconnect:
		m.executeESPServerCommand(message)
	}
}

func (m *Mapper) executeESPServerCommand(message []byte) {
	switch espCommand(message[0]) {
	case espServerGetStatus:
		m.replyServerStatus()
	case espServerSetProtocol:
		m.configureServerProtocol(message)
	case espServerGetSettings:
		m.replyServerSettings()
	case espServerSetSettings:
		m.configureServerSettings(message)
	case espServerConnect, espServerDisconnect:
		m.esp.server.connected = false
	}
}

func (m *Mapper) replyESP(command espResponse, payload ...byte) {
	m.esp.queue = append(m.esp.queue, append([]byte{byte(command)}, payload...))
}

func (m *Mapper) dropESPMessages(command byte, keep int) {
	remaining := 0
	for _, message := range m.esp.queue {
		if message[0] == command {
			remaining++
		}
	}
	queue := m.esp.queue[:0]
	for _, message := range m.esp.queue {
		if message[0] == command && remaining > keep {
			remaining--
			continue
		}
		queue = append(queue, message)
	}
	clear(m.esp.queue[len(queue):])
	m.esp.queue = queue
}

func (m *Mapper) replyServerStatus() {
	status := byte(espServerDisconnected)
	if m.esp.server.connected {
		status = byte(espServerConnected)
	}
	m.replyESP(espServerStatus, status)
}

func (m *Mapper) configureServerProtocol(message []byte) {
	if len(message) == 2 && message[1] <= byte(espProtocolUDPPool) {
		m.esp.server.protocol = espProtocol(message[1])
	}
}

func (m *Mapper) replyServerSettings() {
	server := m.esp.server
	if len(server.hostname) == 0 || server.port == 0 {
		m.replyESP(espServerSettings)
		return
	}
	m.replyESP(espServerSettings, append([]byte{
		byte(server.port >> registerByteShift), byte(server.port), byte(len(server.hostname)),
	}, server.hostname...)...)
}

func (m *Mapper) configureServerSettings(message []byte) {
	if len(message) == 1 {
		m.esp.server.hostname = nil
		m.esp.server.port = 0
		m.esp.server.connected = false
		return
	}
	if len(message) < espServerSettingsHeaderSize ||
		len(message) != espServerSettingsHeaderSize+int(message[espServerHostnameSizeIndex]) ||
		message[espServerHostnameSizeIndex] > espMaximumHostnameSize {

		return
	}
	m.esp.server.port = uint16(message[1])<<registerByteShift | uint16(message[2])
	m.esp.server.hostname = append(m.esp.server.hostname[:0], message[4:]...)
	m.esp.server.connected = false
}

// replyESPRandom uses reproducible state for deterministic emulation tests.
// This is game randomness, not a source of cryptographic keys.
// Range endpoints are inclusive; reversed endpoints are exchanged by the protocol.
func (m *Mapper) replyESPRandom(message []byte) {
	word := espCommand(message[0]) >= espGetRandomWord
	minimum, maximum := uint32(0), uint32(espRandomByteMaximum)
	if word {
		maximum = espRandomWordMaximum
	}
	valid := true
	if message[0]&1 != 0 {
		minimum, maximum, valid = espRandomRange(message, word)
	}
	var value uint32
	if valid {
		if m.esp.random == 0 {
			m.esp.random = espRandomSeed
		}
		m.esp.random ^= m.esp.random << 13
		m.esp.random ^= m.esp.random >> 17
		m.esp.random ^= m.esp.random << 5
		value = minimum + m.esp.random%(maximum-minimum+1)
	}
	if word {
		m.replyESP(espRandomWord, byte(value>>8), byte(value))
	} else {
		m.replyESP(espRandomByte, byte(value))
	}
}

type espCommand byte

type espResponse byte

const (
	espGetStatus         espCommand  = 0
	espGetDebug          espCommand  = 1
	espSetDebug          espCommand  = 2
	espClearBuffers      espCommand  = 4
	espDropMessages      espCommand  = 5
	espGetVersion        espCommand  = 6
	espRestart           espCommand  = 8
	espGetRandomByte     espCommand  = 16
	espGetByteRange      espCommand  = 17
	espGetRandomWord     espCommand  = 18
	espGetWordRange      espCommand  = 19
	espServerGetStatus   espCommand  = 20
	espServerSetProtocol espCommand  = 22
	espServerGetSettings espCommand  = 23
	espServerSetSettings espCommand  = 24
	espServerConnect     espCommand  = 28
	espServerDisconnect  espCommand  = 29
	espReady             espResponse = 0
	espDebugLevel        espResponse = 1
	espFirmwareVersion   espResponse = 2
	espRandomByte        espResponse = 8
	espRandomWord        espResponse = 9
	espServerStatus      espResponse = 10
	espServerSettings    espResponse = 12
)

type espProtocol byte

const (
	espProtocolTCP espProtocol = iota
	espProtocolSecureTCP
	espProtocolUDP
	espProtocolUDPPool
)

type espServerConnectionState byte

const (
	espServerDisconnected espServerConnectionState = iota
	espServerConnected
)

type espState struct {
	pending  []byte
	queue    [][]byte
	received bool
	sent     bool
	debug    byte
	random   uint32
	server   espServerState
}

type espServerState struct {
	protocol  espProtocol
	port      uint16
	hostname  []byte
	connected bool
}

func espRandomRange(message []byte, word bool) (uint32, uint32, bool) {
	var minimum, maximum uint32
	switch {
	case !word && len(message) == 3:
		minimum, maximum = uint32(message[1]), uint32(message[2])
	case word && len(message) == 5:
		minimum = uint32(message[1])<<registerByteShift | uint32(message[2])
		maximum = uint32(message[3])<<registerByteShift | uint32(message[4])
	default:
		return 0, 0, false
	}
	return min(minimum, maximum), max(minimum, maximum), true
}

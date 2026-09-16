package rainbow

import (
	"encoding/gob"
	"errors"
	"fmt"
	"io"
)

const stateFormatVersion = 2

// SaveState writes Rainbow-owned memory, registers, and transfer state.
// Stop emulation before calling this method. CPU, PPU, and console CIRAM state
// belong to the system and must be saved separately at the same instant.
func (m *Mapper) SaveState(writer io.Writer) error {
	encoder := gob.NewEncoder(writer)
	header := stateHeader{
		Version: stateFormatVersion,
		ROM:     m.romHash(),
	}
	if err := encoder.Encode(header); err != nil {
		return fmt.Errorf("writing Rainbow state header: %w", err)
	}
	for _, field := range m.stateFields() {
		if err := encoder.Encode(field); err != nil {
			return fmt.Errorf("writing Rainbow state: %w", err)
		}
	}
	return nil
}

// LoadState validates a temporary copy before it changes the mapper or IRQ input.
// Stop emulation before calling this method. Loading does not write the save file.
func (m *Mapper) LoadState(reader io.Reader) error {
	decoder := gob.NewDecoder(io.LimitReader(reader, serializedDataLimit))
	var header stateHeader
	if err := decoder.Decode(&header); err != nil {
		return fmt.Errorf("reading Rainbow state header: %w", err)
	}
	if header.Version != stateFormatVersion || header.ROM != m.romHash() {
		return errSaveData
	}
	next := *m
	// The decoder can reuse slice storage. Detach slices to preserve live memory
	// if a later field is truncated or fails validation. Arrays copy by value.
	next.prgROM, next.chrROM, next.prgRAM, next.chrRAM = nil, nil, nil, nil
	next.esp.pending, next.esp.queue = nil, nil
	for _, field := range next.stateFields() {
		if err := decoder.Decode(field); err != nil {
			return fmt.Errorf("reading Rainbow state: %w", err)
		}
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errSaveData
	}
	if !m.validStateMemory(&next) || !next.validStateRegisters() {
		return errSaveData
	}
	*m = next
	m.updateIRQStatus()
	return nil
}

// stateFields defines the version 2 record order. Keep private state private:
// encode its fields explicitly instead of exposing registers as a public API.
// A change to this order or its types requires a new state format version.
func (m *Mapper) stateFields() []any {
	return []any{
		&m.prgMode, &m.prgRAMMode, &m.highBanks, &m.lowBanks,
		&m.chrMode, &m.chrSource, &m.spriteExtMode, &m.windowEnabled, &m.chrBanks,
		&m.prgFlash, &m.chrFlash, &m.prgROM, &m.prgRAM, &m.chrROM, &m.chrRAM,
		&m.fpgaRAM, &m.fpgaBankSelect, &m.fpgaAutoAddr, &m.fpgaAutoInc,
		&m.ntBank, &m.ntControl, &m.fillTile, &m.fillAttr,
		&m.ppuBus.enabled, &m.ppuBus.idle, &m.ppuBus.repeated, &m.ppuBus.lastAddress,
		&m.ppuBus.reads, &m.ppuBus.tiles,
		&m.scanIRQ.enabled, &m.scanIRQ.pending, &m.scanIRQ.readyToFire,
		&m.scanIRQ.latch, &m.scanIRQ.offset, &m.scanIRQ.inFrame,
		&m.scanIRQ.inHBlank, &m.scanIRQ.jitter, &m.scanIRQ.counter,
		&m.cycleIRQ.lineActive, &m.cycleIRQ.parityReset, &m.cycleIRQ.parity,
		&m.cycleIRQ.enabled, &m.cycleIRQ.pending, &m.cycleIRQ.counter,
		&m.cycleIRQ.reloadValue, &m.cycleIRQ.enableAfterAck, &m.cycleIRQ.ackOn4011,
		&m.nmiVectorEnabled, &m.irqVectorEnabled, &m.nmiAddr, &m.irqAddr,
		&m.spriteBankLower, &m.spriteBankUpper, &m.oamSlowPage, &m.oamExtPage,
		&m.oamLimit, &m.oamCode, &m.oamCodeLocked, &m.windowSplitRegs,
		&m.ppuCycle, &m.ppuScanLine, &m.bgExtModeOffset, &m.activeSpriteIndex,
		&m.activeSpriteSize, &m.bgExtActive, &m.bgExtData, &m.bgTileSlotIdx, &m.bgTileSlotOffset,
		&m.espEnabled, &m.wifiIrqEnable, &m.wifiRegs,
		&m.esp.pending, &m.esp.queue, &m.esp.received, &m.esp.sent, &m.esp.debug, &m.esp.random,
		&m.esp.server.protocol, &m.esp.server.port, &m.esp.server.hostname, &m.esp.server.connected,
	}
}

func (m *Mapper) validStateMemory(next *Mapper) bool {
	if len(next.prgROM) != len(m.prgROM) || len(next.chrROM) != len(m.chrROM) ||
		len(next.prgRAM) != len(m.prgRAM) || len(next.chrRAM) != len(m.chrRAM) {
		return false
	}
	if len(next.esp.pending) > espMaximumMessageSize || len(next.esp.server.hostname) > espMaximumHostnameSize ||
		next.wifiRegs[regESPReceivePage-regESPControl] > espPageMask ||
		next.wifiRegs[regESPTransmitPage-regESPControl] > espPageMask {
		return false
	}
	for _, message := range next.esp.queue {
		if len(message) == 0 || len(message) > espMaximumMessageSize {
			return false
		}
	}
	return true
}

func (m *Mapper) validStateRegisters() bool {
	return m.prgMode <= prgMode4KHighest && m.prgRAMMode <= prgRAMMode4K &&
		m.chrMode <= chrMode512Highest && m.chrSource <= chrSourceNT &&
		m.fpgaBankSelect <= fpgaBankMask && m.fpgaAutoAddr < fpgaRAMSize && m.activeSpriteIndex >= -1 &&
		m.activeSpriteIndex < len(m.spriteBankLower) && m.bgTileSlotIdx >= 0 && m.bgTileSlotIdx < len(m.ntBank) &&
		m.validStateOAM() && m.esp.server.protocol <= espProtocolUDPPool &&
		m.prgFlash.Phase <= flashBypassExit && m.chrFlash.Phase <= flashBypassExit
}

func (m *Mapper) validStateOAM() bool {
	return m.oamSlowPage <= oamPageMask && m.oamExtPage <= oamPageMask && m.oamLimit < spriteCount
}

type stateHeader struct {
	Version int
	ROM     [romHashSize]byte
}

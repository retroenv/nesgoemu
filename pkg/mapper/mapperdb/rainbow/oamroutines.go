package rainbow

// Specification: https://github.com/BrokeStudio/rainbow-net/blob/master/NES/mapper-doc.md#auto-generated-oam-procedures

const (
	spriteCount            = 64
	oamBytesPerSprite      = 4
	oamPageMask            = 0x07
	oamLimitMask           = spriteCount - 1
	oamDefaultSlowPage     = 7
	oamDefaultExtendedPage = 6
	oamDefaultLimit        = oamLimitMask
	oamRoutineSize         = fpgaFixedStart - oamRoutineStart

	opcodeLDAImmediate = 0xA9
	opcodeSTAAbsolute  = 0x8D
	opcodeRTS          = 0x60
	ppuOAMDataAddress  = 0x2004
	spriteBankAddress  = regSpriteBankLowerStart
)

// readOAMRoutine returns generated 6502 code. The lock prevents operand reads
// at another entry point from replacing the active routine.
// Revision 1 has two entries. $4286 is ordinary generated code, not a clear entry.
func (m *Mapper) readOAMRoutine(address uint16) byte {
	if !m.oamCodeLocked {
		switch address {
		case oamRoutineStart:
			m.generateOAMSlowUpdate()
		case oamSpriteRoutineStart:
			m.generateSpriteBankUpdate()
		}
	}
	if address >= oamSpriteRoutineStart {
		m.oamCodeLocked = false
	}
	return m.oamCode[address-oamRoutineStart]
}

// generateOAMSlowUpdate does not write OAMADDR. Each byte uses LDA immediate
// and STA absolute (six cycles). RTS adds six cycles, so 64 sprites take 1542
// cycles, excluding the caller's JSR.
func (m *Mapper) generateOAMSlowUpdate() {
	m.oamCodeLocked = true
	code := m.oamCode[:0]
	base := fpgaFixedRAMOffset + int(m.oamSlowPage)*fpgaPageSize
	for i := range (int(m.oamLimit) + 1) * oamBytesPerSprite {
		code = append(code, opcodeLDAImmediate, m.fpgaRAM[base+i], opcodeSTAAbsolute,
			byte(ppuOAMDataAddress&registerLowByteMask), byte(ppuOAMDataAddress>>registerByteShift))
	}
	m.oamCode[len(code)] = opcodeRTS
}

func (m *Mapper) generateSpriteBankUpdate() {
	m.oamCodeLocked = true
	code := m.oamCode[:oamSpriteRoutineStart-oamRoutineStart]
	base := fpgaFixedRAMOffset + int(m.oamExtPage)*fpgaPageSize
	for i := range int(m.oamLimit) + 1 {
		code = append(code, opcodeLDAImmediate, m.fpgaRAM[base+i*oamBytesPerSprite], opcodeSTAAbsolute,
			byte(i), byte(spriteBankAddress>>registerByteShift))
	}
	m.oamCode[len(code)] = opcodeRTS
}

func (m *Mapper) writeOAMRegister(address uint16, value byte) {
	switch address {
	case regOAMSlowPage:
		m.oamSlowPage = value & oamPageMask
	case regOAMExtendedPage:
		m.oamExtPage = value & oamPageMask
	case regOAMLimit:
		m.oamLimit = value & oamLimitMask
	}
}

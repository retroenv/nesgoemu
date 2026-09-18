package rainbow

// GraphicsState is a side-effect-free snapshot of Rainbow graphics registers.
type GraphicsState struct {
	CHRControl       byte
	BackgroundRegion byte
	NametableBanks   [ntSlotCount]byte
	NametableControl [ntSlotCount]byte
	CHRBanks         [chrBankCount]uint16
	SpriteBanks      [spriteCount]byte
	SpriteRegion     byte
}

// InspectGraphics returns the current Rainbow graphics-register state.
func (m *Mapper) InspectGraphics() GraphicsState {
	return GraphicsState{
		CHRControl:       m.readCHRControlReg(),
		BackgroundRegion: m.bgExtModeOffset,
		NametableBanks:   m.ntBank,
		NametableControl: m.ntControl,
		CHRBanks:         m.chrBanks,
		SpriteBanks:      m.spriteBankLower,
		SpriteRegion:     m.spriteBankUpper,
	}
}

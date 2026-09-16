package rainbow

// Specification: https://github.com/BrokeStudio/rainbow-net/blob/master/NES/mapper-doc.md#sprite-extended-mode-4200-4240

// SetActiveSpriteExt is called by the PPU sprite evaluator before fetching each
// sprite's tile data. oamIndex is the 0-63 OAM entry being fetched, or -1 when
// sprite evaluation has finished. spriteSize is 8 (8×8) or 16 (8×16).
// It satisfies the sprites.spriteExtFetcher interface.
func (m *Mapper) SetActiveSpriteExt(oamIndex, spriteSize int) {
	m.activeSpriteIndex = oamIndex
	m.activeSpriteSize = spriteSize
}

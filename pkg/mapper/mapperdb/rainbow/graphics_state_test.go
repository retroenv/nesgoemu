package rainbow

import (
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

func TestInspectGraphicsReturnsRegisterSnapshot(t *testing.T) {
	m := newTestMapper(t, 0x8000, 0x200000)
	m.Write(regCHRControl, 0x23)
	m.Write(regBGExtModeOffset, 0x1f)
	m.Write(regNTBankStart, 2)
	m.Write(regNTControlStart, 3)
	m.Write(regCHRBankUpperStart, 1)
	m.Write(regCHRBankLowerStart, 0x23)
	m.Write(regSpriteBankLowerStart, 7)
	m.Write(regSpriteBankUpper, 1)

	state := m.InspectGraphics()
	assert.Equal(t, byte(0x23), state.CHRControl)
	assert.Equal(t, byte(0x1f), state.BackgroundRegion)
	assert.Equal(t, byte(2), state.NametableBanks[0])
	assert.Equal(t, byte(3), state.NametableControl[0])
	assert.Equal(t, uint16(0x123), state.CHRBanks[0])
	assert.Equal(t, byte(7), state.SpriteBanks[0])
	assert.Equal(t, byte(1), state.SpriteRegion)

	m.Write(regSpriteBankLowerStart, 9)
	assert.Equal(t, byte(7), state.SpriteBanks[0])
}

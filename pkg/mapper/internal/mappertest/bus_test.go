package mappertest

import (
	"testing"

	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

func TestNewBus(t *testing.T) {
	cart := &cartridge.Cartridge{
		Mirror: cartridge.MirrorVertical,
	}
	system := NewBus(cart)
	assert.Equal(t, cart, system.Cartridge)
	assert.Equal(t, cartridge.MirrorVertical, system.NameTable.MirrorMode())
	assert.Equal(t, OpenBusValue, system.OpenBus.OpenBus())
}

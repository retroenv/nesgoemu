package nametable

import (
	"testing"

	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

func TestNameTable(t *testing.T) {
	t.Parallel()

	n := New(cartridge.MirrorHorizontal)
	n.SetVRAM(make([]byte, VramSize))

	n.vram[0] = 1

	value := n.Read(0x2400)
	assert.Equal(t, 1, value)

	n.mirrorMode = cartridge.MirrorVertical
	value = n.Read(0x2400)
	assert.Equal(t, 0, value)

	n.Fetch(0x2000)
	value = n.Value()
	assert.Equal(t, 1, value)
}

func TestMapperHooksAndCIRAM(t *testing.T) {
	n := New(cartridge.MirrorHorizontal)
	n.SetVRAM(make([]byte, VramSize))
	n.SetWriteHook(func(address uint16, value byte) bool {
		if address != 0x2000 {
			return false
		}
		n.WriteCIRAM(0x400, value)
		return true
	})
	n.SetReadHook(func(address uint16) (byte, bool) {
		return n.ReadCIRAM(0x400), address == 0x2000
	})
	n.Write(0x2000, 0xAB)
	assert.Equal(t, byte(0xAB), n.Read(0x2000))
	assert.Equal(t, byte(0xAB), n.ReadCIRAM(0xC00))
	assert.Equal(t, byte(0), n.ReadCIRAM(0))
	n.Write(0x2400, 0xCD)
	assert.Equal(t, byte(0xCD), n.Read(0x2400))
	n.SetReadHook(nil)
	n.SetWriteHook(nil)
	n.Write(0x2000, 0xEF)
	assert.Equal(t, byte(0xEF), n.Read(0x2000))
}

func TestFetchValueDuringWrites(t *testing.T) {
	n := New(cartridge.MirrorHorizontal)
	n.SetVRAM(make([]byte, VramSize))
	n.Write(0x2000, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 1000 {
			n.Fetch(0x2000)
		}
	}()
	for {
		select {
		case <-done:
			assert.Equal(t, byte(1), n.Value())
			return
		default:
			assert.LessOrEqual(t, n.Value(), byte(1))
		}
	}
}

func TestReadHookDuringUpdates(t *testing.T) {
	n := New(cartridge.MirrorHorizontal)
	n.SetVRAM(make([]byte, VramSize))
	n.Write(0x2000, 0x2A)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 1000 {
			n.SetReadHook(func(uint16) (byte, bool) { return 0x7F, true })
			n.SetReadHook(nil)
		}
	}()
	for {
		select {
		case <-done:
			assert.Equal(t, byte(0x2A), n.Read(0x2000))
			return
		default:
			value := n.Read(0x2000)
			assert.True(t, value == 0x2A || value == 0x7F)
		}
	}
}

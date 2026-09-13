package addressing

import (
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

func TestResetPreservesCurrentAddress(t *testing.T) {
	addr := New()
	addr.SetAddress(0x2B)
	addr.SetAddress(0x45)
	addr.SetScroll(0xFF)

	addr.Reset()

	assert.False(t, addr.Latch())
	assert.Equal(t, uint16(0x2B45), addr.Address())

	addr.CopyX()
	addr.CopyY()
	assert.Equal(t, uint16(0), addr.Address())
}

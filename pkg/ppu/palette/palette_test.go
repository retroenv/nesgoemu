package palette

import (
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

func TestPalette(t *testing.T) {
	t.Parallel()

	p := &Palette{}
	p.Write(0, 1)
	value := p.Read(0)
	assert.Equal(t, 1, value)

	p.Write(0x21, 1)
	value = p.Read(1)
	assert.Equal(t, 1, value)
}

func TestReadDuringWrites(t *testing.T) {
	p := New()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 1000 {
			p.Write(0x3F10, 1)
			p.Write(0x3F10, 2)
		}
	}()
	for {
		select {
		case <-done:
			assert.Equal(t, byte(2), p.Read(0x3F00))
			assert.Equal(t, byte(2), p.Data()[0])
			return
		default:
			value := p.Read(0x3F00)
			assert.LessOrEqual(t, value, byte(2))
		}
	}
}

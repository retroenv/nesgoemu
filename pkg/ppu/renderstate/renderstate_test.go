package renderstate

import (
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

func TestTickRenderingSkipsOddFrameDot(t *testing.T) {
	active := &RenderState{
		cycle:    339,
		scanLine: 261,
		frame:    1,
	}
	active.TickRendering(true)
	assert.Equal(t, 0, active.Cycle())
	assert.Equal(t, 0, active.ScanLine())
	assert.Equal(t, uint64(2), active.Frame())

	inactive := &RenderState{
		cycle:    339,
		scanLine: 261,
		frame:    1,
	}
	inactive.TickRendering(false)
	assert.Equal(t, 340, inactive.Cycle())
	assert.Equal(t, 261, inactive.ScanLine())
	assert.Equal(t, uint64(1), inactive.Frame())
}

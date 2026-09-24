package screen

import (
	"image/color"
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

func TestSetPixel(t *testing.T) {
	s := New()
	first := color.RGBA{
		R: 1,
		G: 2,
		B: 3,
		A: 4,
	}
	last := color.RGBA{
		R: 5,
		G: 6,
		B: 7,
		A: 8,
	}
	s.SetPixel(0, 0, first)
	s.SetPixel(Width-1, Height-1, last)
	for _, point := range [][2]int{{-1, 0}, {Width, 0}, {0, -1}, {0, Height}} {
		s.SetPixel(point[0], point[1], color.RGBA{
			R: 255,
			A: 255,
		})
	}
	s.FinishRendering()
	assert.Equal(t, first, s.Image().RGBAAt(0, 0))
	assert.Equal(t, last, s.Image().RGBAAt(Width-1, Height-1))
	assert.Equal(t, color.RGBA{}, s.Image().RGBAAt(1, 0))
}

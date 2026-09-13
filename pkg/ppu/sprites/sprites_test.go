package sprites

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/retrogolib/assert"
)

func TestSpriteFetchTiming(t *testing.T) {
	state := &testRenderState{}
	mapper := &testMapper{}
	sprites := New(nil, mapper, nil, state, &testStatus{})
	sprites.sprites[0] = Sprite{
		y:     0,
		index: 1,
	}

	for i := 1; i < maxSprites; i++ {
		sprites.sprites[i].y = 0xFF
	}

	for cycle := 257; cycle <= 320; cycle++ {
		state.cycle = cycle
		before := len(mapper.reads)
		sprites.Render()
		expected := 0
		if cycle%8 == 5 || cycle%8 == 7 {
			expected = 1
		}
		assert.Equal(t, expected, len(mapper.reads)-before)
	}
	assert.Len(t, mapper.reads, 16)
	assert.Equal(t, uint16(16), mapper.reads[0])
	assert.Equal(t, uint16(24), mapper.reads[1])
	assert.Equal(t, uint16(0x0FF0), mapper.reads[14])
	assert.Equal(t, uint16(0x0FF8), mapper.reads[15])
}

func TestSpriteFetchSkipsVBlank(t *testing.T) {
	state := &testRenderState{
		cycle:    261,
		scanLine: 240,
	}
	mapper := &testMapper{}
	sprites := New(nil, mapper, nil, state, &testStatus{})

	sprites.Render()
	assert.Len(t, mapper.reads, 0)

	state.scanLine = 261
	sprites.Render()
	assert.Equal(t, []uint16{0x0FF0}, mapper.reads)
}

func TestSpritePatternAddress(t *testing.T) {
	sprites := &Sprites{
		spriteSize:         8,
		spritePatternTable: 0x1000,
	}
	sprite := &Sprite{
		index:      2,
		attributes: 0x80,
	}

	assert.Equal(t, uint16(0x1027), sprites.spritePatternAddress(sprite, 0))

	sprites.spriteSize = 16
	sprite.index = 3
	assert.Equal(t, uint16(0x1037), sprites.spritePatternAddress(sprite, 0))
	assert.Equal(t, uint16(0x1027), sprites.spritePatternAddress(sprite, 8))
}

func TestSpritePatternAndZeroHit(t *testing.T) {
	sprite := &Sprite{attributes: 2}
	assert.Equal(t, uint32(0x98888888), spritePattern(sprite, 0x80, 0))
	sprite.attributes |= 0x40
	assert.Equal(t, uint32(0x88888889), spritePattern(sprite, 0x80, 0))

	state := &testRenderState{cycle: 1}
	sprites := New(nil, nil, nil, state, &testStatus{})
	sprites.visibleSpriteCount = 1
	sprites.visibleSprites[0] = 5
	sprites.patterns[0] = 0x10000000

	_, zero, value := sprites.Pixel()
	assert.False(t, zero)
	assert.Equal(t, byte(1), value)
}

type testRenderState struct{ cycle, scanLine int }

func (state *testRenderState) Cycle() int    { return state.cycle }
func (state *testRenderState) ScanLine() int { return state.scanLine }

type testStatus struct{}

func (*testStatus) SetSpriteOverflow(bool) {}

type testMapper struct {
	bus.Mapper
	reads []uint16
}

func (mapper *testMapper) Read(address uint16) byte {
	mapper.reads = append(mapper.reads, address)
	return 0
}

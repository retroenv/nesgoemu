package mxm1

import (
	"bytes"
	"testing"

	"github.com/retroenv/nesgoemu/internal/stateio"
	"github.com/retroenv/retrogolib/assert"
)

func TestStateKeepsEffectiveWindowsAndRegisterLatches(t *testing.T) {
	const (
		readLatchValue = 0x55
		ramValue       = 0xaa
	)
	m, _ := newTestMapper(t)
	m.prgSelect = 2
	m.prgBanks[2] = 7
	m.readLatch = readLatchValue
	m.remapNTFlip801 = true
	m.remapOffset801 = 7
	m.remapPosition801 = 9
	m.fetch.metatileFetchesLeft = 2
	m.fetch.metatileLow = 3
	m.ram[0] = ramValue
	m.cpuWindows[2] = window{
		data:     m.ram,
		offset:   0,
		writable: true,
	}
	m.ppuWindows[0] = window{
		data:     m.chrMemory,
		offset:   0,
		writable: true,
	}
	m.ntWindows[1] = window{
		data:   m.chrROM,
		offset: 0,
	}
	var state bytes.Buffer
	assert.NoError(t, m.TransferState(stateio.NewWriter(&state)))
	next, _ := newTestMapper(t)
	assert.NoError(t, next.TransferState(stateio.NewReader(bytes.NewReader(state.Bytes()))))
	assert.Equal(t, byte(2), next.prgSelect)
	assert.Equal(t, uint16(7), next.prgBanks[2])
	assert.Equal(t, byte(readLatchValue), next.readLatch)
	assert.True(t, next.remapNTFlip801)
	assert.Equal(t, byte(7), next.remapOffset801)
	assert.Equal(t, byte(9), next.remapPosition801)
	assert.Equal(t, byte(2), next.fetch.metatileFetchesLeft)
	assert.Equal(t, byte(3), next.fetch.metatileLow)
	assert.True(t, &next.cpuWindows[2].data[0] == &next.ram[0])
	assert.True(t, &next.ppuWindows[0].data[0] == &next.chrMemory[0])
	assert.Equal(t, byte(ramValue), next.cpuWindows[2].data[0])
	var after bytes.Buffer
	assert.NoError(t, next.TransferState(stateio.NewWriter(&after)))
	assert.Equal(t, state.Bytes(), after.Bytes())
}

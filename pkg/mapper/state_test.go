package mapper

import (
	"bytes"
	"testing"

	"github.com/retroenv/nesgoemu/internal/stateio"
	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/assert"
)

func TestArchiveMapperState(t *testing.T) {
	const ramValue = 0x5a
	m := newTestMapper(t, cartridge.New())
	m.Write(mapperbase.PRGRAMStart, ramValue)
	assert.Equal(t, "nrom-v1", SnapshotFormat(m))

	var legacy, saved bytes.Buffer
	assert.NoError(t, m.(stateio.Device).TransferState(stateio.NewWriter(&legacy)))
	assert.NoError(t, SaveState(&saved, m))
	assert.Equal(t, legacy.Bytes(), saved.Bytes())

	next := newTestMapper(t, cartridge.New())
	assert.NoError(t, LoadState(bytes.NewReader(legacy.Bytes()), next))
	assert.Equal(t, byte(ramValue), next.Read(mapperbase.PRGRAMStart))
}

func TestCustomMapperState(t *testing.T) {
	cart := cartridge.New()
	cart.Mapper = 682
	m := newTestMapper(t, cart)
	assert.Equal(t, "rainbow-v3", SnapshotFormat(m))

	var direct, saved bytes.Buffer
	assert.NoError(t, m.(customSnapshot).SaveState(&direct))
	assert.NoError(t, SaveState(&saved, m))
	assert.Equal(t, direct.Bytes(), saved.Bytes())

	next := newTestMapper(t, cart)
	assert.NoError(t, LoadState(bytes.NewReader(saved.Bytes()), next))
	assert.Error(t, LoadState(bytes.NewReader(saved.Bytes()[:saved.Len()-1]), next))
}

func TestMapperWithoutSnapshotFormat(t *testing.T) {
	m := NewMockMapper(&bus.Bus{})
	var data bytes.Buffer
	assert.Equal(t, "", SnapshotFormat(m))
	assert.Error(t, SaveState(&data, m))
	assert.Error(t, LoadState(&data, m))
}

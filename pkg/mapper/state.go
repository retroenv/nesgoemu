package mapper

import (
	"errors"
	"fmt"
	"io"

	"github.com/retroenv/nesgoemu/internal/stateio"
	"github.com/retroenv/nesgoemu/pkg/bus"
)

// SnapshotFormat returns the mapper's state format. An empty string means that
// the mapper does not support snapshots.
func SnapshotFormat(m bus.Mapper) string {
	switch device := m.(type) {
	case customSnapshot:
		return device.SnapshotFormat()
	case archiveSnapshot:
		return device.SnapshotFormat()
	default:
		return ""
	}
}

// SaveState writes mapper state while emulation is stopped.
func SaveState(w io.Writer, m bus.Mapper) error {
	var err error
	switch device := m.(type) {
	case customSnapshot:
		err = device.SaveState(w)
	case archiveSnapshot:
		err = device.TransferState(stateio.NewWriter(w))
	default:
		return errors.New("mapper does not support snapshots")
	}

	if err != nil {
		return fmt.Errorf("saving mapper state (%s): %w", SnapshotFormat(m), err)
	}
	return nil
}

// LoadState restores mapper state into a new, stopped mapper. The caller must
// check the format and limit the input size before it calls this function.
func LoadState(r io.Reader, m bus.Mapper) error {
	var err error
	switch device := m.(type) {
	case customSnapshot:
		err = device.LoadState(r)
	case archiveSnapshot:
		err = device.TransferState(stateio.NewReader(r))
	default:
		return errors.New("mapper does not support snapshots")
	}

	if err != nil {
		return fmt.Errorf("loading mapper state (%s): %w", SnapshotFormat(m), err)
	}
	return nil
}

type snapshotFormat interface {
	SnapshotFormat() string
}

type archiveSnapshot interface {
	snapshotFormat
	stateio.Device
}

type customSnapshot interface {
	snapshotFormat
	SaveState(io.Writer) error
	LoadState(io.Reader) error
}

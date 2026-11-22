package memory

import (
	"fmt"
	"sync"

	"github.com/retroenv/nesgoemu/internal/stateio"
)

// RAM implements the Random-access memory.
type RAM struct {
	mu     sync.RWMutex // protects data
	offset uint16
	size   uint16
	data   []byte
}

// NewRAM returns a new ram.
func NewRAM(offset, size uint16) *RAM {
	r := &RAM{
		offset: offset,
		size:   size,
	}
	r.Reset()
	return r
}

// Reset resets the RAM content.
func (r *RAM) Reset() {
	r.mu.Lock()
	r.data = make([]byte, r.size)
	r.mu.Unlock()
}

// Read a byte from a memory address.
func (r *RAM) Read(address uint16) byte {
	r.mu.RLock()
	b := r.data[address-r.offset]
	r.mu.RUnlock()

	return b
}

// Write a byte to a memory address.
func (r *RAM) Write(address uint16, value byte) {
	r.mu.Lock()
	r.data[address-r.offset] = value
	r.mu.Unlock()
}

// TransferState transfers stopped RAM state. The system owns the record version.
func (r *RAM) TransferState(archive *stateio.Archive) error {
	if err := archive.Values(&r.offset, &r.size); err != nil {
		return fmt.Errorf("transferring memory state: %w", err)
	}

	if err := archive.Bytes(r.data); err != nil {
		return fmt.Errorf("transferring memory state: %w", err)
	}
	return nil
}

package mcpserver

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"sync"
	"time"

	"github.com/retroenv/retrogolib/log"
)

// machineIDBytes is the random source size of a machine identifier.
const machineIDBytes = 12

// Registry stores the live machines and enforces the cap and the idle expiry.
type Registry struct {
	logger *log.Logger
	max    int
	ttl    time.Duration

	mu       sync.Mutex
	machines map[string]*Machine
}

// add stores a machine. It expires idle machines first and reports an error
// when the cap is reached.
func (r *Registry) add(machine *Machine) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.expireLocked()
	if len(r.machines) >= r.max {
		return fmt.Errorf("machine limit %d reached; destroy a machine first", r.max)
	}
	r.machines[machine.id] = machine
	return nil
}

// get returns a machine and refreshes its idle timer.
func (r *Registry) get(id string) (*Machine, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.expireLocked()
	machine, ok := r.machines[id]
	if !ok {
		return nil, unknownMachineError(id, r.ttl)
	}
	machine.touch()
	return machine, nil
}

// remove deletes a machine. It reports false when the identifier is unknown.
func (r *Registry) remove(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.machines[id]; !ok {
		return false
	}
	delete(r.machines, id)
	return true
}

// list returns the stored machines in map order.
func (r *Registry) list() []*Machine {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.expireLocked()
	machines := make([]*Machine, 0, len(r.machines))
	for _, machine := range r.machines {
		machines = append(machines, machine)
	}
	return machines
}

// expireLocked removes the machines that reached the idle limit. The caller
// holds the registry mutex.
func (r *Registry) expireLocked() {
	now := time.Now()
	for id, machine := range r.machines {
		if now.Sub(machine.lastUsed()) > r.ttl {
			r.logger.Info("Expiring idle machine", log.String("machine", id))
			delete(r.machines, id)
		}
	}
}

// newRegistry creates a registry with a machine cap and an idle expiry.
func newRegistry(logger *log.Logger, maximum int, ttl time.Duration) *Registry {
	return &Registry{
		logger:   logger,
		max:      maximum,
		ttl:      ttl,
		machines: map[string]*Machine{},
	}
}

// unknownMachineError builds the error for a missing machine. The message
// tells the agent to recreate the machine.
func unknownMachineError(id string, ttl time.Duration) error {
	return fmt.Errorf("unknown machine_id %q; it may have expired after %s idle "+
		"or the server restarted, call create_machine", id, ttl)
}

// newMachineID returns a random 12-byte base32 identifier.
func newMachineID() (string, error) {
	var buffer [machineIDBytes]byte
	if _, err := rand.Read(buffer[:]); err != nil {
		return "", fmt.Errorf("generating machine id: %w", err)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buffer[:]), nil
}

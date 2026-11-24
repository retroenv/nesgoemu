package mcpserver

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/retroenv/retrogolib/log"
	"github.com/retroenv/retrogolib/mcp"
)

const waitingForMachineCreationFormat = "waiting to create a machine: %w"

// Default limits for the server.
const (
	defaultMaxMachines  = 4
	defaultMachineTTL   = 30 * time.Minute
	defaultRunTimeout   = 10 * time.Second
	maxRunTimeout       = 2 * time.Minute
	defaultReadLength   = 256
	maxReadLength       = 4096
	maxStepInstructions = 1_000_000

	defaultImageScale = 1
	maxImageScale     = 4
)

const instructionsFormat = "This server drives a headless NES emulator. An emulator " +
	"instance is a machine: create_machine returns a machine_id, and every other " +
	"tool takes that machine_id. A machine expires after %s without a call, " +
	"so call create_machine again if a call reports an unknown machine_id.\n" +
	"Emulation only advances inside tools that run time: step, run_frames, run_until, " +
	"set_buttons with hold_frames, record_audio, restore, fork_machine, " +
	"and the analysis tools that take a frame count. No tool emulates in the background, " +
	"so runs are deterministic and repeated calls give the same result.\n" +
	"Addresses accept an integer, \"$C000\", or \"0xC000\". The prg and chr spaces of " +
	"read_memory use a physical offset in the offset argument. A relative file path " +
	"starts at the first allowed root directory.\n" +
	"Every read tool is side-effect free: reading the PPU status register does not " +
	"clear it or change any state. Use read_memory for CPU, PPU, OAM, PRG, CHR, and " +
	"RAM views.\n" +
	"Tracing is off until trace_start, because it slows emulation. run_until and step " +
	"return a trace tail only while tracing is on.\n" +
	"checkpoint names the current point of the event log of inputs and pokes. restore and " +
	"fork_machine replay the log to a checkpoint_id for an exact copy.\n" +
	"Use inspect_machine with selected sections for a stopped-state snapshot. " +
	"step requires mode instructions, over, or out. " +
	"Typical loop: run_frames with inputs and capture for a screenshot and bounded memory reads, decide, and repeat."

// handleIDBytes is the random source size of a handle.
const handleIDBytes = 12

// Options configures the MCP server.
type Options struct {
	Name    string
	Version string

	MachineTTL  time.Duration
	MaxMachines int
	Roots       []string
}

// Server exposes the emulator through the MCP protocol.
type Server struct {
	loads    chan struct{}
	logger   *log.Logger
	mcp      *mcp.Server
	registry *Registry
	roots    *roots
}

// Registry stores the live machines and enforces the cap and the idle expiry.
type Registry struct {
	logger *log.Logger
	max    int
	ttl    time.Duration

	machines map[string]*Machine
	mu       sync.Mutex
}

// New creates an MCP server, registers the tools and resources, and returns it.
func New(logger *log.Logger, opts Options) (*Server, error) {
	if logger == nil {
		logger = log.NewNop()
	}

	root, err := newRoots(opts.Roots)
	if err != nil {
		return nil, err
	}

	maxMachines := opts.MaxMachines
	if maxMachines <= 0 {
		maxMachines = defaultMaxMachines
	}

	ttl := opts.MachineTTL
	if ttl <= 0 {
		ttl = defaultMachineTTL
	}

	server := &Server{
		loads:    make(chan struct{}, 1),
		logger:   logger,
		registry: newRegistry(logger, maxMachines, ttl),
		roots:    root,
	}
	server.mcp = mcp.New(logger, mcp.Options{
		Name:         opts.Name,
		Version:      opts.Version,
		Instructions: instructions(ttl),
	})
	if err := server.registerTools(); err != nil {
		return nil, err
	}
	if err := server.registerResources(); err != nil {
		return nil, err
	}
	return server, nil
}

// Serve reads MCP messages from r and writes the responses to w.
// Serve closes r. The Close method of r must stop a pending read.
func (s *Server) Serve(ctx context.Context, r io.ReadCloser, w io.Writer) error {
	if err := s.mcp.Serve(ctx, r, w); err != nil {
		return fmt.Errorf("serving MCP protocol: %w", err)
	}
	return nil
}

// Handler returns the streamable HTTP handler for the server.
func (s *Server) Handler() http.Handler {
	return s.mcp.Handler()
}

// acquireLoad bounds concurrent ROM loads and replays that create machines.
func (s *Server) acquireLoad(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf(waitingForMachineCreationFormat, err)
	}

	select {
	case s.loads <- struct{}{}:
		return nil
	case <-ctx.Done():
		return fmt.Errorf(waitingForMachineCreationFormat, ctx.Err())
	}
}

func (s *Server) releaseLoad() {
	<-s.loads
}

// add stores a machine. It expires idle machines first and reports an error
// when the cap is reached.
func (r *Registry) add(machine *Machine) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.checkCapacityLocked(); err != nil {
		return err
	}
	r.machines[machine.id] = machine

	return nil
}

// checkCapacity reports an error when the cap is reached. A tool that creates
// a machine calls it before it loads a ROM or replays an event log, so a full
// registry rejects the call before the expensive work.
func (r *Registry) checkCapacity() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.checkCapacityLocked()
}

// checkCapacityLocked expires idle machines and checks the cap. The caller
// holds the registry mutex.
func (r *Registry) checkCapacityLocked() error {
	r.expireLocked()
	if len(r.machines) >= r.max {
		return fmt.Errorf("machine limit %d reached; destroy a machine first", r.max)
	}
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
		if len(machine.requests) == 0 && now.Sub(machine.lastUsed()) > r.ttl {
			r.logger.Info("Expiring idle machine", log.String("machine", id))
			delete(r.machines, id)
		}
	}
}

// newRegistry creates a registry with a machine cap and an idle expiry.
func newRegistry(logger *log.Logger, maximum int, ttl time.Duration) *Registry {
	return &Registry{
		logger: logger,
		max:    maximum,
		ttl:    ttl,

		machines: map[string]*Machine{},
	}
}

// newHandleID returns a random base32 handle.
func newHandleID() (string, error) {
	var buffer [handleIDBytes]byte
	if _, err := rand.Read(buffer[:]); err != nil {
		return "", fmt.Errorf("generating handle id: %w", err)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buffer[:]), nil
}

// instructionsFormat is the natural-language guidance the server sends to
// agents. The format verb is the idle time limit of a machine.
// instructions returns the guidance text for a machine idle time limit.
func instructions(ttl time.Duration) string {
	return fmt.Sprintf(instructionsFormat, ttl)
}

// unknownMachineError builds the error for a missing machine. The message
// tells the agent to recreate the machine.
func unknownMachineError(id string, ttl time.Duration) error {
	return fmt.Errorf("unknown machine_id %q; it may have expired after %s idle "+
		"or the server restarted, call create_machine", id, ttl)
}

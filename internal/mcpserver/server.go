package mcpserver

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/retroenv/retrogolib/log"
	"github.com/retroenv/retrogolib/mcp"
)

// Default limits for the server.
const (
	defaultMaxMachines  = 4
	defaultMachineTTL   = 30 * time.Minute
	defaultRunTimeout   = 10 * time.Second
	maxRunTimeout       = 2 * time.Minute
	maxReadLength       = 4096
	maxStepInstructions = 1_000_000
)

// instructions is the natural-language guidance the server sends to agents.
const instructions = "This server drives a headless NES emulator. An emulator " +
	"instance is a machine: create_machine returns a machine_id, and every other " +
	"tool takes that machine_id. A machine expires after 30 minutes without a call, " +
	"so call create_machine again if a call reports an unknown machine_id.\n" +
	"Emulation only advances inside tools that run time: step, run_frames, run_until, " +
	"step_over, step_out, set_buttons with hold_frames, record_audio, restore, fork_machine, " +
	"and the analysis tools that take a frame count. No tool emulates in the background, " +
	"so runs are deterministic and repeated calls give the same result.\n" +
	"Addresses accept an integer, \"$C000\", or \"0xC000\". The prg and chr spaces of " +
	"read_memory use a physical offset in the offset argument. A relative file path " +
	"starts at the first allowed root directory.\n" +
	"Every read tool is side-effect free: reading the PPU status register does not " +
	"clear it or change any state. Use read_memory for CPU, PPU, OAM, PRG, CHR, and " +
	"RAM views.\n" +
	"Tracing is off until trace_start, because it slows emulation. run_until, step_over, " +
	"and step_out return a trace tail only while tracing is on.\n" +
	"checkpoint names the current point of the event log of inputs and pokes. restore and " +
	"fork_machine replay the log to a checkpoint_id for an exact copy.\n" +
	"Typical loop: run_frames with inputs, screenshot, read_memory, decide, and repeat."

// Options configures the MCP server.
type Options struct {
	Name        string
	Version     string
	Roots       []string
	MaxMachines int
	MachineTTL  time.Duration
}

// Server exposes the emulator through the MCP protocol.
type Server struct {
	logger   *log.Logger
	registry *Registry
	roots    *roots
	mcp      *mcp.Server
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
		logger:   logger,
		registry: newRegistry(logger, maxMachines, ttl),
		roots:    root,
	}
	server.mcp = mcp.New(logger, mcp.Options{
		Name:         opts.Name,
		Version:      opts.Version,
		Instructions: instructions,
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
func (s *Server) Serve(ctx context.Context, r io.Reader, w io.Writer) error {
	if err := s.mcp.Serve(ctx, r, w); err != nil {
		return fmt.Errorf("serving MCP protocol: %w", err)
	}
	return nil
}

// Handler returns the streamable HTTP handler for the server.
func (s *Server) Handler() http.Handler {
	return s.mcp.Handler()
}

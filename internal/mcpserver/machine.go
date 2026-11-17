package mcpserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/crc32"
	"image"
	"image/draw"
	"image/png"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/retroenv/nesgoemu/pkg/controller"
	"github.com/retroenv/nesgoemu/pkg/nes"
	"github.com/retroenv/nesgoemu/pkg/ppu"
	"github.com/retroenv/retrogolib/arch/cpu/cpu6502"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/log"
)

// stepIterationFactor and stepIterationBase bound the loop that services
// interrupts between instructions. They stop a jammed CPU from spinning.
const (
	stepIterationFactor = 10
	stepIterationBase   = 1000
)

// Machine owns one system and serializes every access to it.
type Machine struct {
	id      string
	rom     string
	romHash string

	battery     bool
	batteryData []byte
	entrypoint  int
	palette     []byte // display palette file bytes; nil selects the default

	logger *log.Logger

	mu       sync.Mutex
	requests chan struct{}
	sys      *nes.System

	breakpoints *nes.BreakpointSet
	stopPC      uint16
	// stoppedAtExec is true when the last debug run stopped at an exec
	// breakpoint at stopPC. The next run continues past it.
	stoppedAtExec bool

	callStack  *callStackRecorder
	cdl        *nes.CDLRecorder
	cdlEnabled bool
	traceRing  *nes.TraceRing

	checkpoints  []checkpointRecord
	events       []machineEvent
	historyBytes int

	bankWrites []bankWrite
	searches   map[string]*memorySearch

	created  time.Time
	modified atomic.Int64
}

// touch marks the machine as recently used.
func (m *Machine) touch() {
	m.modified.Store(time.Now().UnixNano())
}

// lastUsed returns the time of the most recent use.
func (m *Machine) lastUsed() time.Time {
	return time.Unix(0, m.modified.Load())
}

// frame returns the number of completed PPU frames.
func (m *Machine) frame() uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.sys.Bus.PPU.Frame()
}

// reset resets the system to its power-on state. The event is recorded before
// the reset, so a replay applies it at the same cycle.
func (m *Machine) reset() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.checkHistoryCapacity(1, 0); err != nil {
		return err
	}
	m.recordReset()
	m.sys.Reset()
	m.callStack.clear()

	return nil
}

// hardReset recreates the system from the original ROM and boot options. It
// clears the event log and the analysis state of the old system.
func (m *Machine) hardReset(ctx context.Context, logger *log.Logger) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sys, _, err := loadSystem(ctx, logger, m.rom, m.romHash, m.entrypoint, m.battery, m.palette, m.traceRing, &m.batteryData)
	if err != nil {
		return err
	}
	m.sys = sys
	m.events = nil
	m.historyBytes = 0
	m.checkpoints = nil
	m.bankWrites = nil
	m.searches = nil
	m.attach(nil)

	return nil
}

// step executes up to count instructions and reports the CPU state around them.
func (m *Machine) step(ctx context.Context, count uint64) (stepResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := stepResult{
		Before: m.sys.State(),
	}
	result.BeforeFrame = m.sys.Bus.PPU.Frame()
	result.BeforeBank, result.BeforeOffset, _ = m.sys.PRGLocation(result.Before.PC)
	frameBefore := result.BeforeFrame
	limit := count*stepIterationFactor + stepIterationBase
	iterations := uint64(0)

	err := m.sys.Guard(func() error {
		for result.Instructions < count && iterations < limit {
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("step stopped: %w", err)
			}
			iterations++
			step, err := m.sys.StepTraced()
			if err != nil {
				return fmt.Errorf("executing step: %w", err)
			}
			if step.InstructionExecuted {
				result.Instructions++
			}
		}

		return nil
	})
	result.After = m.sys.State()
	result.AfterFrame = m.sys.Bus.PPU.Frame()
	result.AfterBank, result.AfterOffset, _ = m.sys.PRGLocation(result.After.PC)
	result.Frames = result.AfterFrame - frameBefore
	if err != nil {
		return result, fmt.Errorf("stepping system: %w", err)
	}
	return result, nil
}

// runFrames advances the system until the frame goal or the wall-clock timeout.
func (m *Machine) runFrames(ctx context.Context, frames uint64, inputs []controllerInput,
	timeout time.Duration) (runFramesResult, error) {

	m.mu.Lock()
	defer m.mu.Unlock()

	return m.runFramesLocked(ctx, frames, inputs, timeout)
}

// runFramesLocked advances the system. The caller holds the machine mutex.
func (m *Machine) runFramesLocked(ctx context.Context, frames uint64, inputs []controllerInput,
	timeout time.Duration) (runFramesResult, error) {

	if err := ctx.Err(); err != nil {
		return runFramesResult{}, fmt.Errorf("starting run: %w", err)
	}
	if err := m.checkHistoryCapacity(len(inputs), 0); err != nil {
		return runFramesResult{}, err
	}
	startCycles := m.sys.CPU.Cycles()
	startFrame := m.sys.Bus.PPU.Frame()
	result := runFramesResult{}
	deadline := time.Time{}
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}

	inputIndex := m.applyInputs(inputs, 0, 0)

	err := m.sys.Guard(func() error {
		for result.CompletedFrames < frames {
			if err := ctx.Err(); err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					result.TimedOut = true
					return nil
				}
				return fmt.Errorf("run stopped: %w", err)
			}
			if !deadline.IsZero() && time.Now().After(deadline) {
				result.TimedOut = true
				return nil
			}
			step, err := m.sys.StepTraced()
			if err != nil {
				return fmt.Errorf("executing step: %w", err)
			}
			if !step.FrameCompleted {
				continue
			}
			result.CompletedFrames++
			inputIndex = m.applyInputs(inputs, inputIndex, result.CompletedFrames)
		}

		return nil
	})

	result.Frame = m.sys.Bus.PPU.Frame()
	result.PC = m.sys.PC
	result.CPUCycles = m.sys.CPU.Cycles() - startCycles
	result.Reason = runReason(result, frames, startFrame)
	if err != nil {
		return result, fmt.Errorf("running frames: %w", err)
	}
	return result, nil
}

// setButtons sets one controller state. A positive hold count runs that many
// frames with the buttons held and then releases them.
func (m *Machine) setButtons(ctx context.Context, controllerID int, buttons controller.Button, holdFrames uint64,
	timeout time.Duration) (runFramesResult, error) {

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return runFramesResult{}, fmt.Errorf("setting buttons: %w", err)
	}
	if controllerID != 1 && controllerID != 2 {
		return runFramesResult{}, fmt.Errorf(controllerNumberFormat, controllerID)
	}
	eventCount := 1
	if holdFrames > 0 {
		eventCount++
	}

	if err := m.checkHistoryCapacity(eventCount, 0); err != nil {
		return runFramesResult{}, err
	}
	device := controllerDevice(m.sys.Bus.Controller1)
	if controllerID == 2 {
		device = m.sys.Bus.Controller2
	}

	device.SetButtonState(allControllerButtons, false)
	device.SetButtonState(buttons, true)
	m.recordInput(controllerID, buttons)
	if holdFrames == 0 {
		return runFramesResult{}, nil
	}

	result, err := m.runFramesLocked(ctx, holdFrames, nil, timeout)
	device.SetButtonState(allControllerButtons, false)
	m.recordInput(controllerID, 0)

	return result, err
}

// screenshot renders the current frame as a PNG with the given scale.
func (m *Machine) screenshot(scale int) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	source := m.sys.Image()
	if source == nil {
		return nil, errors.New("the system has no image")
	}
	cloned := image.NewRGBA(source.Bounds())
	draw.Draw(cloned, cloned.Bounds(), source, source.Bounds().Min, draw.Src)

	rendered := scaledImage(cloned, scale)
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, rendered); err != nil {
		return nil, fmt.Errorf("encoding screenshot: %w", err)
	}
	return buffer.Bytes(), nil
}

// controllerInput changes one controller state before a relative frame starts.
type controllerInput struct {
	Buttons    controller.Button
	Controller int
	Frame      uint64
}

// runFramesResult contains the state at the stop boundary of a bounded run.
type runFramesResult struct {
	CompletedFrames uint64
	CPUCycles       uint64
	Frame           uint64
	PC              uint16
	Reason          nes.RunUntilReason
	TimedOut        bool
}

// stepResult contains the CPU state before and after a bounded step.
type stepResult struct {
	Before       cpu6502.State
	BeforeBank   int
	BeforeFrame  uint64
	BeforeOffset int

	After       cpu6502.State
	AfterBank   int
	AfterFrame  uint64
	AfterOffset int

	Frames       uint64
	Instructions uint64
}

// romSummary contains the ROM identity reported at machine creation.
type romSummary struct {
	MapperID   uint16 `json:"mapper_id"`
	MapperName string `json:"mapper_name"`

	Battery   bool   `json:"battery"`
	CHRSize   int    `json:"chr_size"`
	Mirroring string `json:"mirroring"`
	PRGSize   int    `json:"prg_size"`

	CHRCRC32 uint32 `json:"chr_crc32"`
	PRGCRC32 uint32 `json:"prg_crc32"`
	SHA256   string `json:"sha256"`

	batteryData []byte
}

// controllerDevice is the minimal controller interface the machine needs.
type controllerDevice interface {
	SetButtonState(key controller.Button, pressed bool)
}

// newMachine creates a machine with a live system.
func newMachine(id, romPath string, sys *nes.System, entrypoint int, battery bool, palette []byte) *Machine {
	machine := &Machine{
		id:  id,
		rom: romPath,

		battery:    battery,
		entrypoint: entrypoint,
		palette:    palette,

		requests: make(chan struct{}, 1),
		sys:      sys,

		created: time.Now(),
	}
	machine.touch()
	return machine
}

// loadSystem loads a ROM and creates a headless system and its ROM summary.
// An empty palette selects the default display palette.
func loadSystem(ctx context.Context, logger *log.Logger, romPath, expectedHash string, entrypoint int, battery bool,
	palette []byte, ring *nes.TraceRing, initialBattery *[]byte) (*nes.System, romSummary, error) {

	if err := ctx.Err(); err != nil {
		return nil, romSummary{}, fmt.Errorf("loading ROM: %w", err)
	}
	file, err := openRegularFile(romPath)
	if err != nil {
		return nil, romSummary{}, fmt.Errorf("opening ROM %q: %w", romPath, err)
	}
	defer func() { _ = file.Close() }()
	stop := context.AfterFunc(ctx, func() {
		_ = file.Close()
	})
	defer stop()

	hash := sha256.New()
	cart, err := cartridge.LoadFile(io.TeeReader(file, hash))
	if err != nil {
		return nil, romSummary{}, fmt.Errorf("loading ROM %q: %w", romPath, err)
	}
	romHash := hex.EncodeToString(hash.Sum(nil))
	if expectedHash != "" && romHash != expectedHash {
		return nil, romSummary{}, errors.New("the ROM has changed since the machine was created")
	}

	seed, err := loadBatteryData(ctx, romPath, battery, initialBattery)
	if err != nil {
		return nil, romSummary{}, err
	}

	options, err := bootOptions(logger, cart, seed, palette, ring)
	if err != nil {
		return nil, romSummary{}, err
	}

	sys, err := nes.NewSystem(options)
	if err != nil {
		return nil, romSummary{}, fmt.Errorf("creating system for %q: %w", romPath, err)
	}
	if entrypoint >= 0 {
		sys.PC = uint16(entrypoint)
	}

	summary := buildROMSummary(cart, sys, battery)
	summary.batteryData = seed
	summary.SHA256 = romHash

	return sys, summary, nil
}

// loadMachine loads a ROM and creates a headless machine and its summary. The
// trace ring starts off, because tracing slows emulation.
func loadMachine(ctx context.Context, logger *log.Logger, romPath string, entrypoint int, battery bool, palette []byte,
	initialBattery *[]byte) (*Machine, romSummary, error) {

	ring := nes.NewTraceRing(0)

	sys, summary, err := loadSystem(ctx, logger, romPath, "", entrypoint, battery, palette, ring, initialBattery)
	if err != nil {
		return nil, romSummary{}, err
	}
	id, err := newHandleID()
	if err != nil {
		return nil, romSummary{}, err
	}

	machine := newMachine(id, romPath, sys, entrypoint, battery, palette)
	machine.romHash = summary.SHA256
	machine.batteryData = summary.batteryData
	machine.logger = logger
	machine.traceRing = ring
	machine.breakpoints = nes.NewBreakpointSet()
	machine.attach(nil)

	return machine, summary, nil
}

// runReason names the stop reason of a bounded run.
func runReason(result runFramesResult, frames, startFrame uint64) nes.RunUntilReason {
	switch {
	case result.TimedOut:
		return nes.StopTimeout
	case result.CompletedFrames >= frames:
		return nes.StopFrames
	case result.Frame == startFrame:
		return stopNoProgress
	default:
		return stopStopped
	}
}

// scaledImage scales an image with nearest-neighbor sampling.
func scaledImage(source *image.RGBA, scale int) *image.RGBA {
	if scale <= 1 {
		return source
	}
	bounds := source.Bounds()
	width := bounds.Dx() * scale
	height := bounds.Dy() * scale
	target := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := range height {
		for x := range width {
			target.Set(x, y, source.At(bounds.Min.X+x/scale, bounds.Min.Y+y/scale))
		}
	}

	return target
}

// buildROMSummary returns the ROM identity of a loaded cartridge.
func buildROMSummary(cart *cartridge.Cartridge, sys *nes.System, battery bool) romSummary {
	return romSummary{
		MapperID:   cart.Mapper,
		MapperName: sys.Bus.Mapper.State().Name,

		Battery:   battery,
		CHRSize:   len(cart.CHR),
		Mirroring: mirrorName(cart.Mirror),
		PRGSize:   len(cart.PRG),

		CHRCRC32: crc32.ChecksumIEEE(cart.CHR),
		PRGCRC32: crc32.ChecksumIEEE(cart.PRG),
	}
}

// mirrorName returns the display name of a mirror mode.
func mirrorName(mode cartridge.MirrorMode) string {
	switch mode {
	case cartridge.MirrorHorizontal:
		return "horizontal"
	case cartridge.MirrorVertical:
		return "vertical"
	case cartridge.MirrorSingle0:
		return "single0"
	case cartridge.MirrorSingle1:
		return "single1"
	case cartridge.Mirror4:
		return "four-screen"
	case cartridge.MirrorDiagonal:
		return "diagonal"
	case cartridge.MirrorLShaped:
		return "l-shaped"
	case cartridge.Mirror3Vertical:
		return "3-vertical"
	case cartridge.Mirror3Horizontal:
		return "3-horizontal"
	case cartridge.Mirror3Diagonal:
		return "3-diagonal"
	default:
		return "unknown"
	}
}

// bootOptions configures a headless system with its initial save and palette.
func bootOptions(logger *log.Logger, cart *cartridge.Cartridge, seed, palette []byte,
	ring *nes.TraceRing) (*nes.Options, error) {

	options := []nes.Option{
		nes.WithCartridge(cart),
		nes.WithDisabledGUI(),
		nes.WithDisabledAudio(),
		nes.WithSummaryTarget(nil),
		nes.WithBatteryData(seed),
		nes.WithLogger(logger),
	}
	if len(palette) != 0 {
		displayPalette, err := ppu.ParseDisplayPalette(palette)
		if err != nil {
			return nil, fmt.Errorf("display palette: %w", err)
		}
		options = append(options, nes.WithDisplayPalette(displayPalette))
	}

	if ring != nil {
		options = append(options, nes.WithTraceRing(ring))
	}

	return nes.NewOptions(options...), nil
}

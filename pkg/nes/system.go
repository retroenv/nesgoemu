// Package nes contains the main emulator system.
package nes

import (
	"context"
	"fmt"
	"image"
	"time"

	"github.com/retroenv/nesgoemu/pkg/apu"
	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/controller"
	"github.com/retroenv/nesgoemu/pkg/mapper"
	"github.com/retroenv/nesgoemu/pkg/memory"
	"github.com/retroenv/nesgoemu/pkg/ppu"
	"github.com/retroenv/nesgoemu/pkg/ppu/nametable"
	"github.com/retroenv/nesgoemu/pkg/ppu/screen"
	"github.com/retroenv/retrogolib/arch/cpu/cpu6502"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/gui"
)

// System implements a NES system.
type System struct {
	opts    *Options
	storage batteryStorage

	apu    *apu.APU
	dma    dmaController
	memory *memory.Memory

	*cpu6502.CPU
	Bus *bus.Bus

	dimensions gui.Dimensions
}

// StepResult reports the time advanced by one system step.
type StepResult struct {
	CPUCycles           uint64
	Frame               uint64
	FrameCompleted      bool
	InstructionExecuted bool
}

// NewSystem creates a new NES system.
func NewSystem(opts *Options) (*System, error) {
	if opts == nil {
		opts = &Options{}
	}
	cart := opts.cartridge
	if cart == nil {
		cart = cartridge.New()
	}

	systemBus := &bus.Bus{
		Cartridge: cart,

		Controller1: controller.New(),
		Controller2: controller.New(),
		NameTable:   nametable.New(cart.Mirror),
	}

	systemMemory, mem, err := initializeMemory(systemBus)
	if err != nil {
		return nil, err
	}

	systemBus.Mapper, err = mapper.New(systemBus)
	if err != nil {
		return nil, fmt.Errorf("creating mapper: %w", err)
	}

	sys := &System{
		opts:    opts,
		storage: diskBatteryStorage{files: osBatteryFiles{}},
		memory:  systemMemory,
		Bus:     systemBus,
		dimensions: gui.Dimensions{
			ScaleFactor: 2.0,
			Height:      screen.Height,
			Width:       screen.Width,
		},
	}

	if err := sys.loadBattery(); err != nil {
		return nil, err
	}

	cpuOpts := []cpu6502.Option{
		cpu6502.WithVariant(cpu6502.VariantNES6502),
		cpu6502.WithCycleHook(sys.clockCPUCycle),
	}
	if opts.tracing {
		cpuOpts = append(cpuOpts, cpu6502.WithTracing(), cpu6502.WithPreExecutionHook(tracePreExecutionHook))
	}
	sys.CPU = cpu6502.New(mem, cpuOpts...)
	systemBus.CPU = sys.CPU
	systemBus.DMA = &sys.dma

	sys.apu = apu.New(systemBus)
	systemBus.APU = sys.apu
	systemBus.PPU = ppu.New(systemBus)
	sys.clockComponents(sys.CPU.Cycles())
	return sys, nil
}

// Image returns the emulator screen to show.
func (sys *System) Image() *image.RGBA {
	return sys.Bus.PPU.Image()
}

// Dimensions returns the dimensions for the emulator window.
func (sys *System) Dimensions() gui.Dimensions {
	return sys.dimensions
}

// WindowTitle returns the window title to show.
func (sys *System) WindowTitle() string {
	return "nesgoemu"
}

// InspectRAM reads internal CPU RAM without emulated bus side effects.
func (sys *System) InspectRAM(address uint16) (byte, bool) {
	return sys.memory.InspectRAM(address)
}

// InspectOAM returns a copy of primary OAM without changing PPU state.
func (sys *System) InspectOAM() [256]byte {
	return sys.Bus.PPU.OAM()
}

// StepSystem services one interrupt or CPU instruction and clocks the other components.
func (sys *System) StepSystem() (StepResult, error) {
	cyclesBefore := sys.CPU.Cycles()
	frameBefore := sys.Bus.PPU.Frame()

	instructionExecuted := !sys.CPU.CheckInterrupts()
	if instructionExecuted {
		if err := sys.CPU.Step(); err != nil {
			return StepResult{}, fmt.Errorf("executing CPU step at 0x%04x: %w", sys.PC, err)
		}
	}

	cpuCycles := sys.CPU.Cycles() - cyclesBefore
	frame := sys.Bus.PPU.Frame()

	return StepResult{
		CPUCycles:           cpuCycles,
		Frame:               frame,
		FrameCompleted:      frame != frameBefore,
		InstructionExecuted: instructionExecuted,
	}, nil
}

// NTSC NES timing: CPU runs at ~1.789773 MHz, PPU frame is 60.0988 Hz.
const (
	ntscCPUCyclesPerFrame = 29781
	ntscFrameDuration     = time.Second * 1000 / 60099 // ~16.64ms
)

// runEmulatorSteps runs until cancellation or the given stop address.
func (sys *System) runEmulatorSteps(ctx context.Context, stopAt int) error {
	var state cpuState
	frameCycles := uint64(0)
	nextFrame := time.Now().Add(ntscFrameDuration)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		if stopAt >= 0 && sys.PC == uint16(stopAt) {
			return nil
		}

		cycles := sys.CPU.Cycles()
		if sys.opts.tracing {
			state.A = sys.CPU.A
			state.X = sys.CPU.X
			state.Y = sys.CPU.Y
			state.SP = sys.CPU.SP
			state.Flags = sys.CPU.GetFlags()
			state.Cycles = cycles
		}

		step, err := sys.StepSystem()
		if err != nil {
			return err
		}

		if sys.opts.tracing && step.InstructionExecuted {
			sys.printTraceStep(state)
		}

		cpuCycles := sys.CPU.Cycles() - cycles

		frameCycles += cpuCycles
		if frameCycles >= ntscCPUCyclesPerFrame {
			frameCycles -= ntscCPUCyclesPerFrame
			if sleep := time.Until(nextFrame); sleep > 0 {
				time.Sleep(sleep)
			}
			nextFrame = nextFrame.Add(ntscFrameDuration)
			// If we've fallen far behind (e.g. paused/breakpoint), resync.
			if time.Until(nextFrame) < -ntscFrameDuration {
				nextFrame = time.Now().Add(ntscFrameDuration)
			}
		}
	}
}

func (sys *System) clockComponents(cycles uint64) {
	clocker, _ := sys.Bus.Mapper.(bus.CPUClocker)

	for range cycles {
		if clocker != nil {
			clocker.ClockCPU(1)
		}

		sys.Bus.APU.Step(1)
		sys.Bus.PPU.Step(3)
	}
}

// runRenderer starts the chosen GUI renderer. It stops when the renderer stops,
// the context is cancelled, or the audio playback fails.
func (sys *System) runRenderer(ctx context.Context, opts *Options, guiStarter gui.Initializer,
	audioErrors <-chan error) error {

	render, cleanup, err := guiStarter(sys)
	if err != nil {
		return err
	}
	defer cleanup()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan struct{})
	var cpuError error
	go func() {
		defer close(done)
		cpuError = sys.runEmulatorSteps(ctx, opts.stopAt)
	}()
	defer func() {
		cancel()
		<-done
	}()

	for {
		select {
		case <-done:
			return cpuError
		case <-ctx.Done():
			return nil
		case err := <-audioErrors:
			return err
		default:
		}

		running, err := render()
		if err != nil {
			return err
		}
		if !running {
			return nil
		}
		time.Sleep(time.Second / ppu.FPS)
	}
}

// clockCPUCycle selects a bus owner, clocks the devices, and completes a DMA
// transfer. A false return value lets the CPU complete its own bus access.
func (sys *System) clockCPUCycle(cycle cpu6502.BusCycle) bool {
	request, pending := sys.apu.DMCRequest()
	get := sys.CPU.Cycles()&1 == 0
	action := sys.dma.next(cycle, get, request, pending)
	sys.clockComponents(1)
	sys.memory.BeginCycle()

	switch action {
	case cpuAccess:
		return false
	case repeatRead:
		sys.Bus.Memory.Read(cycle.Address)
	case dmcRead:
		value := sys.Bus.Memory.Read(sys.dma.dmcAddress)
		sys.apu.CompleteDMCTransfer(value, sys.dma.dmcCycles)
	case oamRead:
		address := uint16(sys.dma.oamPage)<<8 | sys.dma.oamOffset
		sys.dma.oamValue = sys.Bus.Memory.Read(address)
		sys.dma.oamFull = true
	case oamWrite:
		sys.Bus.PPU.Write(0x2004, sys.dma.oamValue)
		sys.dma.oamFull = false
		sys.dma.oamOffset++
		sys.dma.oamActive = sys.dma.oamOffset < 256
	}
	return true
}

func initializeMemory(systemBus *bus.Bus) (*memory.Memory, *cpu6502.Memory, error) {
	systemMemory := memory.New(systemBus)
	cpuMemory, err := cpu6502.NewMemory(systemMemory)
	if err != nil {
		return nil, nil, fmt.Errorf("creating memory: %w", err)
	}

	systemBus.Memory = cpuMemory
	systemBus.OpenBus = systemMemory

	return systemMemory, cpuMemory, nil
}

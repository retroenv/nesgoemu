// Package nes contains the main emulator system.
package nes

import (
	"context"
	"fmt"
	"image"
	"sync/atomic"
	"time"

	"github.com/retroenv/nesgoemu/pkg/apu"
	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/controller"
	"github.com/retroenv/nesgoemu/pkg/mapper"
	"github.com/retroenv/nesgoemu/pkg/memory"
	"github.com/retroenv/nesgoemu/pkg/ppu"
	"github.com/retroenv/nesgoemu/pkg/ppu/nametable"
	"github.com/retroenv/nesgoemu/pkg/ppu/screen"
	"github.com/retroenv/retrogolib/arch/cpu/m6502"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/gui"
)

// System implements a NES system.
type System struct {
	opts *Options

	*m6502.CPU
	Bus *bus.Bus

	dimensions gui.Dimensions
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
		Cartridge:   cart,
		Controller1: controller.New(),
		Controller2: controller.New(),
		NameTable:   nametable.New(cart.Mirror),
	}

	mem, err := m6502.NewMemory(memory.New(systemBus))
	if err != nil {
		return nil, fmt.Errorf("creating memory: %w", err)
	}
	systemBus.Memory = mem

	systemBus.Mapper, err = mapper.New(systemBus)
	if err != nil {
		return nil, fmt.Errorf("creating mapper: %w", err)
	}

	sys := &System{
		opts: opts,
		Bus:  systemBus,
		dimensions: gui.Dimensions{
			ScaleFactor: 2.0,
			Height:      screen.Height,
			Width:       screen.Width,
		},
	}

	var cpuOpts []m6502.Option
	if opts.tracing {
		cpuOpts = append(cpuOpts, m6502.WithTracing(), m6502.WithPreExecutionHook(tracePreExecutionHook))
	}
	sys.CPU = m6502.New(mem, cpuOpts...)
	systemBus.CPU = sys.CPU

	systemBus.APU = apu.New(systemBus)
	systemBus.PPU = ppu.New(systemBus)
	return sys, nil
}

// runEmulatorSteps runs the emulator until it is quit or reaches the given stop address.
func (sys *System) runEmulatorSteps(stopAt int) error {
	var state cpuState

	for {
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

		if !sys.CPU.CheckInterrupts() {
			if err := sys.CPU.Step(); err != nil {
				return fmt.Errorf("executing CPU step: %w", err)
			}

			if sys.opts.tracing {
				sys.printTraceStep(state)
			}
		}

		cpuCycles := sys.CPU.Cycles() - cycles
		ppuCycles := cpuCycles * 3
		sys.Bus.PPU.Step(int(ppuCycles))
	}
}

// runRenderer starts the chosen GUI renderer.
func (sys *System) runRenderer(ctx context.Context, opts *Options, guiStarter gui.Initializer) error {
	render, cleanup, err := guiStarter(sys)
	if err != nil {
		return err
	}
	defer cleanup()

	running := uint64(1)
	go func() {
		if err := sys.runEmulatorSteps(opts.stopAt); err != nil {
			panic(err)
		}
		if opts.stopAt >= 0 {
			atomic.StoreUint64(&running, 0)
			return
		}

		// nolint: revive
		for { // forever loop in case reset handler returns
		}
	}()

	for atomic.LoadUint64(&running) == 1 {
		continueRunning, err := render()
		if err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			continueRunning = false
		default:
		}

		if !continueRunning {
			atomic.StoreUint64(&running, 0)
		}

		// TODO replace with better solution
		time.Sleep(time.Second / ppu.FPS)
	}
	return nil
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

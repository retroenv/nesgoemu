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

	*cpu6502.CPU
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
		Cartridge: cart,

		Controller1: controller.New(),
		Controller2: controller.New(),
		NameTable:   nametable.New(cart.Mirror),
	}

	mem, err := cpu6502.NewMemory(memory.New(systemBus))
	if err != nil {
		return nil, fmt.Errorf("creating memory: %w", err)
	}
	systemBus.Memory = mem

	systemBus.Mapper, err = mapper.New(systemBus)
	if err != nil {
		return nil, fmt.Errorf("creating mapper: %w", err)
	}

	sys := &System{
		opts:    opts,
		storage: diskBatteryStorage{files: osBatteryFiles{}},
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

	cpuOpts := []cpu6502.Option{cpu6502.WithVariant(cpu6502.VariantNES6502)}
	if opts.tracing {
		cpuOpts = append(cpuOpts, cpu6502.WithTracing(), cpu6502.WithPreExecutionHook(tracePreExecutionHook))
	}
	sys.CPU = cpu6502.New(mem, cpuOpts...)
	systemBus.CPU = sys.CPU

	systemBus.APU = apu.New(systemBus)
	systemBus.PPU = ppu.New(systemBus)
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

		if !sys.CPU.CheckInterrupts() {
			if err := sys.CPU.Step(); err != nil {
				return fmt.Errorf("executing CPU step: %w", err)
			}

			if sys.opts.tracing {
				sys.printTraceStep(state)
			}
		}

		cpuCycles := sys.CPU.Cycles() - cycles
		sys.clockComponents(cpuCycles)

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

		sys.Bus.PPU.Step(3)
	}
}

// runRenderer starts the chosen GUI renderer.
func (sys *System) runRenderer(ctx context.Context, opts *Options, guiStarter gui.Initializer) error {
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

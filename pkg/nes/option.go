package nes

import (
	"io"
	"os"

	"github.com/retroenv/retrogolib/arch/cpu/cpu6502"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
)

// Options contains options for the nesgoemu system.
type Options struct {
	entrypoint int
	savePath   string
	stopAt     int

	debug        bool
	debugAddress string

	noAudio bool
	noGui   bool

	cartridge *cartridge.Cartridge

	cpuPreExecutionHook func(*cpu6502.CPU, *cpu6502.Instruction, ...any)
	cpuOptions          []cpu6502.Option

	tracing       bool
	tracingTarget io.Writer

	summaryTarget io.Writer
}

// Option defines a Start parameter.
type Option func(*Options)

// NewOptions creates a new options instance from the passed options.
func NewOptions(optionList ...Option) *Options {
	opts := &Options{
		entrypoint: -1,
		stopAt:     -1,
	}
	for _, option := range optionList {
		option(opts)
	}

	if opts.tracing && opts.tracingTarget == nil {
		opts.tracingTarget = os.Stdout
	}

	return opts
}

func (opts *Options) runCPUPreExecutionHooks(cpu *cpu6502.CPU, ins *cpu6502.Instruction, params ...any) {
	if opts.tracing {
		tracePreExecutionHook(cpu, ins, params...)
	}
	if opts.cpuPreExecutionHook != nil {
		opts.cpuPreExecutionHook(cpu, ins, params...)
	}
}

// WithCPUOptions passes CPU options to the new system.
func WithCPUOptions(values ...cpu6502.Option) Option {
	return func(options *Options) {
		options.cpuOptions = append(options.cpuOptions, values...)
	}
}

// WithCPUPreExecutionHook sets a hook for each CPU instruction.
// It replaces a previous hook. The tracing hook runs first when tracing is on.
func WithCPUPreExecutionHook(hook func(*cpu6502.CPU, *cpu6502.Instruction, ...any)) Option {
	return func(options *Options) {
		options.cpuPreExecutionHook = hook
	}
}

// WithCartridge sets a cartridge to load.
func WithCartridge(cart *cartridge.Cartridge) func(*Options) {
	return func(options *Options) {
		options.cartridge = cart
	}
}

// WithDebug enables the debugging mode and webserver.
func WithDebug(debugAddress string) func(*Options) {
	return func(options *Options) {
		options.debug = true
		options.debugAddress = debugAddress
	}
}

// WithTracing enables tracing for the program.
func WithTracing() func(*Options) {
	return func(options *Options) {
		options.tracing = true
	}
}

// WithTracingTarget set the tracing target io writer.
func WithTracingTarget(target io.Writer) func(*Options) {
	return func(options *Options) {
		options.tracing = true
		options.tracingTarget = target
	}
}

// WithEntrypoint enables tracing for the program.
func WithEntrypoint(address int) func(*Options) {
	return func(options *Options) {
		options.entrypoint = address
	}
}

// WithStopAt stops execution of the program at a specific address.
func WithStopAt(address int) func(*Options) {
	return func(options *Options) {
		options.stopAt = address
	}
}

// WithDisabledGUI disabled the GUI.
func WithDisabledGUI() func(*Options) {
	return func(options *Options) {
		options.noGui = true
	}
}

// WithDisabledAudio disables the audio output.
func WithDisabledAudio() func(*Options) {
	return func(options *Options) {
		options.noAudio = true
	}
}

// WithSavePath selects the cartridge save file. An empty path disables file saves.
func WithSavePath(path string) Option {
	return func(opts *Options) { opts.savePath = path }
}

// WithSummaryTarget sets the target for the run summary. A nil target disables the summary.
func WithSummaryTarget(target io.Writer) Option {
	return func(opts *Options) { opts.summaryTarget = target }
}

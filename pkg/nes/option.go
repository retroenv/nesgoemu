package nes

import (
	"io"
	"os"

	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
)

// Options contains options for the nesgoemu system.
type Options struct {
	entrypoint int
	stopAt     int

	debug        bool
	debugAddress string

	noGui     bool
	cartridge *cartridge.Cartridge

	tracing       bool
	tracingTarget io.Writer
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

package nes

import (
	"github.com/retroenv/nesgoemu/pkg/nes/debugger"
	"github.com/retroenv/retrogolib/app"
	"github.com/retroenv/retrogolib/gui"
)

// Start is the main entrypoint for a NES program that starts the execution.
func Start(options ...Option) {
	opts := NewOptions(options...)
	sys := NewSystem(opts)
	if opts.entrypoint >= 0 {
		sys.PC = uint16(opts.entrypoint)
	}

	sys.LinkAliases()

	sys.CPU.SetTracing(opts.tracing, opts.tracingTarget)

	sys.ResetHandler = func() {
		sys.runEmulatorSteps(opts.stopAt)
	}

	ctx := app.Context()
	var debugServer *debugger.Debugger
	if opts.debug {
		debugServer = debugger.New(opts.debugAddress, sys.Bus)
		go debugServer.Start(ctx)
	}

	guiStarter := setupNoGui
	if gui.Setup != nil && !opts.noGui {
		guiStarter = gui.Setup
	}
	if err := sys.runRenderer(ctx, opts, guiStarter); err != nil {
		panic(err)
	}
}

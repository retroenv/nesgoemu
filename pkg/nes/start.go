package nes

import (
	"context"
	"errors"

	"github.com/retroenv/nesgoemu/pkg/nes/debugger"
	"github.com/retroenv/retrogolib/audio"
	"github.com/retroenv/retrogolib/gui"
)

// Start runs the NES program until it stops or the context is canceled.
func Start(ctx context.Context, options ...Option) error {
	opts := NewOptions(options...)
	sys, err := NewSystem(opts)
	if err != nil {
		return err
	}
	if opts.entrypoint >= 0 {
		sys.PC = uint16(opts.entrypoint)
	}

	guiStarter := setupNoGui
	if gui.Available() && !opts.noGui {
		guiStarter = sys.setupGUI
	}

	var playback *audio.Playback
	if audioEnabled(opts) {
		playback, err = startAudio(sys)
		if err != nil {
			return err
		}

		defer playback.Stop()
	}

	var debugErr error
	runCtx := ctx
	var stopDebug func() error
	if opts.debug {
		runCtx, stopDebug = startDebugger(ctx, debugger.New(opts.debugAddress, sys.Bus))
	}

	runErr := sys.runRenderer(runCtx, opts, guiStarter, playbackErrors(playback))
	if stopDebug != nil {
		debugErr = stopDebug()
	}

	sys.memory.FlushPrintf()
	summaryErr := sys.writeSummary(opts.summaryTarget, opts.summaryFeatures)

	return errors.Join(runErr, debugErr, sys.SaveBattery(), summaryErr)
}

func startDebugger(ctx context.Context, server *debugger.Debugger) (context.Context, func() error) {
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		done <- server.Serve(runCtx)
		cancel()
	}()

	return runCtx, func() error {
		cancel()
		return <-done
	}
}

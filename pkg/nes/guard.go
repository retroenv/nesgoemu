package nes

import (
	"fmt"

	"github.com/retroenv/retrogolib/log"
)

// Guard runs fn and converts a panic into an error that names the program
// counter. One bad ROM then becomes a tool error instead of a process exit.
// The logger records the panic when one is set.
func (sys *System) Guard(fn func() error) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("panic at PC 0x%04x: %v", sys.PC, recovered)
			sys.opts.logger.Error("Emulator panic", log.Err(err))
		}
	}()

	return fn()
}

package nes

import (
	"testing"

	"github.com/retroenv/nesgoemu/pkg/controller"
	"github.com/retroenv/retrogolib/assert"
	"github.com/retroenv/retrogolib/input"
)

func TestBackendControllerInput(t *testing.T) {
	device, err := NewNative(NativeOptions{
		CHR: make([]byte, 8192),
	})
	assert.NoError(t, err)
	system := newAudioTestSystem(t, loopProgram)
	backends := []struct {
		name    string
		keyDown func(input.Key)
		keyUp   func(input.Key)
		read    func(uint16) byte
		write   func(uint16, byte)
	}{
		{
			name:    "native",
			keyDown: device.KeyDown,
			keyUp:   device.KeyUp,
			read:    device.Read,
			write:   device.Write,
		},
		{
			name:    "ROM",
			keyDown: system.KeyDown,
			keyUp:   system.KeyUp,
			read:    system.Bus.Memory.Read,
			write:   system.Bus.Memory.Write,
		},
	}
	keys := []struct {
		key    input.Key
		button controller.Button
	}{
		{key: input.Up, button: controller.Up},
		{key: input.Down, button: controller.Down},
		{key: input.Left, button: controller.Left},
		{key: input.Right, button: controller.Right},
		{key: input.Z, button: controller.A},
		{key: input.X, button: controller.B},
		{key: input.Enter, button: controller.Start},
		{key: input.Backspace, button: controller.Select},
	}
	for _, backend := range backends {
		t.Run(backend.name, func(t *testing.T) {
			for _, key := range keys {
				backend.keyDown(key.key)
				assert.Equal(t, byte(key.button), readControllerButtons(backend.read, backend.write))
				backend.keyUp(key.key)
				assert.Equal(t, byte(0), readControllerButtons(backend.read, backend.write))
			}
			backend.keyDown(input.Z)
			backend.keyDown(input.Right)
			backend.keyDown(input.Key(-1))
			backend.keyUp(input.Key(-1))
			assert.Equal(t, byte(controller.A|controller.Right), readControllerButtons(backend.read, backend.write))
		})
	}
}

func readControllerButtons(read func(uint16) byte, write func(uint16, byte)) byte {
	write(0x4016, 1)
	write(0x4016, 0)
	var buttons byte
	for bit := range 8 {
		buttons |= (read(0x4016) & 1) << bit
	}
	return buttons
}

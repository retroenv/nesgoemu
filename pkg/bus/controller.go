package bus

import "github.com/retroenv/nesgoemu/pkg/controller"

// Controller represents a hardware controller.
type Controller interface {
	Read() uint8
	SetButtonState(key controller.Button, pressed bool)
	SetStrobeMode(mode uint8)
}

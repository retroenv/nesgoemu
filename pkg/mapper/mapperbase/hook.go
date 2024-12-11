package mapperbase

// Hook defines a hook type that can be configured after creation.
type Hook interface {
	SetProxyOnly(proxy bool)
}

type ReadHookFunc func(address uint16) (uint8, error)

type WriteHookFunc func(address uint16, value uint8) error

type hook struct {
	startAddress uint16
	endAddress   uint16

	onlyProxy bool // whether to continue mapper memory function execution after hook call
}

type readHook struct {
	hook

	hookFunc ReadHookFunc
}

type writeHook struct {
	hook

	hookFunc WriteHookFunc
}

func (h *hook) SetProxyOnly(proxy bool) {
	h.onlyProxy = proxy
}

// AddReadHook adds an address range read hook that gets called when a read from given range is made.
func (b *Base) AddReadHook(startAddress, endAddress uint16, hookFunc ReadHookFunc) Hook {
	hook := readHook{
		hook: hook{
			startAddress: startAddress,
			endAddress:   endAddress,
		},
		hookFunc: hookFunc,
	}
	b.readHooks = append(b.readHooks, hook)
	return &hook.hook
}

// AddWriteHook adds an address range write hook that gets called when a write into the given range is made.
func (b *Base) AddWriteHook(startAddress, endAddress uint16, hookFunc WriteHookFunc) Hook {
	hook := writeHook{
		hook: hook{
			startAddress: startAddress,
			endAddress:   endAddress,
		},
		hookFunc: hookFunc,
	}
	b.writeHooks = append(b.writeHooks, hook)
	return &hook.hook
}

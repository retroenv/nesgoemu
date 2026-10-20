package nes

import (
	"slices"

	"github.com/retroenv/retrogolib/arch/cpu/cpu6502"
)

// AddPreExecutionHook registers a hook that runs before each instruction
// executes. The hook sees the CPU state after decoding and before the
// operation. It returns a function that removes the hook.
func (sys *System) AddPreExecutionHook(hook func(*cpu6502.CPU, *cpu6502.Instruction, ...any)) func() {
	id := sys.addPreExecutionHook(hook)
	return func() { sys.removePreExecutionHook(id) }
}

// AddStackHook registers a hook that receives each CPU stack push and pull.
// It returns a function that removes the hook.
func (sys *System) AddStackHook(hook func(cpu6502.StackEvent)) func() {
	id := sys.addStackHook(hook)
	return func() { sys.removeStackHook(id) }
}

// AddBusCycleObserver registers an observer for each CPU bus cycle. The
// observer runs before the bus access. A read that DMA takes over is not
// reported, because the CPU repeats it. It returns a function that removes the
// observer.
func (sys *System) AddBusCycleObserver(observer func(cpu6502.BusCycle)) func() {
	id := sys.addBusCycleObserver(observer)
	return func() { sys.removeBusCycleObserver(id) }
}

func (sys *System) addPreExecutionHook(hook preExecutionHook) uint64 {
	sys.hookMu.Lock()
	defer sys.hookMu.Unlock()

	sys.hookSeq++
	entry := preExecutionEntry{
		hook: hook,
		id:   sys.hookSeq,
	}
	next := sys.snapshotHooks()
	next.preExecution = append(next.preExecution, entry)
	sys.storeHooks(next)
	return entry.id
}

func (sys *System) addStackHook(hook stackObserver) uint64 {
	sys.hookMu.Lock()
	defer sys.hookMu.Unlock()

	sys.hookSeq++
	entry := stackEntry{
		hook: hook,
		id:   sys.hookSeq,
	}
	next := sys.snapshotHooks()
	next.stack = append(next.stack, entry)
	sys.storeHooks(next)
	return entry.id
}

func (sys *System) addBusCycleObserver(observer busCycleObserver) uint64 {
	sys.hookMu.Lock()
	defer sys.hookMu.Unlock()

	sys.hookSeq++
	entry := busCycleEntry{
		hook: observer,
		id:   sys.hookSeq,
	}
	next := sys.snapshotHooks()
	next.busCycle = append(next.busCycle, entry)
	sys.storeHooks(next)
	return entry.id
}

func (sys *System) removePreExecutionHook(id uint64) {
	sys.hookMu.Lock()
	defer sys.hookMu.Unlock()

	next := sys.snapshotHooks()
	next.preExecution = slices.DeleteFunc(next.preExecution, func(entry preExecutionEntry) bool {
		return entry.id == id
	})
	sys.storeHooks(next)
}

func (sys *System) removeStackHook(id uint64) {
	sys.hookMu.Lock()
	defer sys.hookMu.Unlock()

	next := sys.snapshotHooks()
	next.stack = slices.DeleteFunc(next.stack, func(entry stackEntry) bool {
		return entry.id == id
	})
	sys.storeHooks(next)
}

func (sys *System) removeBusCycleObserver(id uint64) {
	sys.hookMu.Lock()
	defer sys.hookMu.Unlock()

	next := sys.snapshotHooks()
	next.busCycle = slices.DeleteFunc(next.busCycle, func(entry busCycleEntry) bool {
		return entry.id == id
	})
	sys.storeHooks(next)
}

// snapshotHooks returns a private copy of the current hook set. The caller
// holds the hook mutex.
func (sys *System) snapshotHooks() *hookSet {
	current := sys.hooks.Load()
	if current == nil {
		return &hookSet{}
	}
	return &hookSet{
		preExecution: slices.Clone(current.preExecution),
		stack:        slices.Clone(current.stack),
		busCycle:     slices.Clone(current.busCycle),
	}
}

// storeHooks publishes the hook set. An empty set becomes nil to keep the hot
// path free of loops when no hook is installed.
func (sys *System) storeHooks(next *hookSet) {
	if len(next.preExecution) == 0 && len(next.stack) == 0 && len(next.busCycle) == 0 {
		sys.hooks.Store(nil)
		return
	}
	sys.hooks.Store(next)
}

// runPreExecutionHooks runs the tracing, construction hook, and runtime hooks.
func (sys *System) runPreExecutionHooks(cpu *cpu6502.CPU, ins *cpu6502.Instruction, params ...any) {
	if sys.tracingEnabled() {
		sys.captureTraceOperands(cpu, ins, params...)
	}
	sys.opts.runCPUPreExecutionHooks(cpu, ins, params...)

	set := sys.hooks.Load()
	if set == nil {
		return
	}
	for _, entry := range set.preExecution {
		entry.hook(cpu, ins, params...)
	}
}

// runStackHooks runs the runtime stack hooks.
func (sys *System) runStackHooks(event cpu6502.StackEvent) {
	set := sys.hooks.Load()
	if set == nil {
		return
	}
	for _, entry := range set.stack {
		entry.hook(event)
	}
}

// observeBusCycle runs the runtime bus cycle observers.
func (sys *System) observeBusCycle(cycle cpu6502.BusCycle) {
	set := sys.hooks.Load()
	if set == nil {
		return
	}
	for _, entry := range set.busCycle {
		entry.hook(cycle)
	}
}

// preExecutionHook observes one decoded instruction before it executes.
type preExecutionHook func(cpu *cpu6502.CPU, ins *cpu6502.Instruction, params ...any)

// stackObserver observes one CPU stack push or pull.
type stackObserver func(event cpu6502.StackEvent)

// busCycleObserver observes one CPU bus cycle.
type busCycleObserver func(cycle cpu6502.BusCycle)

type preExecutionEntry struct {
	hook preExecutionHook
	id   uint64
}

type stackEntry struct {
	hook stackObserver
	id   uint64
}

type busCycleEntry struct {
	hook busCycleObserver
	id   uint64
}

// hookSet is the immutable snapshot of the installed runtime hooks. A nil
// pointer means that no hook is installed.
type hookSet struct {
	preExecution []preExecutionEntry
	stack        []stackEntry
	busCycle     []busCycleEntry
}

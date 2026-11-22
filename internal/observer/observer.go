// Package observer provides independent callback registrations.
package observer

import (
	"slices"
	"sync"
	"sync/atomic"
)

type entry[T any] struct {
	call func(T)
	id   uint64
}
type callbacks[T any] struct{ entries []entry[T] }

// Group publishes callbacks without a lock on the event path.
// Remove prevents future events. A callback already in progress can finish.
type Group[T any] struct {
	mu   sync.Mutex
	next uint64

	current atomic.Pointer[callbacks[T]]
}

// Add registers an independent callback and returns its removal function.
func (g *Group[T]) Add(call func(T)) func() {
	g.mu.Lock()
	g.next++
	id := g.next

	next := g.copy()
	next.entries = append(next.entries, entry[T]{
		call: call,
		id:   id,
	})
	g.current.Store(next)
	g.mu.Unlock()

	return func() { g.remove(id) }
}

// Set replaces the legacy callback without changing independent registrations.
func (g *Group[T]) Set(call func(T)) {
	g.mu.Lock()
	defer g.mu.Unlock()
	next := g.copy()
	next.entries = slices.DeleteFunc(next.entries, func(e entry[T]) bool {
		return e.id == 0
	})
	if call != nil {
		next.entries = append(next.entries, entry[T]{
			call: call,
		})
	}

	g.publish(next)
}

// Send calls the current callbacks in registration order.
func (g *Group[T]) Send(event T) {
	if current := g.current.Load(); current != nil {
		for _, e := range current.entries {
			e.call(event)
		}
	}
}

// Active reports whether at least one callback is registered.
func (g *Group[T]) Active() bool { return g.current.Load() != nil }

func (g *Group[T]) remove(id uint64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	next := g.copy()
	next.entries = slices.DeleteFunc(next.entries, func(e entry[T]) bool {
		return e.id == id
	})
	g.publish(next)
}

func (g *Group[T]) publish(next *callbacks[T]) {
	if len(next.entries) == 0 {
		next = nil
	}

	g.current.Store(next)
}

func (g *Group[T]) copy() *callbacks[T] {
	next := &callbacks[T]{}
	if current := g.current.Load(); current != nil {
		next.entries = slices.Clone(current.entries)
	}

	return next
}

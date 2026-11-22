package observer

import (
	"sync"
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

func TestIndependentObserversSurviveLegacyReplacement(t *testing.T) {
	var group Group[int]
	var independent, legacy int
	remove := group.Add(func(value int) {
		independent += value
	})
	group.Set(func(value int) {
		legacy += value
	})
	group.Send(1)
	group.Set(nil)
	group.Send(2)
	remove()
	remove()
	group.Send(4)
	assert.Equal(t, 3, independent)
	assert.Equal(t, 1, legacy)
	assert.False(t, group.Active())
}

func TestConcurrentObserverChanges(t *testing.T) {
	const iterations = 1000

	var group Group[int]
	var workers sync.WaitGroup
	workers.Go(func() {
		for range iterations {
			group.Send(1)
		}
	})
	for range iterations {
		remove := group.Add(func(int) {})
		group.Set(func(int) {})
		remove()
		group.Set(nil)
	}
	workers.Wait()
	assert.False(t, group.Active())
}

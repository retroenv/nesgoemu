package nes

import (
	"testing"
	"time"

	"github.com/retroenv/retrogolib/assert"
)

func TestNextFrameDeadline(t *testing.T) {
	previous := time.Unix(100, 0)
	for _, delay := range []time.Duration{0, 20 * time.Millisecond, 150 * time.Millisecond} {
		assert.Equal(t, previous.Add(ntscFrameDuration), nextFrameDeadline(previous, previous.Add(delay)))
	}
	now := previous.Add(time.Second)
	assert.Equal(t, now.Add(ntscFrameDuration), nextFrameDeadline(previous, now))
}

package mixer

import (
	"math"
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

const tolerance = 0.00001

func TestMixSilence(t *testing.T) {
	assert.Equal(t, 0.0, Mix(0, 0, 0, 0, 0))
}

func TestPulseChannelsAreInterchangeable(t *testing.T) {
	assert.Equal(t, Mix(15, 0, 0, 0, 0), Mix(0, 15, 0, 0, 0))
}

func TestPulseLevelsFollowDACCurve(t *testing.T) {
	tests := []struct {
		name     string
		pulse1   byte
		pulse2   byte
		expected float64
	}{
		{"single channel", 15, 0, 0.149376},
		{"both channels", 15, 15, 0.258479},
		{"half volume", 8, 8, 0.157697},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertNear(t, tt.expected, Mix(tt.pulse1, tt.pulse2, 0, 0, 0))
		})
	}
}

func TestTriangleNoiseAndDMCShareOnePath(t *testing.T) {
	tests := []struct {
		name     string
		triangle byte
		noise    byte
		dmc      byte
		expected float64
	}{
		{"triangle only", 15, 0, 0, 0.246411},
		{"noise only", 0, 15, 0, 0.174431},
		{"dmc only", 0, 0, 127, 0.574264},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertNear(t, tt.expected, Mix(0, 0, tt.triangle, tt.noise, tt.dmc))
		})
	}
}

func TestMixStaysWithinSampleRange(t *testing.T) {
	for pulse1 := range byte(16) {
		for triangle := range byte(16) {
			for _, dmc := range []byte{0, 64, 127} {
				level := Mix(pulse1, pulse1, triangle, triangle, dmc)
				assert.GreaterOrEqual(t, level, 0.0)
				assert.LessOrEqual(t, level, 1.0)
			}
		}
	}
}

// assertNear checks a floating point value with a small tolerance.
func assertNear(t *testing.T, expected, actual float64) {
	t.Helper()

	if math.Abs(expected-actual) > tolerance {
		assert.Fail(t, "values differ", "expected", expected, "actual", actual)
	}
}

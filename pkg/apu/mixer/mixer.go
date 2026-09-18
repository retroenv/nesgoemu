// Package mixer converts the APU channel output levels to the mixed output
// level. The hardware digital to analog converter mixes the channels in a
// nonlinear way.
// https://www.nesdev.org/wiki/APU_Mixer
package mixer

// Mix returns the mixed level of all channels for one CPU cycle.
// Each channel contributes its current digital to analog converter value:
// the pulse and triangle channels use 0 to 15, the noise channel uses 0 to 15,
// and the DMC channel uses 0 to 127.
// https://www.nesdev.org/wiki/APU_Mixer
func Mix(pulse1, pulse2, triangle, noise, dmc byte) float64 {
	pulseSum := float64(pulse1) + float64(pulse2)
	pulseOut := 0.0
	if pulseSum > 0 {
		pulseOut = 95.88 / (8128/pulseSum + 100)
	}

	tdnSum := float64(triangle)/8227 + float64(noise)/12241 + float64(dmc)/22638
	tdnOut := 0.0
	if tdnSum > 0 {
		tdnOut = 159.79 / (1/tdnSum + 100)
	}

	return pulseOut + tdnOut
}

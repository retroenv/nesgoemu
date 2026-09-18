package apu

import "github.com/retroenv/retrogolib/arch/system/nes/register"

// Read returns the value of an APU register.
// $4015 returns the channel status and clears the frame interrupt flag. The
// other registers are write-only, and open bus behavior is not modeled.
// https://www.nesdev.org/wiki/APU_registers
func (a *APU) Read(address uint16) byte {
	if address != register.APU_SND_CHN {
		return 0xff
	}

	return a.readStatus()
}

// Write sets an APU register.
// https://www.nesdev.org/wiki/APU_registers
func (a *APU) Write(address uint16, value byte) {
	switch {
	case address <= register.APU_PL1_HI: // $4000 to $4003
		a.pulse1.Write(address, value)

	case address <= register.APU_PL2_HI: // $4004 to $4007
		a.pulse2.Write(address, value)

	case address <= register.APU_TRI_HI: // $4008 to $400B
		a.triangle.Write(address, value)

	case address <= register.APU_NOISE_HI: // $400C to $400F
		a.noise.Write(address, value)

	case address <= register.APU_DMC_LEN: // $4010 to $4013
		a.dmc.Write(address, value)
		a.updateIRQ()

	case address == register.APU_SND_CHN: // $4015
		a.writeStatus(value)

	case address == register.APU_FRAME: // $4017
		a.frame.Write(value)
		a.updateIRQ()
	}
}

// readStatus reads $4015 and clears the frame interrupt flag.
// https://www.nesdev.org/wiki/APU#Status_($4015)
func (a *APU) readStatus() byte {
	var value byte
	if a.pulse1.LengthActive() {
		value |= 0x01
	}
	if a.pulse2.LengthActive() {
		value |= 0x02
	}
	if a.triangle.LengthActive() {
		value |= 0x04
	}
	if a.noise.LengthActive() {
		value |= 0x08
	}
	if a.dmc.Active() {
		value |= 0x10
	}
	if a.dmc.IRQ() {
		value |= 0x40
	}
	if a.frame.IRQ() {
		value |= 0x80
	}

	a.frame.ClearIRQ()
	a.updateIRQ()

	return value
}

// writeStatus writes $4015. The write clears the DMC interrupt flag and enables
// the channels. An enabled DMC channel restarts its sample when no bytes remain.
// A fetch that ends the sample sets the interrupt flag again.
// https://www.nesdev.org/wiki/APU#Status_($4015)
func (a *APU) writeStatus(value byte) {
	a.dmc.ClearIRQ()

	a.pulse1.SetEnabled(value&0x01 != 0)
	a.pulse2.SetEnabled(value&0x02 != 0)
	a.triangle.SetEnabled(value&0x04 != 0)
	a.noise.SetEnabled(value&0x08 != 0)
	a.dmc.SetEnabled(value&0x10 != 0)

	a.updateIRQ()
}

package nes

import (
	"github.com/retroenv/nesgoemu/pkg/apu/dmc"
	"github.com/retroenv/retrogolib/arch/cpu/cpu6502"
)

type dmaAction byte

const (
	cpuAccess dmaAction = iota
	repeatRead
	dmcRead
	oamRead
	oamWrite
)

type dmcPhase byte

const (
	dmcIdle dmcPhase = iota
	dmcDummy
	dmcReady
)

// dmaController grants the shared CPU bus to DMC and OAM transfers.
// Get cycles have even CPU cycle numbers. Put cycles have odd numbers.
// https://www.nesdev.org/wiki/DMA
type dmaController struct {
	halted bool

	dmcAddress uint16
	dmcArmed   bool
	dmcPhase   dmcPhase

	oamActive  bool
	oamPending bool

	oamPage   byte
	oamOffset uint16

	oamValue byte
	oamFull  bool
}

// RequestOAM selects the page for the next transfer. CPU writes can replace the
// request until the bus controller halts the CPU on a read cycle.
func (d *dmaController) RequestOAM(page byte) {
	d.oamPage = page
	d.oamPending = true
}

func (d *dmaController) next(cycle cpu6502.BusCycle, get bool, request dmc.Request, pending bool) dmaAction {
	if !pending {
		d.dmcArmed = false
	} else if d.dmcPhase == dmcIdle && request.Load == get {
		d.dmcArmed = true
	}
	if cycle.Write {
		return cpuAccess
	}
	startDMC := d.dmcArmed && d.dmcPhase == dmcIdle
	if startDMC {
		d.dmcArmed = false
	}
	if !d.halted {
		if !d.oamPending && !startDMC {
			return cpuAccess
		}
		d.halted = true
		d.oamActive = d.oamPending
		d.oamPending = false
		d.oamOffset = 0
		d.oamFull = false
		if startDMC {
			d.dmcAddress = request.Address
			d.dmcPhase = dmcDummy
		}
		return repeatRead
	}

	return d.transfer(get, request, startDMC)
}

func (d *dmaController) transfer(get bool, request dmc.Request, startDMC bool) dmaAction {
	switch {
	case startDMC:
		d.dmcAddress = request.Address
		d.dmcPhase = dmcDummy
	case d.dmcPhase == dmcDummy:
		d.dmcPhase = dmcReady
	case d.dmcPhase == dmcReady && get:
		d.dmcPhase = dmcIdle
		return dmcRead
	}
	if d.oamActive {
		if get {
			return oamRead
		}
		if d.oamFull {
			return oamWrite
		}
	}
	if !d.oamActive && d.dmcPhase == dmcIdle {
		d.halted = false
		return cpuAccess
	}
	return repeatRead
}

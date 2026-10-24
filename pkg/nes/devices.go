package nes

import (
	"fmt"

	"github.com/retroenv/nesgoemu/pkg/apu"
	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/controller"
	"github.com/retroenv/nesgoemu/pkg/mapper"
	"github.com/retroenv/nesgoemu/pkg/memory"
	"github.com/retroenv/nesgoemu/pkg/ppu/nametable"
	"github.com/retroenv/retrogolib/arch/cpu/cpu6502"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
)

func initializeDevices(cart *cartridge.Cartridge) (*bus.Bus, *memory.Memory, *cpu6502.Memory, error) {
	b := &bus.Bus{
		Cartridge:   cart,
		Controller1: controller.New(),
		Controller2: controller.New(),
		NameTable:   nametable.New(cart.Mirror),
	}
	memory, mem, err := initializeMemory(b)
	if err != nil {
		return nil, nil, nil, err
	}

	b.Mapper, err = mapper.New(b)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("creating mapper: %w", err)
	}

	return b, memory, mem, nil
}

func clockDevices(b *bus.Bus, cycles uint64) {
	clocker, _ := b.Mapper.(bus.CPUClocker)
	for range cycles {
		if clocker != nil {
			clocker.ClockCPU(1)
		}

		b.APU.Step(1)
		b.PPU.Step(3)
	}
}

func clockDeviceCycle(b *bus.Bus, a *apu.APU, d *dmaController, m *memory.Memory, cycles uint64,
	cycle cpu6502.BusCycle) bool {

	request, pending := a.DMCRequest()
	action := d.next(cycle, cycles&1 == 0, request, pending)
	clockDevices(b, 1)
	m.BeginCycle()
	switch action {
	case cpuAccess:
		return false

	case repeatRead:
		dmaRead(b, m, cycle.Address)

	case dmcRead:
		value := dmaRead(b, m, d.dmcAddress)
		a.CompleteDMCTransfer(value, d.dmcCycles)

	case oamRead:
		address := uint16(d.oamPage)<<8 | d.oamOffset
		d.oamValue = dmaRead(b, m, address)
		d.oamFull = true

	case oamWrite:
		b.PPU.Write(0x2004, d.oamValue)
		d.oamFull = false
		d.oamOffset++
		d.oamActive = d.oamOffset < 256
	}

	return true
}

// dmaRead reads on behalf of a DMA transfer. The bus counters list it apart
// from the reads of the CPU.
func dmaRead(b *bus.Bus, m *memory.Memory, address uint16) byte {
	m.SetDMAAccess(true)
	value := b.Memory.Read(address)
	m.SetDMAAccess(false)

	return value
}

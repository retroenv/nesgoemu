package nes

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/feature"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
)

// summaryFieldFormat aligns a summary field name and its value.
const summaryFieldFormat = "  %-11s %s"

// FeatureGroup groups the feature inventory of one hardware component.
type FeatureGroup struct {
	Name     string
	Features []feature.Usage
}

// Summary contains the ROM details and hardware feature usage of a run.
type Summary struct {
	MapperID   uint16
	MapperName string
	PrgROMSize int
	ChrROMSize int
	PrgRAMSize int
	ChrRAMSize int
	MirrorMode cartridge.MirrorMode
	Battery    bool
	Groups     []FeatureGroup
}

// Summary returns the ROM details and feature usage of the current system.
func (sys *System) Summary() Summary {
	cart := sys.Bus.Cartridge
	prgRAM, chrRAM := mapperMemorySizes(sys.Bus.Mapper, cart)

	summary := Summary{
		MapperID:   cart.Mapper,
		MapperName: sys.Bus.Mapper.State().Name,
		PrgROMSize: len(cart.PRG),
		ChrROMSize: len(cart.CHR),
		PrgRAMSize: prgRAM,
		ChrRAMSize: chrRAM,
		MirrorMode: cart.Mirror,
	}
	if backer, ok := sys.Bus.Mapper.(bus.BatteryBacker); ok {
		summary.Battery = backer.BatteryBacked()
	}
	summary.Groups = appendFeatureGroup(summary.Groups, "PPU", sys.Bus.PPU)
	summary.Groups = appendFeatureGroup(summary.Groups, "APU", sys.Bus.APU)
	summary.Groups = appendFeatureGroup(summary.Groups, "Mapper", sys.Bus.Mapper)

	return summary
}

// String returns the summary as plain text.
func (s Summary) String() string {
	lines := []string{
		"Run summary",
		fmt.Sprintf(summaryFieldFormat, "Mapper:", strconv.Itoa(int(s.MapperID))+" "+s.MapperName),
		fmt.Sprintf(summaryFieldFormat, "PRG ROM:", formatSize(s.PrgROMSize)),
		fmt.Sprintf(summaryFieldFormat, "CHR ROM:", formatSize(s.ChrROMSize)),
		fmt.Sprintf(summaryFieldFormat, "PRG RAM:", formatSize(s.PrgRAMSize)),
		fmt.Sprintf(summaryFieldFormat, "CHR RAM:", formatSize(s.ChrRAMSize)),
		fmt.Sprintf(summaryFieldFormat, "Mirroring:", mirrorName(s.MirrorMode)),
		fmt.Sprintf(summaryFieldFormat, "Battery:", yesNo(s.Battery)),
	}

	for _, group := range s.Groups {
		lines = append(lines, "", group.Name+" features")
		for _, feature := range group.Features {
			lines = append(lines, "  ["+usedMark(feature.Used)+"] "+feature.Name)
		}
	}

	return strings.Join(lines, "\n") + "\n"
}

// writeSummary writes the run summary to target. A nil target is ignored.
func (sys *System) writeSummary(target io.Writer) error {
	if target == nil {
		return nil
	}
	if _, err := io.WriteString(target, sys.Summary().String()); err != nil {
		return fmt.Errorf("writing run summary: %w", err)
	}

	return nil
}

func appendFeatureGroup(groups []FeatureGroup, name string, component any) []FeatureGroup {
	user, ok := component.(feature.User)
	if !ok {
		return groups
	}
	features := user.Features()
	if len(features) == 0 {
		return groups
	}

	return append(groups, FeatureGroup{
		Name:     name,
		Features: features,
	})
}

func chrRAMSize(cart *cartridge.Cartridge) int {
	if cart.NES2 != nil {
		return cart.NES2.RAMSizes.CHRVolatile + cart.NES2.RAMSizes.CHRNonvolatile
	}

	return 0
}

func mapperMemorySizes(mapper bus.Mapper, cart *cartridge.Cartridge) (int, int) {
	if sizer, ok := mapper.(bus.MemorySizer); ok {
		return sizer.MemorySizes()
	}

	return mapperbase.PrgRAMSize(cart), chrRAMSize(cart)
}

func formatSize(size int) string {
	if size > 0 && size%1024 == 0 {
		return strconv.Itoa(size/1024) + " KB"
	}

	return strconv.Itoa(size) + " bytes"
}

func mirrorName(mode cartridge.MirrorMode) string {
	switch mode {
	case cartridge.MirrorHorizontal:
		return "horizontal"

	case cartridge.MirrorVertical:
		return "vertical"

	case cartridge.MirrorSingle0:
		return "single-screen 0"

	case cartridge.MirrorSingle1:
		return "single-screen 1"

	case cartridge.Mirror4:
		return "four-screen"

	default:
		return "mode " + strconv.Itoa(int(mode))
	}
}

func usedMark(used bool) string {
	if used {
		return "x"
	}

	return " "
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}

	return "no"
}

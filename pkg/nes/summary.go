package nes

import (
	"cmp"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/feature"
	"github.com/retroenv/nesgoemu/pkg/mapper/mapperbase"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/renderer"
)

// summaryFieldFormat aligns a summary field name and its value.
const summaryFieldFormat = "  %-11s %s"

// FeatureGroup groups the feature inventory of one hardware component.
// The features are sorted: the used features come first, and each part is in
// alphabetical order of the feature names.
type FeatureGroup struct {
	Name     string
	Features []feature.Usage
}

// Summary contains the ROM details and hardware feature usage of a run.
type Summary struct {
	MapperID   uint16
	MapperName string

	// Submapper is the NES 2.0 submapper number. HasSubmapper is false for an
	// iNES 1 header, which has no submapper field.
	Submapper    byte
	HasSubmapper bool

	PrgROMSize int
	ChrROMSize int
	PrgRAMSize int
	ChrRAMSize int
	MirrorMode cartridge.MirrorMode
	Battery    bool

	GUI   *renderer.Info // Renderer of the GUI, nil when the run had no GUI.
	Audio *renderer.Info // Renderer of the audio output, nil when the run had no audio output.

	Groups []FeatureGroup
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
		GUI:        sys.guiInfo,
		Audio:      sys.audioInfo,
	}
	if cart.NES2 != nil {
		summary.Submapper = cart.NES2.Submapper
		summary.HasSubmapper = true
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
		fmt.Sprintf(summaryFieldFormat, "Mapper:", strconv.Itoa(int(s.MapperID))+" - "+s.MapperName),
	}

	if s.HasSubmapper {
		lines = append(lines, fmt.Sprintf(summaryFieldFormat, "Submapper:", strconv.Itoa(int(s.Submapper))))
	}

	lines = append(lines,
		fmt.Sprintf(summaryFieldFormat, "PRG ROM:", formatSize(s.PrgROMSize)),
		fmt.Sprintf(summaryFieldFormat, "CHR ROM:", formatSize(s.ChrROMSize)),
		fmt.Sprintf(summaryFieldFormat, "PRG RAM:", formatSize(s.PrgRAMSize)),
		fmt.Sprintf(summaryFieldFormat, "CHR RAM:", formatSize(s.ChrRAMSize)),
		fmt.Sprintf(summaryFieldFormat, "Mirroring:", mirrorName(s.MirrorMode)),
		fmt.Sprintf(summaryFieldFormat, "Battery:", yesNo(s.Battery)),
	)

	if s.GUI != nil {
		lines = append(lines, fmt.Sprintf(summaryFieldFormat, "GUI:", rendererText(s.GUI)))
	}

	if s.Audio != nil {
		lines = append(lines, fmt.Sprintf(summaryFieldFormat, "Audio:", rendererText(s.Audio)))
	}

	for _, group := range s.Groups {
		lines = append(lines, "", group.Name+" features")
		for _, feature := range group.Features {
			lines = append(lines, "  ["+usedMark(feature.Used)+"] "+feature.Name)
		}
	}

	return strings.Join(lines, "\n") + "\n"
}

// writeSummary writes the run summary to target. A nil target is ignored. The
// summary has the feature lists only when features is true.
func (sys *System) writeSummary(target io.Writer, features bool) error {
	if target == nil {
		return nil
	}

	summary := sys.Summary()
	if !features {
		summary.Groups = nil
	}

	if _, err := io.WriteString(target, summary.String()); err != nil {
		return fmt.Errorf("writing run summary: %w", err)
	}

	return nil
}

// rendererText returns the renderer name and library version. A renderer that
// started after preferred renderers failed also shows the failures on one line.
func rendererText(info *renderer.Info) string {
	text := info.Name
	if info.Version != "" {
		text += " " + info.Version
	}

	if info.Skipped != nil {
		failures := strings.ReplaceAll(info.Skipped.Error(), "\n", "; ")
		text += " (fallback, " + failures + ")"
	}

	return text
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

	sortFeatures(features)

	return append(groups, FeatureGroup{
		Name:     name,
		Features: features,
	})
}

// sortFeatures puts the used features first. Each part is in alphabetical order
// of the feature names, so a reader finds the used features at the top and a
// feature by its name. The components declare their features in the order of
// the identifier names, which differs from the order of the displayed names.
func sortFeatures(features []feature.Usage) {
	slices.SortStableFunc(features, func(a, b feature.Usage) int {
		if a.Used != b.Used {
			if a.Used {
				return -1
			}

			return 1
		}

		return cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
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

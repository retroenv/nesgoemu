// Package main implements a NES ROM emulator.
//
// The blank imports register the SDL renderers with retrogolib. GUI and audio use
// SDL3 first and use SDL2 if SDL3 does not start.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/retroenv/nesgoemu/pkg/nes"
	"github.com/retroenv/nesgoemu/pkg/ppu"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	_ "github.com/retroenv/retrogolib/audio/sdl2"
	_ "github.com/retroenv/retrogolib/audio/sdl3"
	"github.com/retroenv/retrogolib/buildinfo"
	"github.com/retroenv/retrogolib/cli"
	_ "github.com/retroenv/retrogolib/gui/sdl2"
	_ "github.com/retroenv/retrogolib/gui/sdl3"
)

// optionFlags holds the command-line options of the emulator.
type optionFlags struct {
	Features     bool   `flag:"features" usage:"print the hardware features that the program used when the emulator stops"`
	NoSummary    bool   `flag:"no-summary" usage:"do not print the run summary when the emulator stops"`
	Debug        bool   `flag:"d" usage:"start built-in webserver for debug mode"`
	DebugAddress string `flag:"a" usage:"listening address for the debug server to use" default:"127.0.0.1:8080"`
	Entrypoint   int    `flag:"e" usage:"entrypoint to start the CPU" default:"-1"`
	NoGui        bool   `flag:"c" usage:"console mode, disable GUI"`
	NoPrintf     bool   `flag:"no-printf" usage:"ignore program text output written to $401B and $401C"`
	NoAudio      bool   `flag:"m" usage:"mute audio output"`
	Palette      string `flag:"palette" usage:"RGB .pal file (192 or 1536 bytes)"`
	StopAt       int    `flag:"s" usage:"stop execution at address" default:"-1"`
	Tracing      bool   `flag:"t" usage:"print CPU tracing"`
	WAV          string `flag:"wav" usage:"write deterministic audio to a new WAV file"`
	AudioFrames  uint64 `flag:"audio-frames" usage:"number of sample frames to record with -wav" default:"441000"`
}

// positionalArguments holds the positional command-line arguments.
type positionalArguments struct {
	File string `arg:"positional" required:"true" usage:"file to emulate"`
}

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	options, input := readArguments()

	if err := emulateFile(options, input); err != nil {
		fmt.Println(fmt.Errorf("emulation failed: %w", err))
		os.Exit(1)
	}
}

func readArguments() (optionFlags, string) {
	options := optionFlags{}
	positional := positionalArguments{}

	flagSet := cli.NewFlagSet("nesgoemu")
	flagSet.AddSection("Options", &options)
	flagSet.AddPositional(&positional)

	// The flag set prints the usage text by itself for -h and for invalid flags.
	if _, err := flagSet.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, cli.ErrHelpRequested) {
			os.Exit(0)
		}

		var missing *cli.MissingArgsError
		if errors.As(err, &missing) {
			printBanner()
			flagSet.ShowUsage()
		}

		os.Exit(1)
	}

	return options, positional.File
}

func printBanner() {
	fmt.Println("[-----------------------------]")
	fmt.Println("[ nesgoemu - NES ROM emulator ]")
	fmt.Printf("[-----------------------------]\n\n")
	fmt.Printf("version: %s\n\n", buildinfo.Version(version, commit, date))
}

func emulateFile(options optionFlags, input string) error {
	cart, err := readCartridge(input)
	if err != nil {
		return err
	}
	if options.WAV != "" {
		return recordWAV(context.Background(), cart, options)
	}

	opts, err := emulatorOptions(options, cart, input)
	if err != nil {
		return err
	}

	if err := nes.Start(opts...); err != nil {
		return fmt.Errorf("starting emulator: %w", err)
	}

	return nil
}

func emulatorOptions(options optionFlags, cart *cartridge.Cartridge, input string) ([]nes.Option, error) {
	opts := []nes.Option{
		nes.WithCartridge(cart),
		nes.WithSavePath(input + ".sav"),
	}

	if !options.NoSummary {
		opts = append(opts, nes.WithSummaryTarget(os.Stdout))
	}

	if options.Features {
		opts = append(opts, nes.WithSummaryFeatures())
	}

	if options.Palette != "" {
		palette, err := readDisplayPalette(options.Palette)
		if err != nil {
			return nil, err
		}
		opts = append(opts, nes.WithDisplayPalette(palette))
	}

	if options.Debug {
		opts = append(opts, nes.WithDebug(options.DebugAddress))
	}

	if options.Tracing {
		opts = append(opts, nes.WithTracing())
	}

	if options.Entrypoint >= 0 {
		opts = append(opts, nes.WithEntrypoint(options.Entrypoint))
	}

	if options.StopAt >= 0 {
		opts = append(opts, nes.WithStopAt(options.StopAt))
	}

	if options.NoGui {
		opts = append(opts, nes.WithDisabledGUI())
	}

	if options.NoAudio {
		opts = append(opts, nes.WithDisabledAudio())
	}

	if options.NoPrintf {
		opts = append(opts, nes.WithDisabledPrintf())
	}

	return opts, nil
}

func readCartridge(path string) (*cartridge.Cartridge, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file %q: %w", path, err)
	}
	defer func() { _ = file.Close() }()

	cart, err := cartridge.LoadFile(file)
	if err != nil {
		return nil, fmt.Errorf("reading cartridge: %w", err)
	}
	return cart, nil
}

func recordWAV(ctx context.Context, cart *cartridge.Cartridge, options optionFlags) error {
	sys, err := nes.NewSystem(nes.NewOptions(nes.WithCartridge(cart), nes.WithSavePath("")))
	if err != nil {
		return fmt.Errorf("creating recording system: %w", err)
	}
	if options.Entrypoint >= 0 {
		sys.PC = uint16(options.Entrypoint)
	}

	file, err := os.OpenFile(options.WAV, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("creating WAV file: %w", err)
	}
	err = sys.WriteWAV(ctx, file, options.AudioFrames)
	closeErr := file.Close()
	if err != nil {
		return fmt.Errorf("recording WAV file: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("closing WAV file: %w", closeErr)
	}

	return nil
}

// readDisplayPalette reads and parses an RGB .pal file.
func readDisplayPalette(path string) (ppu.DisplayPalette, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ppu.DisplayPalette{}, fmt.Errorf("reading display palette: %w", err)
	}

	palette, err := ppu.ParseDisplayPalette(data)
	if err != nil {
		return ppu.DisplayPalette{}, fmt.Errorf("display palette: %w", err)
	}
	return palette, nil
}

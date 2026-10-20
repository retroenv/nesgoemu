// Package main implements a NES ROM emulator
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/retroenv/nesgoemu/internal/mcpserver"
	"github.com/retroenv/nesgoemu/pkg/nes"
	"github.com/retroenv/nesgoemu/pkg/ppu"
	"github.com/retroenv/retrogolib/arch/system/nes/cartridge"
	"github.com/retroenv/retrogolib/audio"
	audiosdl2 "github.com/retroenv/retrogolib/audio/sdl2"
	"github.com/retroenv/retrogolib/buildinfo"
	"github.com/retroenv/retrogolib/cli"
	"github.com/retroenv/retrogolib/gui"
	guisdl2 "github.com/retroenv/retrogolib/gui/sdl2"
)

// optionFlags holds the command-line options of the emulator.
type optionFlags struct {
	Debug        bool   `flag:"d" usage:"start built-in webserver for debug mode"`
	DebugAddress string `flag:"a" usage:"listening address for the debug server to use" default:"127.0.0.1:8080"`
	Entrypoint   int    `flag:"e" usage:"entrypoint to start the CPU" default:"-1"`
	NoGui        bool   `flag:"c" usage:"console mode, disable GUI"`
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
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		if err := mcpserver.Main(os.Args[2:], buildinfo.Version(version, commit, date)); err != nil {
			fmt.Fprintf(os.Stderr, "mcp server failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

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
	file, err := os.Open(input)
	if err != nil {
		return fmt.Errorf("opening file '%s': %w", input, err)
	}

	cart, err := cartridge.LoadFile(file)
	_ = file.Close()
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}
	if options.WAV != "" {
		return recordWAV(context.Background(), cart, options)
	}

	opts := []nes.Option{
		nes.WithCartridge(cart),
		nes.WithSavePath(input + ".sav"),
		nes.WithSummaryTarget(os.Stdout),
	}

	if options.Palette != "" {
		palette, err := readDisplayPalette(options.Palette)
		if err != nil {
			return err
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
	} else {
		gui.Setup = guisdl2.Setup
		audio.Setup = audiosdl2.Setup
	}
	if options.NoAudio {
		opts = append(opts, nes.WithDisabledAudio())
	}

	if err := nes.Start(opts...); err != nil {
		return fmt.Errorf("starting emulator: %w", err)
	}
	return nil
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

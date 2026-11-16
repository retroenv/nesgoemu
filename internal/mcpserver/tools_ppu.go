package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"strings"

	"github.com/retroenv/nesgoemu/pkg/apu"
	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/ppu"
	"github.com/retroenv/nesgoemu/pkg/ppu/control"
	"github.com/retroenv/retrogolib/arch/system/nes"
	"github.com/retroenv/retrogolib/arch/system/nes/register"
	"github.com/retroenv/retrogolib/mcp"
)

// registerPPUTools registers the PPU, APU, and rendering tools.
func (s *Server) registerPPUTools() error {
	tools := []func() error{
		s.registerNametables,
		s.registerPatternTables,
		s.registerRecordAudio,
	}

	for _, register := range tools {
		if err := register(); err != nil {
			return err
		}
	}

	return nil
}

func (s *Server) registerNametables() error {
	return addImageTool[nametablesArgs](s, mcp.Tool{
		Name:        nametablesToolName,
		Description: "Return the four nametables as tile index grids, and a rendered PNG on request.",
		Annotations: readAnnotations(),
	}, s.nametablesTool)
}

func (s *Server) registerPatternTables() error {
	return addImageTool[machineIDArgs](s, mcp.Tool{
		Name:        patternTablesToolName,
		Description: "Render the two CHR pattern tables as a PNG with the background palette.",
		Annotations: readAnnotations(),
	}, s.patternTablesTool)
}

func (s *Server) registerRecordAudio() error {
	return addTool(s, mcp.Tool{
		Name:        recordAudioToolName,
		Description: "Run frames and write the APU output to a WAV file inside the roots.",
		Annotations: writeAnnotations(),
	}, s.recordAudio)
}

func (s *Server) nametablesTool(_ context.Context, req mcp.ToolRequest) (*mcp.ToolResult, error) {
	var in nametablesArgs
	if err := decodeArgs(req, &in); err != nil {
		return mcp.Errorf(invalidArgumentsFormat, err), nil
	}
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return mcp.Errorf("%v", err), nil
	}
	result, err := machine.nametables()
	if err != nil {
		return mcp.Errorf("%v", err), nil
	}

	content := []mcp.Content{mcp.Text(result.text())}
	if in.Render != nil && *in.Render {
		table := valueOr(in.Table, 0)
		image, err := machine.renderNametable(table)
		if err != nil {
			return mcp.Errorf("%v", err), nil
		}
		content = append(content, mcp.Image(image, mimeTypePNG))
	}

	return &mcp.ToolResult{
		Content: content,
	}, nil
}

func (s *Server) patternTablesTool(_ context.Context, req mcp.ToolRequest) (*mcp.ToolResult, error) {
	var in machineIDArgs
	if err := decodeArgs(req, &in); err != nil {
		return mcp.Errorf(invalidArgumentsFormat, err), nil
	}
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return mcp.Errorf("%v", err), nil
	}
	image, err := machine.patternTables()
	if err != nil {
		return mcp.Errorf("%v", err), nil
	}
	return &mcp.ToolResult{
		Content: []mcp.Content{mcp.Image(image, mimeTypePNG)},
	}, nil
}

func (s *Server) recordAudio(ctx context.Context, _ mcp.ToolRequest, in recordAudioArgs) (recordAudioResult, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return recordAudioResult{}, err
	}
	path, err := s.roots.resolveOutput(in.Path, ".wav")
	if err != nil {
		return recordAudioResult{}, err
	}
	if in.Samples == 0 || in.Samples > maxAudioSamples {
		return recordAudioResult{}, fmt.Errorf("samples %d is outside 1 to %d", in.Samples, maxAudioSamples)
	}
	timeout := maxRunTimeout
	if in.Timeout != "" {
		timeout, err = parseTimeout(in.Timeout)
		if err != nil {
			return recordAudioResult{}, err
		}
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := machine.recordWAV(ctx, path, in.Samples); err != nil {
		return recordAudioResult{}, err
	}
	return recordAudioResult{
		Path:    path,
		Samples: in.Samples,
	}, nil
}

// ppuState returns the PPU register snapshot.
func (m *Machine) ppuState() (ppuSnapshotOut, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, ok := m.sys.Bus.PPU.(*ppu.PPU)
	if !ok {
		return ppuSnapshotOut{}, errors.New("the PPU does not support snapshots")
	}
	snapshot := device.Snapshot()

	return ppuSnapshotOut{
		Dot:      snapshot.Dot,
		Frame:    snapshot.Frame,
		Scanline: snapshot.Scanline,

		Control: int(snapshot.Control),
		Mask:    int(snapshot.Mask),
		Status:  int(snapshot.Status),

		FineX:      int(snapshot.FineX),
		ReadBuffer: int(snapshot.ReadBuffer),
		T:          Address(snapshot.T),
		V:          Address(snapshot.V),
		WriteLatch: snapshot.WriteLatch,
	}, nil
}

// nametables reads the four nametables and their tile grids.
func (m *Machine) nametables() (nametablesData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var data nametablesData
	for table := range nes.NameTableCount {
		base := uint16(0x2000 + table*nes.NameTableSize)
		grid := make([][]int, nametableRows)
		for row := range nametableRows {
			line := make([]int, nametableCols)
			for col := range nametableCols {
				value, _ := m.sys.InspectPPU(base + uint16(row*nametableCols+col))
				line[col] = int(value)
			}

			grid[row] = line
		}

		data.Tables = append(data.Tables, grid)
	}

	return data, nil
}

// renderNametable renders one nametable to a PNG.
func (m *Machine) renderNametable(table int) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if table < 0 || table >= nes.NameTableCount {
		return nil, fmt.Errorf("nametable %d is outside 0 to 3", table)
	}
	device, ok := m.sys.Bus.PPU.(*ppu.PPU)
	if !ok {
		return nil, errors.New("the PPU does not support rendering")
	}
	peek := m.sys.InspectPPU
	base := uint16(0x2000 + table*nes.NameTableSize)
	patternBase := backgroundPatternBase(device)
	paletteData := device.Palette().Data()
	palette := paletteData[:]

	rendered := renderNametable(peek, noEmphasis(device), base, base+0x3C0, patternBase, palette)

	return encodePNG(rendered)
}

// displayColor converts a palette value with the display palette of the PPU.
func (m *Machine) displayColor(value byte) color.RGBA {
	if device, ok := m.sys.Bus.PPU.(*ppu.PPU); ok {
		return device.DisplayColor(value, 0)
	}
	return ppu.RGB(value)
}

// paletteEntries returns the palette as RGB entries.
func (m *Machine) paletteEntries() paletteResult {
	m.mu.Lock()
	defer m.mu.Unlock()

	entries := make([]paletteEntry, 0, nes.PaletteSize)
	for index := range nes.PaletteSize {
		// Read through the PPU address space, so the entries $10, $14, $18,
		// and $1C show the backdrop mirrors, and mask the value to six bits.
		value, _ := m.sys.InspectPPU(uint16(register.PALETTE_START + index))
		value &= 0x3F
		color := m.displayColor(value)
		entries = append(entries, paletteEntry{
			B:     int(color.B),
			Color: int(value),
			G:     int(color.G),
			Index: index,
			R:     int(color.R),
		})
	}

	return paletteResult{
		Entries: entries,
	}
}

// oamEntries returns the sprites.
func (m *Machine) oamEntries() oamResult {
	m.mu.Lock()
	defer m.mu.Unlock()

	oam := m.sys.InspectOAM()
	sprites := make([]spriteEntry, 0, 64)
	for index := range 64 {
		base := index * 4
		sprites = append(sprites, spriteEntry{
			Attributes: int(oam[base+2]),
			Index:      index,
			Tile:       int(oam[base+1]),
			X:          int(oam[base+3]),
			Y:          int(oam[base]),
		})
	}

	return oamResult{
		Sprites: sprites,
	}
}

// patternTables renders the two CHR pattern tables to a PNG.
func (m *Machine) patternTables() ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, ok := m.sys.Bus.PPU.(*ppu.PPU)
	if !ok {
		return nil, errors.New("the PPU does not support pattern tables")
	}
	palette := device.Palette().Data()
	colors := [4]byte{palette[0], palette[1], palette[2], palette[3]}

	peek := device.PeekVRAM
	rgb := noEmphasis(device)
	left := renderPatternTable(peek, rgb, 0x0000, colors)
	right := renderPatternTable(peek, rgb, patternTable, colors)

	combined := image.NewRGBA(image.Rect(0, 0, left.Bounds().Dx()*2, left.Bounds().Dy()))
	drawImage(combined, left, 0)
	drawImage(combined, right, left.Bounds().Dx())

	return encodePNG(combined)
}

// apuState returns the available APU state.
func (m *Machine) apuState() apuStateResult {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := apuStateResult{}
	if device, ok := m.sys.Bus.APU.(*apu.APU); ok {
		result.DMCIRQ = device.DMCIRQ()
		if request, ok := device.DMCRequest(); ok {
			result.DMCAddress = Address(request.Address)
			result.DMCLoad = request.Load
			result.DMCActive = true
		}
	}

	if source, ok := m.sys.Bus.Mapper.(bus.ExpansionAudioSource); ok {
		result.ExpansionAudio = source.ExpansionAudioOutput()
	}

	return result
}

// recordWAV runs the machine and writes a WAV file with the given number of
// samples. The run advances the machine.
func (m *Machine) recordWAV(ctx context.Context, path string, samples uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Audio that earlier runs queued is not part of the new recording.
	m.sys.DiscardQueuedAudio()
	err := writeFileAtomic(path, func(w io.Writer) error {
		return m.sys.Guard(func() error { return m.sys.WriteWAV(ctx, w, samples) })
	})
	if err != nil {
		return fmt.Errorf("recording wav: %w", err)
	}
	return nil
}

// nametablesData holds the four tile grids.
type nametablesData struct {
	Tables [][][]int `json:"tables"`
}

// ppuSnapshotOut reports the PPU register state.
type ppuSnapshotOut struct {
	Dot      int    `json:"dot"`
	Frame    uint64 `json:"frame"`
	Scanline int    `json:"scanline"`

	Control int `json:"control"`
	Mask    int `json:"mask"`
	Status  int `json:"status"`

	FineX      int     `json:"fine_x"`
	ReadBuffer int     `json:"read_buffer"`
	T          Address `json:"t"`
	V          Address `json:"v"`
	WriteLatch bool    `json:"write_latch"`
}

// nametablesArgs selects a table and renders it on request.
type nametablesArgs struct {
	MachineID string `json:"machine_id" description:"Identifier returned by create_machine."`
	Render    *bool  `json:"render,omitempty" description:"Also return a rendered PNG."`
	Table     *int   `json:"table,omitempty" description:"Nametable 0 to 3 to render. Default 0."`
}

// paletteEntry is one palette entry.
type paletteEntry struct {
	B     int `json:"b"`
	Color int `json:"color"`
	G     int `json:"g"`
	Index int `json:"index"`
	R     int `json:"r"`
}

// paletteResult lists the palette entries.
type paletteResult struct {
	Entries []paletteEntry `json:"entries"`
}

// spriteEntry is one OAM sprite.
type spriteEntry struct {
	Attributes int `json:"attributes"`
	Index      int `json:"index"`
	Tile       int `json:"tile"`
	X          int `json:"x"`
	Y          int `json:"y"`
}

// oamResult lists the sprites.
type oamResult struct {
	Sprites []spriteEntry `json:"sprites"`
}

// apuStateResult reports the available APU state.
type apuStateResult struct {
	DMCActive      bool    `json:"dmc_active"`
	DMCAddress     Address `json:"dmc_address"`
	DMCIRQ         bool    `json:"dmc_irq"`
	DMCLoad        bool    `json:"dmc_load"`
	ExpansionAudio float64 `json:"expansion_audio"`
}

// recordAudioArgs selects the output file and length.
type recordAudioArgs struct {
	MachineID string `json:"machine_id" description:"Identifier returned by create_machine."`
	Path      string `json:"path" description:"Output .wav path inside the allowed roots."`
	Samples   uint64 `json:"samples" description:"Number of mono samples at 44100 Hz, up to 2646000 (60 s)."`
	Timeout   string `json:"timeout,omitempty" description:"Wall-clock limit. Default 2m. A recording that reaches it fails and writes no file."`
}

// recordAudioResult reports the written file.
type recordAudioResult struct {
	Path    string `json:"path"`
	Samples uint64 `json:"samples"`
}

// maxAudioSamples limits one recording to 60 seconds.
const maxAudioSamples = 60 * apu.SampleRate

// text renders the tile grids as text.
func (d nametablesData) text() string {
	var builder strings.Builder
	for table, grid := range d.Tables {
		fmt.Fprintf(&builder, "Nametable %d\n", table)
		for _, row := range grid {
			for _, tile := range row {
				fmt.Fprintf(&builder, "%02X ", tile)
			}

			builder.WriteByte('\n')
		}

		builder.WriteByte('\n')
	}

	return builder.String()
}

// decodeArgs decodes raw tool arguments.
func decodeArgs(req mcp.ToolRequest, target any) error {
	if len(req.Arguments) == 0 {
		return nil
	}
	if err := json.Unmarshal(req.Arguments, target); err != nil {
		return fmt.Errorf("decoding arguments: %w", err)
	}
	return nil
}

// encodePNG encodes an image as PNG.
func encodePNG(source image.Image) ([]byte, error) {
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, source); err != nil {
		return nil, fmt.Errorf("encoding png: %w", err)
	}
	return buffer.Bytes(), nil
}

// drawImage copies an image into a target at an x offset.
func drawImage(target, source *image.RGBA, xOffset int) {
	bounds := source.Bounds()
	for y := range bounds.Dy() {
		for x := range bounds.Dx() {
			target.Set(xOffset+x, y, source.At(bounds.Min.X+x, bounds.Min.Y+y))
		}
	}
}

// backgroundPatternBase returns the background pattern table base from PPUCTRL.
func backgroundPatternBase(device *ppu.PPU) uint16 {
	if device.Snapshot().Control&control.CTRL_BG_1000 != 0 {
		return patternTable
	}
	return 0
}

// noEmphasis returns the display color conversion of the PPU without emphasis.
func noEmphasis(device *ppu.PPU) rgbFunc {
	return func(value byte) color.RGBA { return device.DisplayColor(value, 0) }
}

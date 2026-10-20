package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"strings"

	"github.com/retroenv/nesgoemu/pkg/apu"
	"github.com/retroenv/nesgoemu/pkg/bus"
	"github.com/retroenv/nesgoemu/pkg/ppu"
	"github.com/retroenv/retrogolib/mcp"
)

// registerPPUTools registers the PPU, APU, and rendering tools.
func (s *Server) registerPPUTools() error {
	tools := []func() error{
		s.registerPPUState,
		s.registerNametables,
		s.registerPalette,
		s.registerOAM,
		s.registerPatternTables,
		s.registerAPUState,
		s.registerRecordAudio,
	}
	for _, register := range tools {
		if err := register(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) registerPPUState() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name:        "ppu_state",
		Description: "Return the PPU registers, render position, and VRAM addresses.",
		Annotations: readAnnotations(),
	}, s.ppuState)
	return registrationError(err, "ppu_state")
}

func (s *Server) registerNametables() error {
	err := s.mcp.AddTool(mcp.Tool{
		Name:        "nametables",
		Description: "Return the four nametables as tile index grids, and a rendered PNG on request.",
		Annotations: readAnnotations(),
	}, s.nametablesTool)
	return registrationError(err, "nametables")
}

func (s *Server) registerPalette() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name:        "palette",
		Description: "Return the 32 palette entries as RGB colors.",
		Annotations: readAnnotations(),
	}, s.paletteTool)
	return registrationError(err, "palette")
}

func (s *Server) registerOAM() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name:        "oam",
		Description: "Return the 64 sprite entries as x, y, tile, and attributes.",
		Annotations: readAnnotations(),
	}, s.oamTool)
	return registrationError(err, "oam")
}

func (s *Server) registerPatternTables() error {
	err := s.mcp.AddTool(mcp.Tool{
		Name:        "pattern_tables",
		Description: "Render the two CHR pattern tables as a PNG with the background palette.",
		Annotations: readAnnotations(),
	}, s.patternTablesTool)
	return registrationError(err, "pattern_tables")
}

func (s *Server) registerAPUState() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name:        "apu_state",
		Description: "Return the available APU state: DMC request and IRQ, and expansion audio.",
		Annotations: readAnnotations(),
	}, s.apuState)
	return registrationError(err, "apu_state")
}

func (s *Server) registerRecordAudio() error {
	err := mcp.AddTool(s.mcp, mcp.Tool{
		Name:        "record_audio",
		Description: "Run frames and write the APU output to a WAV file inside the roots.",
		Annotations: writeAnnotations(),
	}, s.recordAudio)
	return registrationError(err, "record_audio")
}

func (s *Server) ppuState(_ context.Context, _ mcp.ToolRequest, in machineIDArgs) (ppuSnapshotOut, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return ppuSnapshotOut{}, err
	}
	return machine.ppuState()
}

func (s *Server) nametablesTool(_ context.Context, req mcp.ToolRequest) (*mcp.ToolResult, error) {
	var in nametablesArgs
	if err := decodeArgs(req, &in); err != nil {
		return mcp.Errorf("Invalid arguments: %v", err), nil
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
		content = append(content, mcp.Image(image, "image/png"))
	}
	return &mcp.ToolResult{Content: content}, nil
}

func (s *Server) paletteTool(_ context.Context, _ mcp.ToolRequest, in machineIDArgs) (paletteResult, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return paletteResult{}, err
	}
	return machine.paletteEntries(), nil
}

func (s *Server) oamTool(_ context.Context, _ mcp.ToolRequest, in machineIDArgs) (oamResult, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return oamResult{}, err
	}
	return machine.oamEntries(), nil
}

func (s *Server) patternTablesTool(_ context.Context, req mcp.ToolRequest) (*mcp.ToolResult, error) {
	var in machineIDArgs
	if err := decodeArgs(req, &in); err != nil {
		return mcp.Errorf("Invalid arguments: %v", err), nil
	}
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return mcp.Errorf("%v", err), nil
	}
	image, err := machine.patternTables()
	if err != nil {
		return mcp.Errorf("%v", err), nil
	}
	return &mcp.ToolResult{Content: []mcp.Content{mcp.Image(image, "image/png")}}, nil
}

func (s *Server) apuState(_ context.Context, _ mcp.ToolRequest, in machineIDArgs) (apuStateResult, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return apuStateResult{}, err
	}
	return machine.apuState(), nil
}

func (s *Server) recordAudio(ctx context.Context, _ mcp.ToolRequest, in recordAudioArgs) (recordAudioResult, error) {
	machine, err := s.registry.get(in.MachineID)
	if err != nil {
		return recordAudioResult{}, err
	}
	path, err := s.roots.resolve(in.Path)
	if err != nil {
		return recordAudioResult{}, err
	}
	if in.Samples == 0 || in.Samples > maxAudioSamples {
		return recordAudioResult{}, fmt.Errorf("samples %d is outside 1 to %d", in.Samples, maxAudioSamples)
	}
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
		Scanline:   snapshot.Scanline,
		Dot:        snapshot.Dot,
		Frame:      snapshot.Frame,
		Control:    int(snapshot.Control),
		Mask:       int(snapshot.Mask),
		Status:     int(snapshot.Status),
		V:          Address(snapshot.V),
		T:          Address(snapshot.T),
		FineX:      int(snapshot.FineX),
		WriteLatch: snapshot.WriteLatch,
		ReadBuffer: int(snapshot.ReadBuffer),
	}, nil
}

// nametables reads the four nametables and their tile grids.
func (m *Machine) nametables() (nametablesData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var data nametablesData
	for table := range 4 {
		base := uint16(0x2000 + table*0x400)
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

	if table < 0 || table > 3 {
		return nil, fmt.Errorf("nametable %d is outside 0 to 3", table)
	}
	device, ok := m.sys.Bus.PPU.(*ppu.PPU)
	if !ok {
		return nil, errors.New("the PPU does not support rendering")
	}
	peek := m.sys.InspectPPU
	base := uint16(0x2000 + table*0x400)
	patternBase := backgroundPatternBase(device)
	paletteData := device.Palette().Data()
	palette := paletteData[:]

	rendered := renderNametable(peek, base, base+0x3C0, patternBase, palette)
	return encodePNG(rendered)
}

// paletteEntries returns the palette as RGB entries.
func (m *Machine) paletteEntries() paletteResult {
	m.mu.Lock()
	defer m.mu.Unlock()

	data := m.sys.Bus.PPU.Palette().Data()
	entries := make([]paletteEntry, 0, len(data))
	for index, value := range data {
		color := ppu.RGB(value)
		entries = append(entries, paletteEntry{
			Index: index,
			Color: int(value),
			R:     int(color.R),
			G:     int(color.G),
			B:     int(color.B),
		})
	}
	return paletteResult{Entries: entries}
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
			Index:      index,
			Y:          int(oam[base]),
			Tile:       int(oam[base+1]),
			Attributes: int(oam[base+2]),
			X:          int(oam[base+3]),
		})
	}
	return oamResult{Sprites: sprites}
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
	left := renderPatternTable(peek, 0x0000, colors)
	right := renderPatternTable(peek, patternTable, colors)

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

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating wav file: %w", err)
	}
	err = m.sys.Guard(func() error { return m.sys.WriteWAV(ctx, file, samples) })
	if err != nil {
		_ = file.Close()
		return fmt.Errorf("recording wav: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("closing wav file: %w", err)
	}
	return nil
}

// nametablesData holds the four tile grids.
type nametablesData struct {
	Tables [][][]int `json:"tables"`
}

// ppuSnapshotOut reports the PPU register state.
type ppuSnapshotOut struct {
	Scanline   int     `json:"scanline"`
	Dot        int     `json:"dot"`
	Frame      uint64  `json:"frame"`
	Control    int     `json:"control"`
	Mask       int     `json:"mask"`
	Status     int     `json:"status"`
	V          Address `json:"v"`
	T          Address `json:"t"`
	FineX      int     `json:"fine_x"`
	WriteLatch bool    `json:"write_latch"`
	ReadBuffer int     `json:"read_buffer"`
}

// nametablesArgs selects a table and renders it on request.
type nametablesArgs struct {
	MachineID string `json:"machine_id" description:"Identifier returned by create_machine."`
	Table     *int   `json:"table,omitempty" description:"Nametable 0 to 3 to render. Default 0."`
	Render    *bool  `json:"render,omitempty" description:"Also return a rendered PNG."`
}

// paletteEntry is one palette entry.
type paletteEntry struct {
	Index int `json:"index"`
	Color int `json:"color"`
	R     int `json:"r"`
	G     int `json:"g"`
	B     int `json:"b"`
}

// paletteResult lists the palette entries.
type paletteResult struct {
	Entries []paletteEntry `json:"entries"`
}

// spriteEntry is one OAM sprite.
type spriteEntry struct {
	Index      int `json:"index"`
	X          int `json:"x"`
	Y          int `json:"y"`
	Tile       int `json:"tile"`
	Attributes int `json:"attributes"`
}

// oamResult lists the sprites.
type oamResult struct {
	Sprites []spriteEntry `json:"sprites"`
}

// apuStateResult reports the available APU state.
type apuStateResult struct {
	DMCAddress     Address `json:"dmc_address"`
	DMCLoad        bool    `json:"dmc_load"`
	DMCActive      bool    `json:"dmc_active"`
	DMCIRQ         bool    `json:"dmc_irq"`
	ExpansionAudio float64 `json:"expansion_audio"`
}

// recordAudioArgs selects the output file and length.
type recordAudioArgs struct {
	MachineID string `json:"machine_id" description:"Identifier returned by create_machine."`
	Path      string `json:"path" description:"Output .wav path inside the allowed roots."`
	Samples   uint64 `json:"samples" description:"Number of mono samples at 44100 Hz, up to 2646000 (60 s)."`
}

// recordAudioResult reports the written file.
type recordAudioResult struct {
	Path    string `json:"path"`
	Samples uint64 `json:"samples"`
}

// maxAudioSamples limits one recording to 60 seconds at 44100 Hz.
const maxAudioSamples = 60 * 44100

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
	if device.Snapshot().Control&0x10 != 0 {
		return patternTable
	}
	return 0
}

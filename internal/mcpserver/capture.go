package mcpserver

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/retroenv/retrogolib/mcp"
)

// maxCaptureReads bounds the memory ranges of one run capture.
const maxCaptureReads = 8

func (m *Machine) captureRun(result runFramesOut, capture preparedCapture) (runFramesOut, error) {
	if !capture.requested {
		return result, nil
	}

	observed := &runCaptureResult{}

	for _, read := range capture.reads {
		observed.Memory = append(observed.Memory, m.readMemory(read.space, read.start, read.length))
	}

	if capture.screenshot {
		data, err := m.screenshot(capture.scale)
		if err != nil {
			return runFramesOut{}, fmt.Errorf("ran %d frames to frame %d; screenshot capture failed: %w",
				result.CompletedFrames, result.Frame, err)
		}
		result.image = data
		observed.Screenshot = true
		observed.Scale = capture.scale
	}

	result.Capture = observed

	return result, nil
}

type runCaptureArgs struct {
	Memory     []captureMemoryArgs `json:"memory,omitempty" description:"At most 8 memory ranges, each at most 4096 bytes. Reads do not advance time."`
	Scale      *int                `json:"scale,omitempty" minimum:"1" maximum:"4" description:"Screenshot scale 1 to 4. Requires screenshot true. Default 1."`
	Screenshot bool                `json:"screenshot,omitempty" description:"Include a PNG image from the stopped machine. Default false."`
}

type captureMemoryArgs struct {
	Address Address `json:"address,omitempty" description:"CPU or PPU address. Default 0."`
	Length  *int    `json:"length,omitempty" minimum:"1" maximum:"4096" description:"Byte count. Default 256; maximum 4096."`
	Offset  *int    `json:"offset,omitempty" minimum:"0" description:"Physical PRG or CHR offset. Replaces address."`
	Space   string  `json:"space" enum:"cpu,ppu,oam,prg,chr,ram" description:"Memory space to read."`
}

type runCaptureResult struct {
	Memory     []readMemoryResult `json:"memory,omitempty"`
	Scale      int                `json:"scale,omitempty"`
	Screenshot bool               `json:"screenshot"`
}

type preparedCapture struct {
	reads      []memoryRead
	requested  bool
	scale      int
	screenshot bool
}

type memoryRead struct {
	length int
	space  string
	start  int
}

func prepareCapture(in *runCaptureArgs) (preparedCapture, error) {
	if in == nil {
		return preparedCapture{}, nil
	}
	if len(in.Memory) > maxCaptureReads {
		return preparedCapture{}, fmt.Errorf("capture accepts at most %d memory ranges", maxCaptureReads)
	}
	if in.Scale != nil && !in.Screenshot {
		return preparedCapture{}, errors.New("capture scale requires screenshot true")
	}
	scale := valueOr(in.Scale, defaultImageScale)
	if scale < 1 || scale > maxImageScale {
		return preparedCapture{}, fmt.Errorf("capture scale must be from 1 to %d", maxImageScale)
	}
	prepared := preparedCapture{
		requested:  true,
		scale:      scale,
		screenshot: in.Screenshot,
	}

	for _, read := range in.Memory {
		selection, err := prepareMemoryRead(read.Space, read.Address, read.Offset, read.Length)
		if err != nil {
			return preparedCapture{}, fmt.Errorf("capture memory: %w", err)
		}
		prepared.reads = append(prepared.reads, selection)
	}

	return prepared, nil
}

func prepareMemoryRead(space string, address Address, offset, length *int) (memoryRead, error) {
	if !validSpace(space) {
		return memoryRead{}, fmt.Errorf("space %q is not one of cpu, ppu, oam, prg, chr, ram", space)
	}
	count := valueOr(length, defaultReadLength)
	if count <= 0 || count > maxReadLength {
		return memoryRead{}, fmt.Errorf("length %d is outside 1 to %d", count, maxReadLength)
	}
	start := int(address)
	if offset != nil {
		if space != spacePRG && space != spaceCHR {
			return memoryRead{}, errors.New("offset applies to the prg and chr spaces only; use address")
		}
		start = *offset
	}

	if start < 0 {
		return memoryRead{}, fmt.Errorf(negativeOffsetFormat, start)
	}
	return memoryRead{
		length: count,
		space:  space,
		start:  start,
	}, nil
}

func runCaptureContent(result runFramesOut) []mcp.Content {
	if len(result.image) == 0 {
		return nil
	}
	return []mcp.Content{mcp.Image(result.image, mimeTypePNG)}
}

func runFramesInputSchema() ([]byte, error) {
	return inputSchema(reflect.TypeFor[runFramesArgs](), configureCaptureSchema)
}

func configureCaptureSchema(schema map[string]any) {
	capture := schemaProperty(schema, fieldCapture)
	memory := schemaProperty(capture, fieldMemory)
	memory[schemaMaxItems] = maxCaptureReads
	capture[schemaOneOf] = []any{
		map[string]any{
			schemaRequired:   []string{fieldScreenshot},
			schemaProperties: map[string]any{fieldScreenshot: map[string]any{schemaEnum: []bool{true}}},
		},
		map[string]any{
			schemaProperties: map[string]any{
				fieldScale:      false,
				fieldScreenshot: map[string]any{schemaEnum: []bool{false}},
			},
		},
	}
}

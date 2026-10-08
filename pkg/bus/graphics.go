package bus

// GraphicsEventKind identifies a mapper graphics operation.
type GraphicsEventKind uint8

const (
	// GraphicsMemoryWrite records an upload, including unchanged data.
	GraphicsMemoryWrite GraphicsEventKind = iota
	// GraphicsNametableFetch identifies a page used by a visible background fetch.
	GraphicsNametableFetch
	// GraphicsCHRMapping records an effective CHR window mapping.
	GraphicsCHRMapping
)

// GraphicsMemory identifies a physical graphics memory source.
type GraphicsMemory uint8

const (
	GraphicsFPGARAM GraphicsMemory = iota
	GraphicsCHRRAM
	GraphicsCHRROM
	GraphicsCIRAM
)

// GraphicsMapping identifies a physical memory byte.
// Offset is measured from the start of Memory.
type GraphicsMapping struct {
	Memory GraphicsMemory
	Offset int
}

// PPUMappingInspector reports pattern-table mappings at $0000-$1FFF.
// Read mappings use the current graphics context. Write mappings report RAM
// or flash targets. Unmapped addresses and generated read data return false.
// Queries do not read memory or change state. The caller must stop emulation
// or call the queries on the emulation goroutine.
type PPUMappingInspector interface {
	PPUReadMapping(address uint16) (GraphicsMapping, bool)
	PPUWriteMapping(address uint16) (GraphicsMapping, bool)
}

// GraphicsPage identifies a physical 1 KiB memory page.
type GraphicsPage struct {
	Memory GraphicsMemory
	Index  uint32
}

// GraphicsEvent describes an upload, a visible nametable fetch, or a CHR mapping.
// Offset is the byte offset in Page for uploads. Slot identifies the nametable
// slot for fetches or the PPU start address for CHR mappings.
type GraphicsEvent struct {
	Kind   GraphicsEventKind
	Page   GraphicsPage
	Slot   uint16
	Offset uint16
}

// MapperGraphicsObserver is an optional mapper graphics event source.
// Events run on the emulation goroutine. The callback must not change mapper
// state. A nil callback disables events. Events are not game frame boundaries.
type MapperGraphicsObserver interface {
	ObserveGraphics(observer func(GraphicsEvent))
}

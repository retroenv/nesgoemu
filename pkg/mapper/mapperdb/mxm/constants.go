package mxm

// Register names describe mapper 800 in the demo package. Readback and
// write layouts overlap but are not symmetric, so their ranges have separate
// names. These addresses do not apply to the earlier mapper-220 minimapper.
const name = "MXM"

const (
	// "Identity" is a descriptive name. Reads of $5000-$5005 return these six
	// fixed bytes. The meaning of each byte is not known. Keep them unchanged
	// for compatibility.
	identityResponse = "\x9f\x00\x17\x14\x17\x14"

	registerGroupMask = 0xfff0 // The decoder dispatches blocks of 16 addresses.

	// The fixed-response registers ignore writes. StateData reads a sparse snapshot
	// and advances StateCursor; writing StateCursor selects the next byte.
	regIdentityStart = 0x5000
	regIdentityEnd   = 0x5005
	regStateData     = 0x5006
	regStateCursor   = 0x5007

	// Select a CPU window, its source flags, then its 16-bit 8 KiB bank.
	// The alternate bank temporarily replaces only the $C000 window; the
	// normal bank registers remain available for restoration when disabled.
	regPRGMode       = 0x5010 // Stored/read back; this core keeps 8 KiB windows.
	regPRGSelect     = 0x5011
	regPRGBankHigh   = 0x5012
	regPRGBankLow    = 0x5013
	regAlternateMode = 0x5014
	regAlternateRAM  = 0x5015
	regAlternateHigh = 0x5016
	regAlternateLow  = 0x5017

	// Reads expose four consecutive {flags, bank high, bank low} records.
	// For example, reading $5011 reads bank 0's high byte, although writing
	// $5011 changes the window selector and flags.
	regPRGReadback    = 0x5010
	regPRGReadbackEnd = 0x501b
	regPRGSelection   = 0x501c
	prgReadbackStride = 3

	// Sixteen independently banked 512-byte PPU windows cover $0000-$1FFF.
	// RAM source selection means cartridge WorkRam, not the CHR-RAM allocation.
	regCHRMode     = 0x5020 // Stored; window size remains 512 bytes.
	regCHRSelect   = 0x5021
	regCHRBankHigh = 0x5022
	regCHRBankLow  = 0x5023

	// A detected scanline match latches pending state. Enabled matches also
	// assert CPU IRQ. Reading Acknowledge clears it and returns a phase value
	// that the game uses to compensate for CPU interrupt-entry timing.
	regIRQEnable      = 0x5030
	regIRQTarget      = 0x5031
	regIRQOffset      = 0x5032 // Stored, but unused by the demo's comparator.
	regIRQAcknowledge = 0x5033

	// Extended backgrounds use a per-tile descriptor table for each nametable.
	// The descriptor chooses a pattern bank and supplies palette information;
	// the normal PPU still consumes name, attribute, low-plane and high-plane bytes.
	regDescriptorMode   = 0x5040
	regDescriptorSelect = 0x5041
	regDescriptorHigh   = 0x5042
	regDescriptorLow    = 0x5043
	regManualScroll     = 0x504e // Nonzero prevents captured $2005 writes from setting Y.
	regVerticalPosition = 0x504f

	// Banked nametable reads select 1 KiB windows in graphics ROM or WorkRam.
	// Enabling this mode also redirects ALL PPU writes through WriteBank;
	// the upload destination is independent of the nametable read banks.
	regNametableMode   = 0x5050
	regNametableSelect = 0x5051
	regNametableHigh   = 0x5052
	regNametableLow    = 0x5053

	// Background and sprite paths have separate second-set selectors. The
	// read inhibit counts CPU cycles; it suppresses data without stopping
	// fetch/IRQ tracking. WriteBank selects a 64 KiB WorkRam upload segment.
	// Selecting another graphics set changes emulated fetch data immediately;
	// any blending of consecutive displayed frames happens outside the mapper.
	regPlaneMode      = 0x5080
	regPlaneSelect    = 0x5081
	regReadInhibit    = 0x5082
	regWriteBankGroup = 0x5090
	regWriteBankHigh  = 0x5092
	regWriteBankLow   = 0x5093
	regGraphicsLatch  = 0x50b0

	// Copy offsets and count are in 8 KiB units. Writing Count executes the
	// copy immediately, without a busy period. Unlike other bank registers,
	// these address pairs put the LOW byte at the lower register address.
	regCopyGroup       = 0x50c0
	regCopySource      = 0x50c1
	regCopyDestination = 0x50c2 // Retained selector; copies always target WorkRam.
	regCopyFromLow     = 0x50c3
	regCopyFromHigh    = 0x50c4
	regCopyToLow       = 0x50c5
	regCopyToHigh      = 0x50c6
	regCopyCount       = 0x50c7

	// Metatile mode bypasses ordinary tile decoding: WorkRam tables supply
	// complete palette/plane bytes, optionally through one animation lookup.
	regMetatileMode   = 0x50d0
	regMetatileTable0 = 0x50d1
	regMetatileTable1 = 0x50d2
	regMetatileTable2 = 0x50d3
	regMetatileTable3 = 0x50d4
	regMetatileStride = 0x50d5 // Selects 8, 16, 32 or 64 entries per animation.
	regMetatileFrame  = 0x50d6

	// Reading Mode resets the upload cursor; writing Data uploads successive
	// entries across the whole table. Page selects the 256-byte lookup page
	// used for rendering, not the upload destination.
	regRemapMode = 0x50e0
	regRemapData = 0x50e1
	regRemapPage = 0x50e2
	regScanline  = 0x5116
)

const (
	controlEnable        = 0x01
	bankSourceRAM        = 0x80 // Clear selects ROM; set selects cartridge WorkRam.
	prgGraphicsLatch     = 0x40 // CPU reads return zero; actual data goes to $50B0.
	prgFlagsMask         = 0xf8 // Bits 5:3 are preserved even though unused here.
	prgSelectMask        = 0x03
	chrSelectMask        = 0x0f
	ntSelectMask         = 0x03
	modeMask             = 0x03
	extendedModeEnable   = 0x80 // Enables descriptor or banked-nametable handling.
	descriptorPatternRAM = 0x40 // Pattern source, independent of descriptor source.
	planeBackgroundBit   = 0x01
	planeSpriteBit       = 0x02
	remapPageMask        = 0x03

	copySourcePRG      = 0
	copySourceRAM      = 1
	copySourceCHR      = 2
	copySourceRAMAlias = 3 // This selector also selects RAM.

	attributeModeTile        = 0 // One palette per tile descriptor.
	attributeModeFineY       = 1 // Palette descriptors have a bank-sized stride per row.
	attributeModePackedRows  = 2 // Four scanlines' two-bit palettes packed into a byte.
	attributeModePatternOnly = 3 // Attribute fetches take the pattern path as well.
)

// Offsets in the $5006 state stream are independent of CPU register addresses.
const (
	statePRGMode        = 0x00
	statePRGSelect      = 0x01
	statePRGBankHigh    = 0x02
	statePRGBankLow     = 0x03
	stateDescriptor     = 0x32
	stateDescriptorHigh = 0x33
	stateDescriptorLow  = 0x34
	stateWide           = 0x50
	stateSpriteMode     = 0x51
	stateSecondPlane    = 0x52
	stateSpritePlane    = 0x53
	stateUnavailable    = 0xff // Unimplemented stream fields return this byte.
)

const (
	// This board's CPU windows start at $6000. Four can switch; the fifth
	// ($E000) is fixed. $6000 is not universally RAM: the older mapper-220
	// minimapper places control and palette-response registers in this range.
	// Masks select a byte inside a window, while bank registers choose its base.
	prgBankSize    = 0x2000
	chrBankSize    = 0x0200
	ntBankSize     = 0x0400
	prgOffsetMask  = prgBankSize - 1
	chrOffsetMask  = chrBankSize - 1
	ntOffsetMask   = ntBankSize - 1
	cpuWindowBase  = 0x6000
	cpuWindowCount = 5
	prgWindowCount = 4 // $8000-$FFFF, excluding the low $6000 window.
	chrWindowCount = 16
	ntWindowCount  = 4
	alternateSlot  = 3      // $C000-$DFFF.
	lastPRGBank    = 0xffff // Bank -1 as a signed 16-bit number; see mapCPU.
	defaultCHRRAM  = 0x2000
	nmiVectorLow   = 0xfffa
	nmiVectorHigh  = 0xfffb
	irqPhaseMask   = 0x0f // Four-bit counter, advanced once per CPU cycle.
	irqAssertPhase = 3    // Reloaded on the first CPU clock observing pending IRQ.
	byteMask       = 0x00ff
	bankHighMask   = 0xff00

	// Reset maps banks $100..$10F, except slot 10 maps bank 3. These effective
	// mappings are separate from the bank-register readback words, which reset
	// to zero. Preserve the observed quirk rather than deriving one from the other.
	initialCHRBank          = 0x100
	initialCHRExceptionSlot = 10
	initialCHRExceptionBank = 3
)

const (
	// PPU $2000-$2FFF contains the four nametables; the last 64 bytes of each
	// 1 KiB page are attributes. $3000-$3EFF mirrors nametable storage but is
	// outside the name-fetch range used by the mapper's IRQ/phase detector.
	patternTableEnd       = 0x2000
	nameTableStart        = 0x2000
	nameTableFetchEnd     = 0x3000
	paletteStart          = 0x3f00
	attributeTableOffset  = 0x03c0
	attributeTableBase    = nameTableStart | attributeTableOffset
	nameTableSelectMask   = 0x0c00
	attributeRowMask      = 0x38
	verticalRemapKeepMask = 0xfc1f // Replace coarse Y (bits 9:5), preserve other bits.
	patternFineYKeepMask  = 0xfff8 // Replace pattern-row fine Y (bits 2:0).
	tileHeight            = 8
	tilesPerRow           = 32
	tileRowShift          = 5
	fineYMask             = tileHeight - 1
	tileColumnMask        = tilesPerRow - 1
	paletteMask           = 0x03

	// The PPU selects one of four two-bit fields from an attribute fetch.
	// Multiplication by binary 01010101 repeats a palette number in all fields:
	// 0, 1, 2, 3 become $00, $55, $AA, $FF. These are NES palette selectors,
	// not RGB colors or entries in a .pal display file.
	paletteRepeat = 0x55

	descriptorSize     = 2 // Big-endian word: high-byte palette bits and low 12-bit bank.
	descriptorBankSize = 0x0800
	patternPageSize    = 0x1000
	patternPageMask    = patternPageSize - 1
	descriptorBankMask = 0x0fff

	// These counts are tile-name reads, not dots or bytes. Each sprite slot
	// gives one name read. The mapper replaces background data only when
	// (nameReads + 2) % 42 < 33.
	fetchPhaseCount      = 42
	fetchPhaseOffset     = 2
	backgroundFetchCount = 33 // Also advances the mapper's vertical position.
	postNameFetchCount   = 3  // Attribute, pattern low, pattern high.
	remapPageSize        = 256
	remapPageCount       = 4
	remapTableSize       = remapPageSize * remapPageCount
	writeBankShift       = 16
	ppuRegisterCount     = 8
	ppuRegisterMask      = ppuRegisterCount - 1
	ppuScrollIndex       = 5

	visibleScanlines  = 240
	preRenderScanline = 261

	// The reference emulator of the demo fetches an attribute address in the
	// second dummy fetch of each sprite slot. At these dots, the mapper reads
	// the second nametable fetch of nesgoemu as an attribute fetch, so the
	// phase counts above match the demo. See TickPPU.
	firstDummyAttribute = 259
	lastDummyAttribute  = 315
	spriteFetchSlotDots = 8
)

// These are fixed WorkRam layouts used by the demo's metatile fetch engine,
// not CPU/PPU bus addresses. Each name-table cell has eight fine-Y entries.
// Each entry is a big-endian ID that selects predecoded plane and palette data.
const (
	metatilePlaneTable     = 0x80000 // Four bytes per ID: low/high planes for two sets.
	metatilePaletteTable   = 0xb0000 // Two IDs per byte; each ID owns two two-bit palettes.
	metatileTable0         = 0xb8000
	metatileTable1         = 0xbc000
	metatileExtraTables    = 0xe4000
	metatileTableSize      = 0x4000
	metatileFallbackTable  = 0xba800 // Used when a table selector exceeds the nine entries.
	metatileAnimationTable = 0xc0000 // Frame-indexed replacement IDs; one lookup only.
	metatileAnimationMask  = 0xc000  // Both high ID bits set means animation indirection.
	metatileIndexMask      = 0x3fff
	metatileRecordSize     = 4
	metatileDefaultStride  = 8
)

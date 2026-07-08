// CRT cartridge image builder.

package c64

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
)

// CartridgeType represents the hardware EXROM/GAME line configuration.
type CartridgeType int

const (
	// CRTNormal8K represents an 8 KiB cartridge (EXROM=0, GAME=1).
	CRTNormal8K CartridgeType = iota
	// CRTNormal16K represents a 16 KiB cartridge (EXROM=0, GAME=0).
	CRTNormal16K
	// CRTUltimax represents an Ultimax cartridge (EXROM=1, GAME=0).
	CRTUltimax
)

// ChipType represents the type of chip in a CHIP packet.
type ChipType uint16

const (
	ChipTypeROM    ChipType = 0
	ChipTypeRAM    ChipType = 1
	ChipTypeFlash  ChipType = 2
	ChipTypeEEPROM ChipType = 3
)

// Chip represents a CHIP packet in a .crt file.
type Chip struct {
	Type        ChipType
	Bank        uint16
	LoadAddress uint16
	Data        []byte
}

// Cartridge represents the high-level structure of a C64 cartridge image.
type Cartridge struct {
	Name     string
	Type     CartridgeType
	HwType   uint16 // Typically 0 (normal)
	Subtype  byte
	Exrom    byte
	Game     byte
	Chips    []Chip
}

// Bytes serializes the Cartridge into a VICE-compatible .crt byte stream.
func (c *Cartridge) Bytes() ([]byte, error) {
	var buf bytes.Buffer

	// 1. Write global header (64 bytes)
	// Signature (16 bytes)
	sig := [16]byte{}
	copy(sig[:], "C64 CARTRIDGE   ")
	buf.Write(sig[:])

	// Header Length (4 bytes, big-endian)
	if err := binary.Write(&buf, binary.BigEndian, uint32(64)); err != nil {
		return nil, err
	}

	// Version (2 bytes, big-endian, typically 0x0100)
	if err := binary.Write(&buf, binary.BigEndian, uint16(0x0100)); err != nil {
		return nil, err
	}

	// Hardware Type (2 bytes, big-endian)
	if err := binary.Write(&buf, binary.BigEndian, c.HwType); err != nil {
		return nil, err
	}

	// EXROM and GAME lines
	buf.WriteByte(c.Exrom)
	buf.WriteByte(c.Game)

	// Subtype (1 byte)
	buf.WriteByte(c.Subtype)

	// Reserved (5 bytes, zeroed)
	buf.Write(make([]byte, 5))

	// Name (32 bytes, uppercase ASCII, zero-padded)
	nameBytes := [32]byte{}
	copy(nameBytes[:], c.Name)
	// Convert to uppercase as recommended
	for i, b := range nameBytes {
		if b >= 'a' && b <= 'z' {
			nameBytes[i] = b - 'a' + 'A'
		}
	}
	buf.Write(nameBytes[:])

	// 2. Write CHIP packets
	for i, chip := range c.Chips {
		dataSize := len(chip.Data)
		if dataSize == 0 {
			return nil, fmt.Errorf("chip %d data cannot be empty", i)
		}

		targetSize := 8192
		if dataSize > 8192 {
			targetSize = 16384
		}
		if dataSize > 16384 {
			return nil, fmt.Errorf("chip %d data too large: %d bytes (max 16 KiB)", i, dataSize)
		}

		paddedData := chip.Data
		if dataSize != targetSize {
			paddedData = make([]byte, targetSize)
			copy(paddedData, chip.Data)
			for j := dataSize; j < targetSize; j++ {
				paddedData[j] = 0xFF
			}
		}

		// Signature (4 bytes): "CHIP"
		buf.Write([]byte("CHIP"))

		// Packet Length (4 bytes, big-endian) = 16 + targetSize
		pktLen := uint32(16 + targetSize)
		if err := binary.Write(&buf, binary.BigEndian, pktLen); err != nil {
			return nil, err
		}

		// Chip Type (2 bytes, big-endian)
		if err := binary.Write(&buf, binary.BigEndian, uint16(chip.Type)); err != nil {
			return nil, err
		}

		// Bank Number (2 bytes, big-endian)
		if err := binary.Write(&buf, binary.BigEndian, chip.Bank); err != nil {
			return nil, err
		}

		// Load Address (2 bytes, big-endian)
		if err := binary.Write(&buf, binary.BigEndian, chip.LoadAddress); err != nil {
			return nil, err
		}

		// Data Size (2 bytes, big-endian)
		if err := binary.Write(&buf, binary.BigEndian, uint16(targetSize)); err != nil {
			return nil, err
		}

		// Payload data
		buf.Write(paddedData)
	}

	return buf.Bytes(), nil
}

// NewRawCartridge wraps raw ROM bytes into a .CRT file of the specified type.
// It automatically splits/pads and handles load addresses appropriately.
func NewRawCartridge(cartType CartridgeType, name string, romData []byte) ([]byte, error) {
	var exrom, game byte
	switch cartType {
	case CRTNormal8K:
		exrom = 0
		game = 1
	case CRTNormal16K:
		exrom = 0
		game = 0
	case CRTUltimax:
		exrom = 1
		game = 0
	default:
		return nil, fmt.Errorf("invalid cartridge type: %v", cartType)
	}

	c := &Cartridge{
		Name:    name,
		Type:    cartType,
		Exrom:   exrom,
		Game:    game,
		HwType:  0,
		Subtype: 0,
	}

	switch cartType {
	case CRTNormal8K:
		if len(romData) > 8192 {
			return nil, fmt.Errorf("rom data size %d exceeds 8 KiB limit for 8K cartridge", len(romData))
		}
		c.Chips = []Chip{
			{
				Type:        ChipTypeROM,
				LoadAddress: 0x8000,
				Data:        romData,
			},
		}
	case CRTNormal16K:
		if len(romData) > 16384 {
			return nil, fmt.Errorf("rom data size %d exceeds 16 KiB limit for 16K cartridge", len(romData))
		}
		paddedROM := romData
		if len(romData) < 16384 {
			paddedROM = make([]byte, 16384)
			copy(paddedROM, romData)
			for i := len(romData); i < 16384; i++ {
				paddedROM[i] = 0xFF
			}
		}
		c.Chips = []Chip{
			{
				Type:        ChipTypeROM,
				LoadAddress: 0x8000,
				Data:        paddedROM,
			},
		}
	case CRTUltimax:
		if len(romData) > 16384 {
			return nil, fmt.Errorf("rom data size %d exceeds 16 KiB limit for Ultimax cartridge", len(romData))
		}
		// An Ultimax cartridge can be 8 KiB or 16 KiB.
		// If the data is <= 8 KiB, it maps entirely to $E000-$FFFF.
		// If the data is > 8 KiB, the first 8 KiB maps to $8000-$9FFF, and the remaining maps to $E000-$FFFF.
		if len(romData) <= 8192 {
			c.Chips = []Chip{
				{
					Type:        ChipTypeROM,
					LoadAddress: 0xE000,
					Data:        romData,
				},
			}
		} else {
			first8K := romData[:8192]
			second8K := romData[8192:]
			c.Chips = []Chip{
				{
					Type:        ChipTypeROM,
					LoadAddress: 0x8000,
					Data:        first8K,
				},
				{
					Type:        ChipTypeROM,
					LoadAddress: 0xE000,
					Data:        second8K,
				},
			}
		}
	}

	return c.Bytes()
}

// NewXIPCartridge builds an Execute-In-Place (XIP) cartridge from a compiled Program.
//
// In an XIP cartridge, your code runs directly from the cartridge ROM chip itself, 
// rather than being copied into the Commodore 64's RAM. 
//
// Because the C64 CPU executes code directly from the cartridge ROM, you must compile
// your assembly program to target the correct ROM memory addresses:
//
// - For Normal 8K/16K cartridges, the ROM is mapped starting at $8000. The cartridge's 
//   automatic startup code (bootstrap) takes the first 64 bytes ($8000-$803F) as reserved space.
//   Therefore, you must compile your program starting at $8040 (e.g., using '* = $8040' in your assembly).
//
// - For Ultimax cartridges, the ROM is mapped at $E000-$FFFF (and also $8000-$9FFF for 16K).
//   The startup code takes the first 64 bytes ($E000-$E03F), and the system vectors take $FFFA-$FFFF.
//   Therefore, your program must target either $8000-$9FFF or $E040-$FFFA.
//
// If your program is compiled for RAM (like the default $0801 BASIC start), use
// NewRAMCartridge instead, which automatically copies your program to RAM at startup.
func NewXIPCartridge(cartType CartridgeType, name string, prg *Program) ([]byte, error) {
	if prg == nil || prg.Size() == 0 {
		return nil, fmt.Errorf("empty program")
	}

	const bootstrapReservedSize = 64

	var bootProg *Program
	var err error
	var userOffset int
	var romSize int

	switch cartType {
	case CRTNormal8K, CRTNormal16K:
		bootstrap := fmt.Sprintf(`
* = $8000
    .word cold_start
    .word warm_start
    .byte $C3, $C2, $CD, $38, $30 ; "CBM80" signature
cold_start:
warm_start:
    sei
    cld
    jsr $ff84   ; IOINIT: Initialize CIA chips
    jsr $ff87   ; RAMTAS: Initialize RAM/ZP
    jsr $ff8a   ; RESTOR: Restore Kernal vectors
    jsr $ff81   ; CINT: Initialize screen/VIC-II
    cli
    jmp $%04X
`, prg.LoadAddress())

		bootProg, err = Assemble(bootstrap)
		if err != nil {
			return nil, fmt.Errorf("failed to assemble cartridge bootstrap: %w", err)
		}

		if bootProg.Size() > bootstrapReservedSize {
			return nil, fmt.Errorf("bootstrap code size %d exceeds reserved space %d", bootProg.Size(), bootstrapReservedSize)
		}

		firstFree := uint16(0x8000 + bootstrapReservedSize)
		if prg.LoadAddress() < firstFree {
			return nil, fmt.Errorf("program load address $%04X overlaps with bootstrap code (requires at least $%04X)",
				prg.LoadAddress(), firstFree)
		}

		var maxAddr uint16
		if cartType == CRTNormal8K {
			romSize = 8192
			maxAddr = 0xA000
		} else {
			romSize = 16384
			maxAddr = 0xC000
		}

		if uint32(prg.LoadAddress())+uint32(prg.Size()) > uint32(maxAddr) {
			return nil, fmt.Errorf("program size %d at $%04X exceeds cartridge limit $%04X",
				prg.Size(), prg.LoadAddress(), maxAddr)
		}

		userOffset = int(prg.LoadAddress() - 0x8000)

	case CRTUltimax:
		bootstrap := fmt.Sprintf(`
* = $E000
cold_start:
    sei
    cld
    ldx #$ff
    txs
    lda #$1b
    sta $d011
    lda #$08
    sta $d016
    cli
    jmp $%04X
dummy_nmi:
dummy_irq:
    rti
`, prg.LoadAddress())

		bootProg, err = Assemble(bootstrap)
		if err != nil {
			return nil, fmt.Errorf("failed to assemble cartridge bootstrap: %w", err)
		}

		if bootProg.Size() > bootstrapReservedSize {
			return nil, fmt.Errorf("bootstrap code size %d exceeds reserved space %d", bootProg.Size(), bootstrapReservedSize)
		}

		// Ultimax has two valid ROM mapping regions: $8000-$9FFF (16K Ultimax only) and $E000-$FFFA.
		if prg.LoadAddress() >= 0x8000 && prg.LoadAddress() < 0xA000 {
			romSize = 16384
			if uint32(prg.LoadAddress())+uint32(prg.Size()) > 0xA000 {
				return nil, fmt.Errorf("program size %d at $%04X exceeds cartridge limit $A000",
					prg.Size(), prg.LoadAddress())
			}
			userOffset = int(prg.LoadAddress() - 0x8000)
		} else if prg.LoadAddress() >= 0xE000 {
			firstFree := uint16(0xE000 + bootstrapReservedSize)
			if prg.LoadAddress() < firstFree {
				return nil, fmt.Errorf("program load address $%04X overlaps with bootstrap code (requires at least $%04X)",
					prg.LoadAddress(), firstFree)
			}
			if uint32(prg.LoadAddress())+uint32(prg.Size()) > 0xFFFA {
				return nil, fmt.Errorf("program size %d at $%04X overlaps with cartridge vectors (must end before $FFFA)",
					prg.Size(), prg.LoadAddress())
			}
			romSize = 8192
			userOffset = int(prg.LoadAddress() - 0xE000)
		} else {
			return nil, fmt.Errorf("invalid load address $%04X for Ultimax cartridge (must be in $8000-$9FFF or $E040-$FFFA)",
				prg.LoadAddress())
		}

	default:
		return nil, fmt.Errorf("invalid cartridge type for XIP: %v", cartType)
	}

	romData := make([]byte, romSize)
	for i := range romData {
		romData[i] = 0xFF
	}

	if cartType == CRTUltimax {
		// Ultimax vectors at $FFFA-$FFFF:
		// $FFFA-$FFFB: dummy_nmi / dummy_irq address ($E013)
		// $FFFC-$FFFD: cold_start address ($E000)
		// $FFFE-$FFFF: dummy_nmi / dummy_irq address ($E013)
		vectorsBytes := []byte{0x13, 0xE0, 0x00, 0xE0, 0x13, 0xE0}

		if romSize == 16384 {
			// For 16K Ultimax, the bootstrap is at the start of the second 8K bank.
			copy(romData[8192:], bootProg.Code())
			copy(romData[8192+0x1FFA:], vectorsBytes)
		} else {
			copy(romData, bootProg.Code())
			copy(romData[0x1FFA:], vectorsBytes)
		}
	} else {
		copy(romData, bootProg.Code())
	}

	copy(romData[userOffset:], prg.Code())

	return NewRawCartridge(cartType, name, romData)
}

// parseSYSEntryPoint parses the SYS address from the first BASIC line if present.
func parseSYSEntryPoint(code []byte) (uint16, bool) {
	if len(code) < 6 {
		return 0, false
	}
	nextPtr := uint16(code[0]) | uint16(code[1])<<8
	if nextPtr == 0 {
		return 0, false
	}
	// Walk the first line body starting at offset 4.
	for i := 4; i < len(code) && code[i] != 0; i++ {
		if code[i] == 0x9E { // SYS token
			var addr uint16
			foundDigit := false
			for j := i + 1; j < len(code) && code[j] != 0; j++ {
				b := code[j]
				if b == ' ' {
					continue
				}
				if b >= '0' && b <= '9' {
					addr = addr*10 + uint16(b-'0')
					foundDigit = true
				} else {
					break
				}
			}
			if foundDigit {
				return addr, true
			}
		}
	}
	return 0, false
}

// hasBASICHeader checks if the code starts with a valid C64 BASIC line structure.
// A C64 BASIC program line starts with:
// - next line pointer (2 bytes, non-zero, e.g. pointing to next line at $080c)
// - line number (2 bytes, e.g. 10)
// - line body (terminated by a null byte)
func hasBASICHeader(code []byte) bool {
	// A C64 BASIC program line requires at least:
	// - 2 bytes: pointer to the next BASIC line (nextPtr)
	// - 2 bytes: BASIC line number
	// - 1 byte: BASIC line content (e.g., token or character)
	// - 1 byte: null terminator (end of line)
	if len(code) < 6 {
		return false
	}

	// Read the 16-bit little-endian next-line pointer.
	nextPtr := uint16(code[0]) | uint16(code[1])<<8

	// The standard start of BASIC RAM is $0801. The nextPtr points to the memory
	// address of the next BASIC line (just after the current line's null terminator).
	// A standard, short '10 SYS...' header ends well before $0900. If nextPtr is outside
	// this reasonable range, it's highly likely to be raw machine code instructions instead.
	if nextPtr < 0x0805 || nextPtr > 0x0900 {
		return false
	}

	// Calculate where nextPtr lies relative to the program start ($0801).
	offsetOfNextLine := int(nextPtr - 0x0801)

	// In a valid BASIC line, the byte immediately preceding the next line
	// pointer must be a null byte (0x00) that terminates the current line.
	terminatorOffset := offsetOfNextLine - 1
	if terminatorOffset >= len(code) || code[terminatorOffset] != 0 {
		return false
	}

	return true
}

// NewRAMCartridge builds a cartridge that relocates a standard PRG program into RAM at startup.
// By default, if the program's load address is $0801 (standard BASIC start), the entry point is
// parsed from the BASIC SYS header, defaulting to $080d. Otherwise, the entry point defaults to the load address.
// You can optionally pass a custom entry point address as the last argument.
func NewRAMCartridge(cartType CartridgeType, name string, prg *Program, entryPoint ...uint16) ([]byte, error) {
	if prg == nil || prg.Size() == 0 {
		return nil, fmt.Errorf("empty program")
	}
	if cartType == CRTUltimax {
		return nil, fmt.Errorf("ultimax cartridge is not supported for RAM relocation")
	}

	ramDest := prg.LoadAddress()
	entry := ramDest

	// If the program is loaded at $0801 (the standard start of C64 BASIC RAM),
	// we need to determine if it has a BASIC header block.
	if ramDest == 0x0801 {
		if parsedAddr, ok := parseSYSEntryPoint(prg.Code()); ok {
			// A valid 'SYS <address>' header was found, so jump to that address.
			entry = parsedAddr
		} else if hasBASICHeader(prg.Code()) {
			// It has a BASIC header, but we couldn't parse a specific SYS address.
			// Fall back to $080D, which is the standard machine code start address
			// immediately following a default '10 SYS 2061' header.
			entry = uint16(0x080D)
		}
		// If it has no BASIC header at all, it's a raw machine-code program
		// loaded at $0801, so we keep entry = $0801.
	}
	if len(entryPoint) > 0 {
		entry = entryPoint[0]
	}

	progLen := prg.Size()
	numPages := progLen / 256
	remBytes := progLen % 256

	// Generate bootstrap assembly using the indirect zero-page pointer copy loop.
	// Zero-page locations $fb-$fe are standard temporary pointers on C64.
	bootstrapAsm := fmt.Sprintf(`
* = $8000
    .word cold_start
    .word warm_start
    .byte $C3, $C2, $CD, $38, $30 ; "CBM80" signature

cold_start:
warm_start:
    sei
    cld
    jsr $ff84   ; IOINIT: Initialize CIA chips
    jsr $ff87   ; RAMTAS: Initialize RAM/ZP
    jsr $ff8a   ; RESTOR: Restore Kernal vectors
    jsr $ff81   ; CINT: Initialize screen/VIC-II

    lda #<rom_source
    sta $fb
    lda #>rom_source
    sta $fc
    
    lda #<$%04X ; RAM destination
    sta $fd
    lda #>$%04X
    sta $fe

    ldy #0
    ldx #%d      ; Number of pages
    beq copy_rem
copy_page_loop:
    lda ($fb),y
    sta ($fd),y
    iny
    bne copy_page_loop
    inc $fc
    inc $fe
    dex
    bne copy_page_loop

copy_rem:
    ldy #0
copy_rem_loop:
    cpy #%d      ; Remaining bytes
    beq done
    lda ($fb),y
    sta ($fd),y
    iny
    bne copy_rem_loop

done:
    lda #$7f     ; Clear CIA interrupts
    sta $dc0d
    sta $dd0d
    cli
    jmp $%04X   ; Entry point

rom_source:
`, ramDest, ramDest, numPages, remBytes, entry)

	// Append program bytes to the assembly
	var sb strings.Builder
	sb.WriteString(bootstrapAsm)
	code := prg.Code()
	for i, b := range code {
		if i > 0 {
			if i%16 == 0 {
				sb.WriteString("\n    .byte ")
			} else {
				sb.WriteByte(',')
			}
		} else {
			sb.WriteString("    .byte ")
		}
		fmt.Fprintf(&sb, "$%02x", b)
	}
	sb.WriteByte('\n')

	prog, err := Assemble(sb.String())
	if err != nil {
		return nil, fmt.Errorf("failed to assemble relocation bootstrap: %w", err)
	}

	return NewRawCartridge(cartType, name, prog.Code())
}

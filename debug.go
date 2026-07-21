// Decode live C64 state from RAM: screen, BASIC, chips, sprites, bitmaps.

package ultimate

import (
	"context"
	"fmt"
	"image"

	"github.com/c64uploader/go-ultimate/c64"
)

// DebugService reads C64 RAM and decodes it into structured hardware views.
// DebugService reads C64 RAM and decodes it into structured hardware views.
// Mem must be set before calling any DebugService method.
type DebugService struct {
	Mem MemoryReaderWriter // memory backend; defaults to Machine
}

func (d *DebugService) read(ctx context.Context, addr uint16, n int) ([]byte, error) {
	return d.Mem.ReadMemory(ctx, addr, n)
}

// Screen reads screen memory and returns 25 rows of decoded text.
// The base address comes from the VIC-II memory-setup register and CIA2 bank select.
// Returns an error if the VIC is not in a text mode (bitmap mode is on).
func (d *DebugService) Screen(ctx context.Context) (*c64.Screen, error) {
	// Read $D000–$DD01 in one block to get VIC registers, Color RAM, and CIA2.
	ioData, err := d.read(ctx, 0xD000, 3330)
	if err != nil {
		return nil, fmt.Errorf("read C64 I/O area $D000-$DD01: %w", err)
	}

	if ioData[0x11]&0x20 != 0 {
		return nil, fmt.Errorf("machine is not in text mode (bitmap mode is active)")
	}

	vicD018 := ioData[0x18]
	cia2DD00 := ioData[0xD00]
	colorData := ioData[0x800 : 0x800+1000]

	bankBase := (3 - int(cia2DD00&3)) * 0x4000
	screenOffset := int((vicD018>>4)&0x0F) * 0x0400
	screenAddr := uint16(bankBase + screenOffset)

	screenData, err := d.read(ctx, screenAddr, 1000)
	if err != nil {
		return nil, fmt.Errorf("read screen memory $%04X: %w", screenAddr, err)
	}

	cs := c64.CharsetUppercase
	if vicD018&0x08 != 0 {
		cs = c64.CharsetLowercase
	}

	rows := make([]string, 25)
	for r := range 25 {
		rows[r] = c64.DecodeScreen(screenData[r*40:r*40+40], cs)
	}
	return &c64.Screen{Rows: rows, RawScreen: screenData, RawColor: colorData, Charset: cs}, nil
}

// BASIC reads tokenized program memory from $0801 and decodes it to source lines.
func (d *DebugService) BASIC(ctx context.Context) ([]c64.BASICLine, error) {
	data, err := d.read(ctx, 0x0801, 4096)
	if err != nil {
		return nil, err
	}
	return c64.DecodeBASICProgram(data), nil
}

// ScreenMode returns the current VIC-II display mode.
func (d *DebugService) ScreenMode(ctx context.Context) (c64.ScreenMode, error) {
	regs, err := d.read(ctx, 0xD011, 6)
	if err != nil {
		return 0, fmt.Errorf("read screen mode registers: %w", err)
	}
	d011 := regs[0]
	d016 := regs[5]

	ecm := d011&0x40 != 0 // bit 6: extended color mode
	bmm := d011&0x20 != 0 // bit 5: bitmap mode
	mcm := d016&0x10 != 0 // bit 4: multicolor mode

	switch {
	case ecm && !bmm:
		return c64.ScreenExtendedColor, nil
	case bmm && !mcm:
		return c64.ScreenBitmap, nil
	case bmm && mcm:
		return c64.ScreenMulticolorBitmap, nil
	case !bmm && mcm:
		return c64.ScreenMulticolorText, nil
	default:
		return c64.ScreenText, nil
	}
}

// Charset reads the active character generator dot data (2048 bytes, 256 characters × 8 bytes).
// The address is determined from $D018 bits 1–3.
func (d *DebugService) Charset(ctx context.Context) (*c64.Charset, error) {
	vicCtrl, err := d.read(ctx, 0xD018, 1)
	if err != nil {
		return nil, fmt.Errorf("read $D018: %w", err)
	}
	bank := (vicCtrl[0] >> 1) & 0x07
	addr := uint16(bank) * 0x0800
	data, err := d.read(ctx, addr, 2048)
	if err != nil {
		return nil, fmt.Errorf("read charset at $%04X: %w", addr, err)
	}
	return &c64.Charset{Raw: data}, nil
}

// Sprites reads position, flags, and sprite data for all eight hardware sprites.
func (d *DebugService) Sprites(ctx context.Context) (c64.Sprites, error) {
	ptrs, err := d.read(ctx, 0x07F8, 8)
	if err != nil {
		return nil, err
	}
	vic, err := d.read(ctx, 0xD000, 47)
	if err != nil {
		return nil, err
	}
	cia2, err := d.read(ctx, 0xDD00, 1)
	if err != nil {
		return nil, fmt.Errorf("read CIA2: %w", err)
	}

	bankBase := (3 - int(cia2[0]&3)) * 0x4000
	enabled := vic[0x15]
	mc0 := int(vic[0x25]) & 0x0F
	mc1 := int(vic[0x26]) & 0x0F
	out := make(c64.Sprites, 8)
	for i := range 8 {
		x := int(vic[i*2]) | int(vic[0x10]>>i&1)<<8
		y := int(vic[i*2+1])
		ptr := int(ptrs[i])
		addr := uint16(bankBase + ptr*64)

		raw, err := d.read(ctx, addr, 64)
		if err != nil {
			return nil, fmt.Errorf("read sprite %d sprite data: %w", i, err)
		}

		out[i] = c64.Sprite{
			Number:      i,
			Enabled:     enabled&(1<<i) != 0,
			X:           x,
			Y:           y,
			Color:       int(vic[0x27+i]) & 0x0F,
			Multicolor:  vic[0x1C]&(1<<i) != 0,
			XExpand:     vic[0x1D]&(1<<i) != 0,
			YExpand:     vic[0x17]&(1<<i) != 0,
			Multicolor0: mc0,
			Multicolor1: mc1,
			Raw:         raw,
		}
	}
	return out, nil
}

// Bitmap decodes the current VIC bitmap screen into a 320×200 image.
// Returns an error if the VIC is not in bitmap mode.
func (d *DebugService) Bitmap(ctx context.Context) (*image.Paletted, error) {
	vicCtrl, err := d.read(ctx, 0xD011, 8)
	if err != nil {
		return nil, fmt.Errorf("read VIC registers: %w", err)
	}
	d011 := vicCtrl[0]
	d016 := vicCtrl[5]
	d018 := vicCtrl[7]

	if (d011 & 0x20) == 0 {
		return nil, fmt.Errorf("machine is not in bitmap mode")
	}
	isMulticolor := (d016 & 0x10) != 0

	cia2, err := d.read(ctx, 0xDD00, 1)
	if err != nil {
		return nil, fmt.Errorf("read CIA2 $DD00: %w", err)
	}

	bankBase := (3 - int(cia2[0]&3)) * 0x4000
	bitmapAddr := uint16(bankBase + int((d018>>3)&1)*0x2000)
	screenAddr := uint16(bankBase + int((d018>>4)&0x0F)*0x0400)

	bgColorData, err := d.read(ctx, 0xD021, 1)
	if err != nil {
		return nil, fmt.Errorf("read background color: %w", err)
	}
	bgColor := bgColorData[0] & 0x0F

	bitmapData, err := d.read(ctx, bitmapAddr, 8000)
	if err != nil {
		return nil, err
	}
	screenColors, err := d.read(ctx, screenAddr, 1000)
	if err != nil {
		return nil, err
	}
	colorRAM, err := d.read(ctx, 0xD800, 1000)
	if err != nil {
		return nil, err
	}

	return c64.DecodeBitmap(bitmapData, screenColors, colorRAM, bgColor, isMulticolor), nil
}





// Memory decoders for BASIC, chips, sprites, and bitmaps.

package c64

import (
	"fmt"
	"image"
	"image/color"
	"strings"
)

// BASICLine is one detokenized line from a C64 BASIC program.
type BASICLine struct {
	Number  int    `json:"number"`
	Content string `json:"content"`
}

var basicTokens = map[byte]string{
	0x80: "END", 0x81: "FOR", 0x82: "NEXT", 0x83: "DATA",
	0x84: "INPUT#", 0x85: "INPUT", 0x86: "DIM", 0x87: "READ",
	0x88: "LET", 0x89: "GOTO", 0x8A: "RUN", 0x8B: "IF",
	0x8C: "RESTORE", 0x8D: "GOSUB", 0x8E: "RETURN", 0x8F: "REM",
	0x90: "STOP", 0x91: "ON", 0x92: "WAIT", 0x93: "LOAD",
	0x94: "SAVE", 0x95: "VERIFY", 0x96: "DEF", 0x97: "POKE",
	0x98: "PRINT#", 0x99: "PRINT", 0x9A: "CONT", 0x9B: "LIST",
	0x9C: "CLR", 0x9D: "CMD", 0x9E: "SYS", 0x9F: "OPEN",
	0xA0: "CLOSE", 0xA1: "GET", 0xA2: "NEW", 0xA3: "TAB(",
	0xA4: "TO", 0xA5: "FN", 0xA6: "SPC(", 0xA7: "THEN",
	0xA8: "NOT", 0xA9: "STEP", 0xAA: "+", 0xAB: "-",
	0xAC: "*", 0xAD: "/", 0xAE: "^", 0xAF: "AND",
	0xB0: "OR", 0xB1: ">", 0xB2: "=", 0xB3: "<",
	0xB4: "SGN", 0xB5: "INT", 0xB6: "ABS", 0xB7: "USR",
	0xB8: "FRE", 0xB9: "POS", 0xBA: "SQR", 0xBB: "RND",
	0xBC: "LOG", 0xBD: "EXP", 0xBE: "COS", 0xBF: "SIN",
	0xC0: "TAN", 0xC1: "ATN", 0xC2: "PEEK", 0xC3: "LEN",
	0xC4: "STR$", 0xC5: "VAL", 0xC6: "ASC", 0xC7: "CHR$",
	0xC8: "LEFT$", 0xC9: "RIGHT$", 0xCA: "MID$", 0xCB: "GO",
}

// DecodeBASICLine detokenizes one BASIC line body (without the line-link header).
func DecodeBASICLine(data []byte) string {
	var b strings.Builder
	inQuotes := false
	for _, c := range data {
		switch {
		case c == '"':
			inQuotes = !inQuotes
			b.WriteByte(c)
		case inQuotes:
			if c >= 32 && c < 127 {
				b.WriteByte(c)
			} else {
				fmt.Fprintf(&b, "«%02X»", c)
			}
		case c >= 0x80:
			if tok, ok := basicTokens[c]; ok {
				b.WriteString(tok)
			} else {
				fmt.Fprintf(&b, "«%02X»", c)
			}
		case c >= 32 && c < 127:
			b.WriteByte(c)
		default:
			// control characters: skip
		}
	}
	return b.String()
}

// DecodeBASICProgram walks the linked line list from $0801 and returns source lines.
func DecodeBASICProgram(data []byte) []BASICLine {
	var lines []BASICLine
	pos := 0
	for pos < len(data)-4 {
		nextPtr := int(data[pos]) | int(data[pos+1])<<8
		if nextPtr == 0 {
			break
		}
		lineNum := int(data[pos+2]) | int(data[pos+3])<<8

		end := pos + 4
		for end < len(data) && data[end] != 0 {
			end++
		}
		if end > len(data) {
			end = len(data)
		}

		var chunk []byte
		if pos+4 < end {
			chunk = data[pos+4 : end]
		}
		lines = append(lines, BASICLine{
			Number:  lineNum,
			Content: DecodeBASICLine(chunk),
		})

		nextOffset := nextPtr - 0x0801
		if nextOffset <= pos || nextOffset >= len(data) {
			break
		}
		pos = nextOffset
	}
	return lines
}

// colorNames lists the 16 standard C64 color names in index order.
var colorNames = []string{
	"BLACK", "WHITE", "RED", "CYAN",
	"PURPLE", "GREEN", "BLUE", "YELLOW",
	"ORANGE", "BROWN", "PINK", "DARK GREY",
	"GREY", "LIGHT GREEN", "LIGHT BLUE", "LIGHT GREY",
}

// ColorNames returns the 16 VIC palette color names (index 0–15).
func ColorNames() []string {
	return append([]string(nil), colorNames...)
}

// ColorName returns the palette name for index, or "" if out of range.
func ColorName(index int) string {
	if index < 0 || index >= len(colorNames) {
		return ""
	}
	return colorNames[index]
}

// Screen is decoded 25×40 text plus raw screen and color RAM bytes.
type Screen struct {
	Rows      []string `json:"screen"` // 25 decoded text rows
	RawScreen []byte   `json:"-"`      // 1000 screen-code bytes
	RawColor  []byte   `json:"-"`      // 1000 color-RAM bytes
}

// ScreenMode is the VIC-II display mode ($D011 bits 5-6, $D016 bit 4).
type ScreenMode int

const (
	ScreenText             ScreenMode = iota // standard character mode
	ScreenMulticolorText                     // multicolor character mode
	ScreenBitmap                             // standard bitmap mode
	ScreenMulticolorBitmap                   // multicolor bitmap mode
	ScreenExtendedColor                      // extended background color mode
)

// String returns the mode name used in VIC-II documentation.
func (m ScreenMode) String() string {
	names := [...]string{
		"screen text",
		"screen multicolor text",
		"screen bitmap",
		"screen multicolor bitmap",
		"screen extended color",
	}
	if int(m) < len(names) {
		return names[m]
	}
	return fmt.Sprintf("unknown screen mode %d", m)
}

// Charset holds character generator dot data (2048 bytes, 256 characters × 8 bytes each).
type Charset struct {
	Raw []byte `json:"-"` // 2048 bytes of character dot patterns
}

// Character returns the 8-byte dot pattern for a single character (index 0–255).
func (cs *Charset) Character(index int) ([]byte, error) {
	if len(cs.Raw) < 2048 {
		return nil, fmt.Errorf("c64: charset data too short: %d bytes, need at least 2048", len(cs.Raw))
	}
	if index < 0 || index >= 256 {
		return nil, fmt.Errorf("c64: character index %d out of range (0–255)", index)
	}
	offset := index * 8
	return cs.Raw[offset : offset+8], nil
}

// CharacterImage renders a single character as an 8×8 paletted image
// using the given foreground and background colors from the VIC palette.
func (cs *Charset) CharacterImage(index int, fg, bg Color) (*image.Paletted, error) {
	dots, err := cs.Character(index)
	if err != nil {
		return nil, err
	}
	charPalette := color.Palette{
		palette[bg&0x0F],
		palette[fg&0x0F],
	}
	img := image.NewPaletted(image.Rect(0, 0, 8, 8), charPalette)
	for y := range 8 {
		for x := range 8 {
			if dots[y]&(1<<uint(7-x)) != 0 {
				img.SetColorIndex(x, y, 1) // foreground
			} else {
				img.SetColorIndex(x, y, 0) // background
			}
		}
	}
	return img, nil
}

// ImageMap renders all 256 characters in a 32-column × 8-row grid (256×64 pixels).
func (cs *Charset) ImageMap(fg, bg Color) *image.Paletted {
	charPalette := color.Palette{
		palette[bg&0x0F],
		palette[fg&0x0F],
	}
	img := image.NewPaletted(image.Rect(0, 0, 256, 64), charPalette)
	for i := range 256 {
		if len(cs.Raw) < i*8+8 {
			break
		}
		dots := cs.Raw[i*8 : i*8+8]
		col := (i % 32) * 8
		row := (i / 32) * 8
		for y := range 8 {
			for x := range 8 {
				if dots[y]&(1<<uint(7-x)) != 0 {
					img.SetColorIndex(col+x, row+y, 1)
				} else {
					img.SetColorIndex(col+x, row+y, 0)
				}
			}
		}
	}
	return img
}

// Sprite is one of the eight VIC-II hardware sprites.
//
// Multicolor0 and Multicolor1 are shared VIC-II registers ($D025/$D026),
// not per-sprite values. They are stored here so that Image() is
// self-contained.
type Sprite struct {
	Number      int    `json:"number"`
	Enabled     bool   `json:"enabled"`
	X           int    `json:"x"`
	Y           int    `json:"y"`
	Color       int    `json:"color"`
	Multicolor  bool   `json:"multicolor,omitempty"`
	XExpand     bool   `json:"x_expand,omitempty"`
	YExpand     bool   `json:"y_expand,omitempty"`
	Multicolor0 int    `json:"multicolor_0,omitempty"`
	Multicolor1 int    `json:"multicolor_1,omitempty"`
	Raw         []byte `json:"-"`
}

// Sprites is a set of eight VIC-II hardware sprites.
type Sprites []Sprite

// String returns a compact representation of the sprite for logging and debugging.
func (s Sprite) String() string {
	if !s.Enabled {
		return fmt.Sprintf("sprite %d (disabled)", s.Number)
	}
	return fmt.Sprintf("sprite %d @ (%d,%d) color=%s",
		s.Number, s.X, s.Y, ColorName(s.Color))
}

// Image renders the 24×21 sprite data (48×42 when expanded) as a paletted image.
// Returns an error if the sprite data is too short (need at least 63 bytes).
func (s *Sprite) Image() (*image.Paletted, error) {
	if len(s.Raw) < 63 {
		return nil, fmt.Errorf("c64: sprite %d: sprite data too short: %d bytes, need at least 63", s.Number, len(s.Raw))
	}

	// Index 0: Transparent
	// Index 1: Main Sprite Color
	// Index 2: Sprite Multicolor 0
	// Index 3: Sprite Multicolor 1
	spritePalette := color.Palette{
		color.RGBA{0, 0, 0, 0},
		palette[s.Color&0x0F].(color.RGBA),
		palette[s.Multicolor0&0x0F].(color.RGBA),
		palette[s.Multicolor1&0x0F].(color.RGBA),
	}

	width, height := 24, 21
	if s.XExpand {
		width = 48
	}
	if s.YExpand {
		height = 42
	}

	img := image.NewPaletted(image.Rect(0, 0, width, height), spritePalette)

	for y := 0; y < height; y++ {
		spriteY := y
		if s.YExpand {
			spriteY = y / 2
		}

		rowOffset := spriteY * 3
		rowBits := uint32(s.Raw[rowOffset])<<16 | uint32(s.Raw[rowOffset+1])<<8 | uint32(s.Raw[rowOffset+2])

		for x := 0; x < width; x++ {
			spriteX := x
			if s.XExpand {
				spriteX = x / 2
			}

			var colorIdx byte

			if s.Multicolor {
				bitCol := spriteX &^ 1
				shift := 22 - bitCol
				bitPair := (rowBits >> shift) & 3

				switch bitPair {
				case 0:
					colorIdx = 0
				case 1:
					colorIdx = 2
				case 2:
					colorIdx = 1
				case 3:
					colorIdx = 3
				}
			} else {
				shift := 23 - spriteX
				bit := (rowBits >> shift) & 1

				if bit == 1 {
					colorIdx = 1
				} else {
					colorIdx = 0
				}
			}

			img.SetColorIndex(x, y, colorIdx)
		}
	}

	return img, nil
}

// Color is a VIC-II color index (0–15).
type Color byte

const (
	ColorBlack      Color = 0
	ColorWhite      Color = 1
	ColorRed        Color = 2
	ColorCyan       Color = 3
	ColorPurple     Color = 4
	ColorGreen      Color = 5
	ColorBlue       Color = 6
	ColorYellow     Color = 7
	ColorOrange     Color = 8
	ColorBrown      Color = 9
	ColorPink       Color = 10
	ColorDarkGrey   Color = 11
	ColorGrey       Color = 12
	ColorLightGreen Color = 13
	ColorLightBlue  Color = 14
	ColorLightGrey  Color = 15
)

// Name returns the human-readable name for this color index.
func (c Color) Name() string {
	if int(c) < len(colorNames) {
		return colorNames[c]
	}
	return ""
}

var palette = color.Palette{
	color.RGBA{0x00, 0x00, 0x00, 0xFF}, // 0: Black
	color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}, // 1: White
	color.RGBA{0x88, 0x00, 0x00, 0xFF}, // 2: Red
	color.RGBA{0xAA, 0xFF, 0xEE, 0xFF}, // 3: Cyan
	color.RGBA{0xCC, 0x44, 0xCC, 0xFF}, // 4: Purple
	color.RGBA{0x00, 0xCC, 0x55, 0xFF}, // 5: Green
	color.RGBA{0x00, 0x00, 0xAA, 0xFF}, // 6: Blue
	color.RGBA{0xEE, 0xEE, 0x77, 0xFF}, // 7: Yellow
	color.RGBA{0xDD, 0x88, 0x55, 0xFF}, // 8: Orange
	color.RGBA{0x66, 0x44, 0x00, 0xFF}, // 9: Brown
	color.RGBA{0xFF, 0x77, 0x77, 0xFF}, // 10: Pink (Light Red)
	color.RGBA{0x33, 0x33, 0x33, 0xFF}, // 11: Dark Grey
	color.RGBA{0x77, 0x77, 0x77, 0xFF}, // 12: Grey
	color.RGBA{0xAA, 0xFF, 0x66, 0xFF}, // 13: Light Green
	color.RGBA{0x00, 0x88, 0xFF, 0xFF}, // 14: Light Blue
	color.RGBA{0xBB, 0xBB, 0xBB, 0xFF}, // 15: Light Grey
}

// Palette returns the standard 16-color VIC palette as RGBA values.
func Palette() color.Palette {
	return append(color.Palette(nil), palette...)
}

// encodeImageRaw converts a 24×21 paletted image to 63 sprite bytes (standard mode).
// Palette: index 0 = transparent, non-zero = opaque.
func encodeImageRaw(img *image.Paletted) ([]byte, error) {
	b := img.Bounds()
	if b.Dx() != 24 || b.Dy() != 21 {
		return nil, fmt.Errorf("c64: sprite image must be 24×21, got %d×%d", b.Dx(), b.Dy())
	}
	raw := make([]byte, 64)
	for y := range 21 {
		rowOffset := y * 3
		var rowBits uint32
		for x := range 24 {
			ci := img.ColorIndexAt(x, y)
			if ci != 0 {
				shift := 23 - x
				rowBits |= uint32(1) << shift
			}
		}
		raw[rowOffset] = byte(rowBits >> 16)
		raw[rowOffset+1] = byte(rowBits >> 8)
		raw[rowOffset+2] = byte(rowBits)
	}
	return raw, nil
}

// encodeMulticolorImageRaw converts a 12×21 paletted image to 63 sprite bytes (multicolor mode).
// Each pixel is a 2-bit index: 0=transparent, 1=multicolor0, 2=main color, 3=multicolor1.
func encodeMulticolorImageRaw(img *image.Paletted) ([]byte, error) {
	b := img.Bounds()
	if b.Dx() != 12 || b.Dy() != 21 {
		return nil, fmt.Errorf("c64: multicolor sprite image must be 12×21, got %d×%d", b.Dx(), b.Dy())
	}
	raw := make([]byte, 64)
	for y := range 21 {
		rowOffset := y * 3
		var rowBits uint32
		for x := range 12 {
			ci := img.ColorIndexAt(x, y)
			shift := uint(22 - x*2)
			rowBits |= uint32(ci&3) << shift
		}
		raw[rowOffset] = byte(rowBits >> 16)
		raw[rowOffset+1] = byte(rowBits >> 8)
		raw[rowOffset+2] = byte(rowBits)
	}
	return raw, nil
}

// NewSpriteFromImage creates a Sprite with Raw set from a 24×21 paletted image
// in standard sprite mode. The returned Sprite has only Raw populated;
// caller should set Color, Multicolor0/1, etc. as needed.
// Palette: index 0 = transparent, non-zero = opaque (main color).
func NewSpriteFromImage(img *image.Paletted) (*Sprite, error) {
	raw, err := encodeImageRaw(img)
	if err != nil {
		return nil, err
	}
	return &Sprite{Raw: raw}, nil
}

// NewSpriteFromMulticolorImage creates a Sprite with Raw set from a 12×21
// paletted image in multicolor sprite mode. Each pixel is a 2-bit index:
// 0=transparent, 1=multicolor0, 2=main color, 3=multicolor1.
func NewSpriteFromMulticolorImage(img *image.Paletted) (*Sprite, error) {
	raw, err := encodeMulticolorImageRaw(img)
	if err != nil {
		return nil, err
	}
	return &Sprite{Raw: raw}, nil
}

// DecodeBitmap builds a 320×200 image from bitmap, screen-color, and color-RAM data.
// Set isMulticolor for the VIC multicolor bitmap layout.
func DecodeBitmap(bitmapData, screenColors, colorRAM []byte, bgColor byte, isMulticolor bool) *image.Paletted {
	img := image.NewPaletted(image.Rect(0, 0, 320, 200), palette)

	for y := range 200 {
		cy := y / 8
		py := y % 8
		for x := range 320 {
			cx := x / 8
			charIdx := cy*40 + cx
			byteAddr := charIdx*8 + py

			b := bitmapData[byteAddr]
			var colorIdx byte

			if isMulticolor {
				// Multicolor: pixels are 2 bits wide
				shift := 6 - ((x % 8) & 0xFE)
				bitPair := (b >> shift) & 3

				switch bitPair {
				case 0:
					colorIdx = bgColor
				case 1:
					colorIdx = screenColors[charIdx] >> 4
				case 2:
					colorIdx = screenColors[charIdx] & 0x0F
				case 3:
					colorIdx = colorRAM[charIdx] & 0x0F
				}
			} else {
				// Standard high-res: 1 bit per pixel
				shift := 7 - (x % 8)
				bit := (b >> shift) & 1

				if bit == 1 {
					colorIdx = screenColors[charIdx] >> 4
				} else {
					colorIdx = screenColors[charIdx] & 0x0F
				}
			}

			img.SetColorIndex(x, y, colorIdx)
		}
	}

	return img
}

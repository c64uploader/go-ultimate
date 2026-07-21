package c64

import (
	"image"
	"image/color"
	"reflect"
	"testing"
)

func TestEncodeScreen(t *testing.T) {
	got := EncodeScreen("screen read ok")
	want := []byte{0x13, 0x03, 0x12, 0x05, 0x05, 0x0e, 0x20, 0x12, 0x05, 0x01, 0x04, 0x20, 0x0f, 0x0b}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("EncodeScreen = %v, want %v", got, want)
	}
	// EncodeScreen produces codes 1-26. In uppercase charset these are A-Z.
	if text := DecodeScreen(got, CharsetUppercase); text != "SCREEN READ OK" {
		t.Fatalf("roundtrip uppercase = %q, want SCREEN READ OK", text)
	}
	// In lowercase charset the same codes are a-z.
	if text := DecodeScreen(got, CharsetLowercase); text != "screen read ok" {
		t.Fatalf("roundtrip lowercase = %q, want screen read ok", text)
	}
}

func TestDecodeScreen(t *testing.T) {
	tests := []struct {
		in   []byte
		cs   CharsetMode
		want string
	}{
		// Codes 1-26: letters (case depends on charset)
		{[]byte{1, 2, 3}, CharsetUppercase, "ABC"},
		{[]byte{1, 2, 3}, CharsetLowercase, "abc"},
		// Codes 65-90: A-Z in lowercase charset, graphics in uppercase
		{[]byte{65, 66, 67}, CharsetLowercase, "ABC"},
		{[]byte{65, 66, 67}, CharsetUppercase, "???"},
		// Punctuation and digits: charset-independent
		{[]byte{32, 33, 34}, CharsetUppercase, " !\""},
		{[]byte{32, 33, 34}, CharsetLowercase, " !\""},
		{[]byte{0}, CharsetUppercase, "@"},
		{[]byte{91}, CharsetUppercase, ":"},
	}
	for _, tc := range tests {
		if got := DecodeScreen(tc.in, tc.cs); got != tc.want {
			t.Errorf("DecodeScreen(%v, %d) = %q, want %q", tc.in, tc.cs, got, tc.want)
		}
	}
}

func TestDecodeBASICLine(t *testing.T) {
	tests := []struct {
		in   []byte
		want string
	}{
		{[]byte{0x99, 0x20, 0x22, 'H', 'I', 0x22}, "PRINT \"HI\""},
		{[]byte{0x89, 0x20, '1', '0'}, "GOTO 10"},
	}
	for _, tc := range tests {
		if got := DecodeBASICLine(tc.in); got != tc.want {
			t.Errorf("DecodeBASICLine(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestDecodeBASICProgram(t *testing.T) {
	// Program at $0801. Line links are absolute addresses; offsets below
	// account for the $0801 base.
	data := []byte{
		// Line 10 @ offset 0: link $080C (offset 11)
		0x0C, 0x08,
		0x0A, 0x00, // line 10
		0x99, 0x20, 0x22, 'H', 'I', 0x22, 0x00, // PRINT "HI" + term
		// Line 20 @ offset 11: link $0815 (offset 20)
		0x15, 0x08,
		0x14, 0x00, // line 20
		0x89, ' ', '1', '0', 0x00, // GOTO 10 + term
		// End @ offset 20
		0x00, 0x00,
	}
	want := []BASICLine{
		{Number: 10, Content: "PRINT \"HI\""},
		{Number: 20, Content: "GOTO 10"},
	}
	got := DecodeBASICProgram(data)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestColorNames(t *testing.T) {
	if ColorName(2) != "RED" {
		t.Fatalf("ColorName(2) = %q", ColorName(2))
	}
}

func TestDecodeBitmap(t *testing.T) {
	bitmapData := make([]byte, 8000)
	screenColors := make([]byte, 1000)
	colorRAM := make([]byte, 1000)

	// Set some test bits
	// Byte 0: top-left of the screen (x=0..7, y=0)
	bitmapData[0] = 0b10100000 // Standard: pixel 0=fg, pixel 1=bg, pixel 2=fg, others=bg
	screenColors[0] = 0x25     // Foreground = 2 (Red), Background = 5 (Green)

	// Test Standard Mode
	img := DecodeBitmap(bitmapData, screenColors, colorRAM, 0, false)
	if img.Bounds() != image.Rect(0, 0, 320, 200) {
		t.Fatalf("unexpected bounds: %v", img.Bounds())
	}
	// x=0: fg (Red)
	if got := img.ColorIndexAt(0, 0); got != 2 {
		t.Errorf("pixel (0,0) = %d, want 2", got)
	}
	// x=1: bg (Green)
	if got := img.ColorIndexAt(1, 0); got != 5 {
		t.Errorf("pixel (1,0) = %d, want 5", got)
	}
	// x=2: fg (Red)
	if got := img.ColorIndexAt(2, 0); got != 2 {
		t.Errorf("pixel (2,0) = %d, want 2", got)
	}
	// x=3: bg (Green)
	if got := img.ColorIndexAt(3, 0); got != 5 {
		t.Errorf("pixel (3,0) = %d, want 5", got)
	}

	// Test Multicolor Mode
	// Byte 0: top-left of the screen (x=0..7, y=0)
	// In multicolor, each pair of bits is one double-wide pixel.
	// bits 7-6: 01 (colorIdx from upper nibble of screenColors = 2)
	// bits 5-4: 10 (colorIdx from lower nibble of screenColors = 5)
	// bits 3-2: 11 (colorIdx from colorRAM = 7)
	// bits 1-0: 00 (colorIdx from bgColor = 9)
	bitmapData[0] = 0b01101100
	screenColors[0] = 0x25
	colorRAM[0] = 0x07
	bgColor := byte(9)

	imgMC := DecodeBitmap(bitmapData, screenColors, colorRAM, bgColor, true)

	// x=0,1 (bits 7-6) -> 01 -> screenColors upper nibble = 2 (Red)
	if got := imgMC.ColorIndexAt(0, 0); got != 2 {
		t.Errorf("MC pixel (0,0) = %d, want 2", got)
	}
	if got := imgMC.ColorIndexAt(1, 0); got != 2 {
		t.Errorf("MC pixel (1,0) = %d, want 2", got)
	}
	// x=2,3 (bits 5-4) -> 10 -> screenColors lower nibble = 5 (Green)
	if got := imgMC.ColorIndexAt(2, 0); got != 5 {
		t.Errorf("MC pixel (2,0) = %d, want 5", got)
	}
	if got := imgMC.ColorIndexAt(3, 0); got != 5 {
		t.Errorf("MC pixel (3,0) = %d, want 5", got)
	}
	// x=4,5 (bits 3-2) -> 11 -> colorRAM = 7 (Yellow)
	if got := imgMC.ColorIndexAt(4, 0); got != 7 {
		t.Errorf("MC pixel (4,0) = %d, want 7", got)
	}
	if got := imgMC.ColorIndexAt(5, 0); got != 7 {
		t.Errorf("MC pixel (5,0) = %d, want 7", got)
	}
	// x=6,7 (bits 1-0) -> 00 -> bgColor = 9 (Brown)
	if got := imgMC.ColorIndexAt(6, 0); got != 9 {
		t.Errorf("MC pixel (6,0) = %d, want 9", got)
	}
	if got := imgMC.ColorIndexAt(7, 0); got != 9 {
		t.Errorf("MC pixel (7,0) = %d, want 9", got)
	}
}

func TestSpriteImage(t *testing.T) {
	raw := make([]byte, 64)
	// Row 0: top row, bytes 0, 1, 2. Let's set some bits
	// Byte 0: 0b10100000 -> standard: pixel 0=fg, pixel 1=transparent (0), pixel 2=fg
	raw[0] = 0b10100000

	sprite := &Sprite{
		Color: 2, // Red
		Raw:   raw,
	}

	// Test Standard 24x21
	img, err := sprite.Image()
	if err != nil {
		t.Fatalf("Image(): %v", err)
	}
	if img.Bounds() != image.Rect(0, 0, 24, 21) {
		t.Fatalf("bounds = %v", img.Bounds())
	}
	if got := img.ColorIndexAt(0, 0); got != 1 {
		t.Errorf("got %d, want 1 (Red)", got)
	}
	if got := img.ColorIndexAt(1, 0); got != 0 {
		t.Errorf("got %d, want 0 (transparent)", got)
	}

	// Test Expanded X & Y
	sprite.XExpand = true
	sprite.YExpand = true
	imgExpand, err := sprite.Image()
	if err != nil {
		t.Fatalf("Image(): %v", err)
	}
	if imgExpand.Bounds() != image.Rect(0, 0, 48, 42) {
		t.Fatalf("bounds = %v", imgExpand.Bounds())
	}
	// pixel (0,0) and (1,0) should be same color due to expansion
	if got := imgExpand.ColorIndexAt(0, 0); got != 1 {
		t.Errorf("got %d, want 1", got)
	}
	if got := imgExpand.ColorIndexAt(1, 0); got != 1 {
		t.Errorf("got %d, want 1", got)
	}
	if got := imgExpand.ColorIndexAt(2, 0); got != 0 {
		t.Errorf("got %d, want 0", got)
	}
}

func TestSpriteRoundtrip(t *testing.T) {
	// Standard sprite: start with raw bytes, decode to image, encode back
	raw := make([]byte, 64)
	raw[0] = 0b10000000 // pixel (0,0) set

	sprite := &Sprite{Color: 2, Raw: raw}
	img, err := sprite.Image()
	if err != nil {
		t.Fatalf("Image(): %v", err)
	}

	sprite2, err := NewSpriteFromImage(img)
	if err != nil {
		t.Fatalf("NewSpriteFromImage: %v", err)
	}
	if sprite2.Raw[0]&0x80 == 0 {
		t.Error("pixel (0,0) not preserved after roundtrip")
	}
	if len(sprite2.Raw) != 64 {
		t.Errorf("expected 64 bytes, got %d", len(sprite2.Raw))
	}

	// Wrong-size image should error
	wrong := image.NewPaletted(image.Rect(0, 0, 10, 10), nil)
	if _, err := NewSpriteFromImage(wrong); err == nil {
		t.Error("expected error for wrong-size image")
	}
}

func TestSpriteMulticolorRoundtrip(t *testing.T) {
	pal := color.Palette{
		color.RGBA{0, 0, 0, 0}, // 0 = transparent
		color.RGBA{0, 0, 0, 0}, // 1 = multicolor0
		color.RGBA{0, 0, 0, 0}, // 2 = main color
		color.RGBA{0, 0, 0, 0}, // 3 = multicolor1
	}
	img := image.NewPaletted(image.Rect(0, 0, 12, 21), pal)
	img.SetColorIndex(0, 0, 2) // bit pair 10
	img.SetColorIndex(1, 0, 3) // bit pair 11

	sprite, err := NewSpriteFromMulticolorImage(img)
	if err != nil {
		t.Fatalf("NewSpriteFromMulticolorImage: %v", err)
	}
	if len(sprite.Raw) != 64 {
		t.Fatalf("expected 64 bytes, got %d", len(sprite.Raw))
	}

	// bits 22-21 = 10, bits 20-19 = 11 → rowBits = 0x00B00000
	// byte0 = 0xB0, byte1 = 0x00, byte2 = 0x00
	if sprite.Raw[0] != 0xB0 || sprite.Raw[1] != 0x00 || sprite.Raw[2] != 0x00 {
		t.Errorf("row 0: bytes = $%02X $%02X $%02X, want $B0 $00 $00",
			sprite.Raw[0], sprite.Raw[1], sprite.Raw[2])
	}

	// Wrong-size image should error
	wrong := image.NewPaletted(image.Rect(0, 0, 10, 10), nil)
	if _, err := NewSpriteFromMulticolorImage(wrong); err == nil {
		t.Error("expected error for wrong-size multicolor image")
	}
}

func TestColorType(t *testing.T) {
	c := Color(2)
	if c.Name() != "RED" {
		t.Errorf("Color(2).Name() = %q, want RED", c.Name())
	}
	if ColorBlack != 0 || ColorWhite != 1 || ColorRed != 2 {
		t.Errorf("Color constants mismatch")
	}
	if ColorLightGrey != 15 {
		t.Errorf("ColorLightGrey = %d, want 15", ColorLightGrey)
	}
}

func TestCharsetCharacter(t *testing.T) {
	raw := make([]byte, 2048)
	// Character 0: checkerboard pattern
	raw[0] = 0b10101010
	raw[1] = 0b01010101
	raw[2] = 0b10101010
	raw[3] = 0b01010101
	raw[4] = 0b10101010
	raw[5] = 0b01010101
	raw[6] = 0b10101010
	raw[7] = 0b01010101

	cs := &Charset{Raw: raw}

	dots, err := cs.Character(0)
	if err != nil {
		t.Fatalf("Character(0): %v", err)
	}
	if len(dots) != 8 {
		t.Errorf("len = %d, want 8", len(dots))
	}
	if dots[0] != 0b10101010 {
		t.Errorf("dots[0] = %08b, want 10101010", dots[0])
	}

	// Out-of-range index
	if _, err := cs.Character(-1); err == nil {
		t.Error("expected error for Character(-1)")
	}
	if _, err := cs.Character(256); err == nil {
		t.Error("expected error for Character(256)")
	}

	// Short data
	csShort := &Charset{Raw: make([]byte, 100)}
	if _, err := csShort.Character(0); err == nil {
		t.Error("expected error for short Raw")
	}
}

func TestCharsetCharacterImage(t *testing.T) {
	raw := make([]byte, 2048)
	// Character 0: top-left pixel set
	raw[0] = 0b10000000

	cs := &Charset{Raw: raw}

	img, err := cs.CharacterImage(0, ColorWhite, ColorBlack)
	if err != nil {
		t.Fatalf("CharacterImage(0): %v", err)
	}

	bounds := img.Bounds()
	if bounds.Dx() != 8 || bounds.Dy() != 8 {
		t.Errorf("bounds = %v, want (0,0)-(8,8)", bounds)
	}

	// Top-left pixel (0,0) should be foreground (index 1).
	if got := img.ColorIndexAt(0, 0); got != 1 {
		t.Errorf("pixel (0,0) = %d, want 1 (foreground)", got)
	}

	// Pixel (1,0) should be background (index 0).
	if got := img.ColorIndexAt(1, 0); got != 0 {
		t.Errorf("pixel (1,0) = %d, want 0 (background)", got)
	}

	// Out-of-range index
	if _, err := cs.CharacterImage(256, ColorWhite, ColorBlack); err == nil {
		t.Error("expected error for CharacterImage(256)")
	}
}

func TestCharsetImageMap(t *testing.T) {
	raw := make([]byte, 2048)
	// Character 0: top-left pixel set
	raw[0] = 0b10000000
	// Character 32 (second row, first column): top-left pixel set
	raw[32*8] = 0b10000000

	cs := &Charset{Raw: raw}

	img := cs.ImageMap(ColorWhite, ColorBlack)

	bounds := img.Bounds()
	if bounds.Dx() != 256 || bounds.Dy() != 64 {
		t.Errorf("bounds = %v, want (0,0)-(256,64)", bounds)
	}

	// Character 0, pixel (0,0) → foreground.
	if got := img.ColorIndexAt(0, 0); got != 1 {
		t.Errorf("char 0 pixel (0,0) = %d, want 1", got)
	}

	// Character 32 at column 0, row 8 → foreground.
	if got := img.ColorIndexAt(0, 8); got != 1 {
		t.Errorf("char 32 pixel (0,8) = %d, want 1", got)
	}

	// Character 0, pixel (1,0) → background.
	if got := img.ColorIndexAt(1, 0); got != 0 {
		t.Errorf("char 0 pixel (1,0) = %d, want 0", got)
	}
}


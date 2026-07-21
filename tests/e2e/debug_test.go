package e2e

import (
	"context"
	"image"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/c64uploader/go-ultimate/c64"
	"github.com/c64uploader/go-ultimate/c64/codec"
)

func TestE2E_Screen(t *testing.T) {
	client, ctx := setupE2E(t)
	rebootAndReady(ctx, t, client)

	screen, err := client.Debug.Screen(ctx)
	if err != nil {
		t.Fatalf("Screen failed: %v", err)
	}

	if len(screen.Rows) != 25 {
		t.Fatalf("expected 25 screen rows, got %d", len(screen.Rows))
	}
	for i, row := range screen.Rows {
		if len(row) != 40 {
			t.Errorf("row %d: expected 40 columns, got %d", i, len(row))
		}
	}
	if len(screen.RawScreen) != 1000 {
		t.Errorf("expected 1000 bytes of raw screen data, got %d", len(screen.RawScreen))
	}
	if len(screen.RawColor) != 1000 {
		t.Errorf("expected 1000 bytes of raw color data, got %d", len(screen.RawColor))
	}

	// Decoded rows must match the raw screen codes.
	for i := range 25 {
		cc := codec.ScreenUppercase
		if screen.Charset == codec.CharsetLowercase {
			cc = codec.ScreenLowercase
		}
		want := cc.DecodeString(screen.RawScreen[i*40 : i*40+40])
		if screen.Rows[i] != want {
			t.Errorf("row %d: decoded text %q does not match raw screen codes", i, screen.Rows[i])
		}
	}

	// Write a known string into screen RAM and verify Debug.Screen decodes it.
	const (
		screenRow = 20
		screenCol = 5
		wantText  = "SCREEN READ OK"
	)
	asm := `
* = $c100
    ; Stop the KERNAL cursor blink from modifying screen RAM while we
    ; write and while the host reads the screen back.
    sei
    lda #$7f
    sta $dc0d
    lda $dc0d
    lda #$01
    sta $cc

    ldy #$00
copy:
    lda message,Y
    beq done
    sta $0725,Y
    iny
    jmp copy
done:
    lda #$01
    sta $c000
hold:
    jmp hold
message:
    .encoding "screencode_upper"
    .text "SCREEN READ OK"
    .byte 0
`
	statusVal, err := run6502(ctx, t, client, asm, 0xC000)
	if err != nil {
		t.Fatalf("run6502 failed: %v", err)
	}
	if statusVal != 0x01 {
		t.Fatalf("expected status 0x01, got $%02X", statusVal)
	}

	screen, err = client.Debug.Screen(ctx)
	if err != nil {
		t.Fatalf("Screen failed after write: %v", err)
	}

	got := strings.TrimSpace(screen.Rows[screenRow][screenCol:])
	if !strings.HasPrefix(got, wantText) {
		t.Errorf("row %d col %d: got %q, want prefix %q", screenRow, screenCol, got, wantText)
	}
}

func TestE2E_Charset(t *testing.T) {
	client, ctx := setupE2E(t)
	rebootAndReady(ctx, t, client)

	cs, err := client.Debug.Charset(ctx)
	if err != nil {
		t.Fatalf("Charset failed: %v", err)
	}
	t.Logf("Charset: raw bytes=%d", len(cs.Raw))
	if len(cs.Raw) != 2048 {
		t.Errorf("expected 2048 bytes of character data, got %d", len(cs.Raw))
	}
}

func TestE2E_Sprites(t *testing.T) {
	client, ctx := setupE2E(t)
	rebootAndReady(ctx, t, client)

	// Go Gopher sprite (63 bytes, 24x21 multicolor, 12 logical pixels wide).
	// Colors: 00=transparent 01=white(eyes/teeth) 10=cyan(body) 11=black(pupils/nose)
	var gopherData = []byte{
		0x00, 0x00, 0x00, 0x14, 0x54, 0x50, 0x1d, 0x55,
		0xd0, 0x1d, 0x55, 0xd0, 0x16, 0x56, 0x50, 0x0a,
		0x9a, 0x80, 0x0a, 0xda, 0xc0, 0x0a, 0xda, 0xc0,
		0x0a, 0x9a, 0x80, 0x06, 0x76, 0x40, 0x05, 0x75,
		0x40, 0x05, 0x65, 0x40, 0x05, 0x65, 0x40, 0x15,
		0x55, 0x50, 0x15, 0x55, 0x50, 0x05, 0x55, 0x40,
		0x05, 0x55, 0x40, 0x05, 0x55, 0x40, 0x05, 0x55,
		0x40, 0x05, 0x55, 0x40, 0x01, 0x01, 0x00, 0x81,
	}

	// Write sprite data directly to $0C00 (sprite pointer $30 * 64)
	if err := client.Machine.WriteMemory(ctx, 0x0C00, gopherData); err != nil {
		t.Fatalf("WriteMemory failed: %v", err)
	}

	asm := `
* = $c100
    ; 1. Enable Sprite 0
    lda #$01
    sta $d015

    ; 2. Enable Multicolor mode for Sprite 0
    lda #$01
    sta $d01c

    ; 3. Position Sprite 0
    lda #100
    sta $d000
    lda #150
    sta $d001

    ; 4. Set Colors
    lda #$01        ; White
    sta $d027
    lda #$03        ; Cyan
    sta $d025
    lda #$00        ; Black
    sta $d026

    ; 5. Point Sprite 0 to Page $30 ($0C00)
    lda #$30
    sta $07f8

    lda #$01
    sta $c000
    rts
`

	statusVal, err := run6502(ctx, t, client, asm, 0xC000)
	if err != nil {
		t.Fatalf("run6502 failed: %v", err)
	}
	if statusVal != 0x01 {
		t.Fatalf("expected status 0x01, got $%02X", statusVal)
	}

	sprites, err := client.Debug.Sprites(ctx)
	if err != nil {
		t.Fatalf("Sprites failed: %v", err)
	}

	s0 := sprites[0]
	if !s0.Enabled {
		t.Fatal("Expected Sprite 0 to be enabled")
	}
	if s0.Color != 0x01 {
		t.Errorf("Expected Sprite 0 base color $01, got $%02X", s0.Color)
	}

	img, err := s0.Image()
	if err != nil {
		t.Errorf("Image(): %v", err)
	}
	_ = img

	// write the sprite image to a PNG file for visual inspection (optional)
	// Uncomment the following lines to save the sprite image as a PNG file
	/*	outFile, err := os.Create("sprite0.png")
		if err != nil {
			t.Fatalf("Failed to create sprite0.png: %v", err)
		}
		defer outFile.Close()
		if err := png.Encode(outFile, img); err != nil {
			t.Fatalf("Failed to encode sprite image: %v", err)
		}
		t.Log("Sprite 0 image saved to sprite0.png")
	*/
}

func TestE2E_ScreenMode(t *testing.T) {
	client, ctx := setupE2E(t)
	rebootAndReady(ctx, t, client)

	// After reset, the VIC is in standard text mode.
	mode, err := client.Debug.ScreenMode(ctx)
	if err != nil {
		t.Fatalf("ScreenMode failed: %v", err)
	}
	if mode != c64.ScreenText {
		t.Errorf("expected screen text mode, got %v", mode)
	}
}

func TestE2E_Bitmap(t *testing.T) {
	client, ctx := setupE2E(t)
	rebootAndReady(ctx, t, client)

	bitmapData, err := os.ReadFile("testdata/golang_bitmap.bin")
	if err != nil {
		t.Fatalf("failed to load bitmap data: %v", err)
	}
	screenData, err := os.ReadFile("testdata/golang_screen.bin")
	if err != nil {
		t.Fatalf("failed to load screen color data: %v", err)
	}

	if len(bitmapData) != 8000 {
		t.Fatalf("unexpected bitmap size: got %d, want 8000", len(bitmapData))
	}
	if len(screenData) != 1000 {
		t.Fatalf("unexpected screen data size: got %d, want 1000", len(screenData))
	}

	asm := `
* = $c100
    lda #$3b
    sta $d011
    lda #$18
    sta $d018

    lda #$01
    sta $c000
hold:
    jmp hold
`
	statusVal, err := run6502(ctx, t, client, asm, 0xC000)
	if err != nil {
		t.Fatalf("run6502 failed to enable bitmap mode: %v", err)
	}
	if statusVal != 0x01 {
		t.Fatalf("expected status 0x01, got $%02X", statusVal)
	}

	if err := client.Machine.WriteMemory(ctx, 0x2000, bitmapData); err != nil {
		t.Fatalf("failed to write bitmap data: %v", err)
	}
	if err := client.Machine.WriteMemory(ctx, 0x0400, screenData); err != nil {
		t.Fatalf("failed to write screen colors: %v", err)
	}

	var img *image.Paletted
	img, err = client.Debug.Bitmap(ctx)
	if err != nil {
		t.Fatalf("Bitmap failed: %v", err)
	}

	bounds := img.Bounds()
	if bounds.Dx() != 320 || bounds.Dy() != 200 {
		t.Fatalf("unexpected image bounds: got %dx%d, want 320x200", bounds.Dx(), bounds.Dy())
	}

	seen := map[uint8]struct{}{}
	for y := 0; y < 200; y += 16 {
		for x := 0; x < 320; x += 16 {
			seen[img.ColorIndexAt(x, y)] = struct{}{}
		}
	}
	if len(seen) < 2 {
		t.Errorf("expected varied colors from converted image, got only %d distinct in samples", len(seen))
	}
}

func TestE2E_Interactive_SID(t *testing.T) {
	client, _ := setupE2E(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	rebootAndReady(ctx, t, client)

	startSIDAsm := `
* = $c200
    lda #$0f
    sta $d418
    lda #$00
    sta $d405
    lda #$f0
    sta $d406
    lda #$00
    sta $d400
    lda #$20
    sta $d401

    lda #$11
    sta $d404

    lda #$01
    sta $c000
    rts
`
	t.Log("Interactive SID Test: Starting audio tone...")
	_, err := run6502(ctx, t, client, startSIDAsm, 0xC000)
	if err != nil {
		t.Fatalf("SID interactive test failed to start tone: %v", err)
	}

	passed, err := runInteractivePrompt(ctx, t, client, "\n\nare you hearing the sid audio tone?")
	if err != nil {
		t.Fatalf("Interactive prompt failed: %v", err)
	}

	stopSIDAsm := `
* = $c200
    lda #$10
    sta $d404
    lda #$01
    sta $c000
    rts
`
	_, _ = run6502(ctx, t, client, stopSIDAsm, 0xC000)

	if !passed {
		t.Error("Audio verification failed: user reported no audio was heard.")
	}
}

func TestE2E_AssembleAndRun(t *testing.T) {
	client, ctx := setupE2E(t)
	rebootAndReady(ctx, t, client)

	asm := `
* = $c100
    lda #$42
    sta $c010
    ldx #$55
    stx $c011
    lda #$01
    sta $c000
    rts
`
	statusVal, err := run6502(ctx, t, client, asm, 0xC000)
	if err != nil {
		t.Fatalf("run6502 failed: %v", err)
	}

	if statusVal != 0x01 {
		t.Errorf("expected status 0x01, got $%02X", statusVal)
	}

	vals, err := client.Machine.ReadMemory(ctx, 0xC010, 2)
	if err != nil {
		t.Fatalf("ReadMemory failed: %v", err)
	}

	if vals[0] != 0x42 {
		t.Errorf("expected $C010 to be $42, got $%02X", vals[0])
	}
	if vals[1] != 0x55 {
		t.Errorf("expected $C011 to be $55, got $%02X", vals[1])
	}
}

func TestE2E_Disassemble(t *testing.T) {
	client, ctx := setupE2E(t)
	rebootAndReady(ctx, t, client)

	asm := `
* = $c100
    lda #$42
    sta $c010
    nop
    rts
`
	prog, err := c64.Assemble(asm)
	if err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}

	if err := client.Machine.Inject(ctx, prog); err != nil {
		t.Fatalf("Inject failed: %v", err)
	}

	data, err := client.Machine.ReadMemory(ctx, 0xC100, 7)
	if err != nil {
		t.Fatalf("ReadMemory failed: %v", err)
	}
	insts := c64.Disassemble(data, 0xC100)

	if len(insts) != 4 {
		t.Fatalf("expected 4 instructions, got %d", len(insts))
	}

	expected := []struct {
		address uint16
		code    string
	}{
		{0xC100, "LDA #$42"},
		{0xC102, "STA $C010"},
		{0xC105, "NOP"},
		{0xC106, "RTS"},
	}

	for i, exp := range expected {
		if insts[i].Address != exp.address {
			t.Errorf("instruction %d: expected address $%04X, got $%04X", i, exp.address, insts[i].Address)
		}
		if insts[i].Code != exp.code {
			t.Errorf("instruction %d: expected code %q, got %q", i, exp.code, insts[i].Code)
		}
	}
}



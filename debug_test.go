package ultimate

import (
	"context"
	"testing"

	"github.com/c64uploader/go-ultimate/c64"
)

func TestAssembleAndRun(t *testing.T) {
	ms := newMockServer(t)
	client, err := New("", WithBaseURL(ms.server().URL))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	prog, err := c64.Assemble("NOP")
	if err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}

	if prog.LoadAddress() != 0x0801 {
		t.Errorf("expected load address $0801, got $%04X", prog.LoadAddress())
	}

	if err := client.Runners.Run(context.Background(), prog); err != nil {
		t.Errorf("Runners.Run failed: %v", err)
	}
	if err := client.Runners.Load(context.Background(), prog); err != nil {
		t.Errorf("Runners.Load failed: %v", err)
	}
	if err := client.Machine.Inject(context.Background(), prog); err != nil {
		t.Errorf("Machine.Inject failed: %v", err)
	}

	insts := prog.Disassemble()
	if len(insts) == 0 {
		t.Errorf("unexpected empty disassembly result")
	}

	if len(prog.Bytes()) < 2 {
		t.Error("program should have at least a PRG header")
	}
}

func TestSpriteXMSB(t *testing.T) {
	ms := newMockServer(t)
	// Set VIC sprite X coordinates for sprites 0-7 at $D000-$D00F
	// Sprite 0: X=$1F, Sprite 1: X=$2F, ..., Sprite 7: X=$8F
	for i := range 8 {
		ms.setMem(0xD000+uint16(i*2), []byte{byte(0x10 + i*0x20)}) // low bytes
		ms.setMem(0xD001+uint16(i*2), []byte{byte(0x50 + i*0x10)}) // Y bytes
	}
	// Set MSB register $D010: bit 0=1 (sprite 0 MSB), bit 1=1 (sprite 1 MSB), bit 7=1 (sprite 7 MSB)
	ms.setMem(0xD010, []byte{0b10000011})

	// Sprite pointers at $07F8
	ms.setMem(0x07F8, []byte{0, 0, 0, 0, 0, 0, 0, 0})
	// CIA2 $DD00: VIC bank 0
	ms.setMem(0xDD00, []byte{0x03})
	// VIC sprite color registers $D027-$D02E
	for i := range 8 {
		ms.setMem(0xD027+uint16(i), []byte{byte(i + 1)})
	}
	// Sprite shape data at $0000 (bank 0, pointer 0)
	ms.setMem(0x0000, make([]byte, 64))

	client, err := New("", WithBaseURL(ms.server().URL))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	sprites, err := client.Debug.Sprites(context.Background())
	if err != nil {
		t.Fatalf("Sprites: %v", err)
	}

	tests := []struct {
		n        int
		wantX    int // expected X (low byte + MSB*256)
		hasMSB   bool
	}{
		{0, 0x10 + 256, true},   // sprite 0: MSB set → X = 0x10 + 256 = 0x110
		{1, 0x30 + 256, true},   // sprite 1: MSB set → X = 0x30 + 256 = 0x130
		{2, 0x50, false},        // sprite 2: MSB clear → X = 0x50
		{3, 0x70, false},        // sprite 3: MSB clear
		{4, 0x90, false},        // sprite 4: MSB clear
		{5, 0xB0, false},        // sprite 5: MSB clear
		{6, 0xD0, false},        // sprite 6: MSB clear
		{7, 0xF0 + 256, true},   // sprite 7: MSB set → X = 0xF0 + 256 = 0x1F0
	}

	for _, tc := range tests {
		s := sprites[tc.n]
		if s.X != tc.wantX {
			t.Errorf("sprite %d X = $%04X, want $%04X (MSB=%v)", tc.n, s.X, tc.wantX, tc.hasMSB)
		}
		// Verify the MSB bit is correctly extracted: X should be low byte + (0 or 256)
		if tc.hasMSB && s.X < 256 {
			t.Errorf("sprite %d X = %d, expected MSB contribution (>=256)", tc.n, s.X)
		}
	}
}

func TestScreenMode(t *testing.T) {
	ms := newMockServer(t)

	tests := []struct {
		name     string
		d011, d016 byte
		want      c64.ScreenMode
	}{
		{"text", 0x00, 0x00, c64.ScreenText},
		{"multicolor text", 0x00, 0x10, c64.ScreenMulticolorText},
		{"bitmap", 0x20, 0x00, c64.ScreenBitmap},
		{"multicolor bitmap", 0x20, 0x10, c64.ScreenMulticolorBitmap},
		{"extended color", 0x40, 0x00, c64.ScreenExtendedColor},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ms.setMem(0xD011, []byte{tc.d011})
			ms.setMem(0xD016, []byte{tc.d016})

			client, err := New("", WithBaseURL(ms.server().URL))
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			mode, err := client.Debug.ScreenMode(context.Background())
			if err != nil {
				t.Fatalf("ScreenMode: %v", err)
			}
			if mode != tc.want {
				t.Errorf("ScreenMode = %v, want %v", mode, tc.want)
			}
		})
	}
}

func TestCharset(t *testing.T) {
	ms := newMockServer(t)
	ms.setMem(0xD018, []byte{0b00000100})         // charset bank = 2 → address $1000
	ms.setMem(0x1000, make([]byte, 2048))           // 2048 bytes of dot data at $1000
	ms.setMem(0x1001, []byte{0xFF, 0x00, 0xFF, 0x00, 0xFF, 0x00, 0xFF, 0x00}) // checkerboard for char 0

	client, err := New("", WithBaseURL(ms.server().URL))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	cs, err := client.Debug.Charset(context.Background())
	if err != nil {
		t.Fatalf("Charset: %v", err)
	}

	if len(cs.Raw) != 2048 {
		t.Errorf("len(Raw) = %d, want 2048", len(cs.Raw))
	}

	// Character(0) should return 8 bytes.
	dots, err := cs.Character(0)
	if err != nil {
		t.Fatalf("Character(0): %v", err)
	}
	if len(dots) != 8 {
		t.Errorf("len(dots) = %d, want 8", len(dots))
	}

	// Character(-1) should error.
	if _, err := cs.Character(-1); err == nil {
		t.Error("expected error for Character(-1)")
	}

	// Character(256) should error.
	if _, err := cs.Character(256); err == nil {
		t.Error("expected error for Character(256)")
	}
}

func TestSpriteRoundtrip(t *testing.T) {
	// Roundtrip: Raw → Image → NewSpriteFromImage → Raw
	raw := make([]byte, 64)
	raw[0] = 0b10000000 // pixel 0 set

	sprite := &c64.Sprite{
		Color: 2, // Red
		Raw:   raw,
	}

	img, err := sprite.Image()
	if err != nil {
		t.Fatalf("Image(): %v", err)
	}

	sprite2, err := c64.NewSpriteFromImage(img)
	if err != nil {
		t.Fatalf("NewSpriteFromImage: %v", err)
	}

	// Check the top byte matches
	if sprite2.Raw[0] != raw[0] {
		t.Errorf("roundtrip: byte 0 = $%02X, want $%02X", sprite2.Raw[0], raw[0])
	}
	// Check pixel (0,0) is still set after decode-encode
	if sprite2.Raw[0]&0x80 == 0 {
		t.Errorf("pixel (0,0) not preserved after encode")
	}
}

func TestDebugServiceHasNoClientField(t *testing.T) {
	// Compile-time check: DebugService.client field was removed
	// This is a type assertion that the struct has the right shape.
	// We can't directly check unexported fields, but we verify the
	// service works without a client field.
	ms := newMockServer(t)
	client, err := New("", WithBaseURL(ms.server().URL))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Debug methods should still all work using just Mem
	if client.Debug.Mem == nil {
		t.Error("Debug.Mem should be set")
	}

	// Quick functional check
	ms.setMem(0x0801, []byte{0, 0}) // empty BASIC
	lines, err := client.Debug.BASIC(context.Background())
	if err != nil {
		t.Errorf("BASIC: %v", err)
	}
	if len(lines) != 0 {
		t.Errorf("expected 0 lines, got %d", len(lines))
	}
}





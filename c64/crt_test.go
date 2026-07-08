package c64

import (
	"bytes"
	"testing"
)

func TestCartridge_Normal8K(t *testing.T) {
	// Minimal valid cart image: vectors + CBM80 + sei + sta + jmp (payload size 8K padded).
	data := make([]byte, 8192)
	// cold vector -> 0x0009 (after header)
	data[0] = 0x09
	data[1] = 0x80
	data[2] = 0x09
	data[3] = 0x80
	copy(data[4:9], []byte{0xC3, 0xC2, 0xCD, 0x38, 0x30})
	data[9] = 0x78 // sei
	data[10] = 0xA9
	data[11] = 0x42
	data[12] = 0x8D
	data[13] = 0x00
	data[14] = 0xC0
	data[15] = 0x4C
	data[16] = 0x0F
	data[17] = 0x80

	crt, err := NewRawCartridge(CRTNormal8K, "DEMO", data)
	if err != nil {
		t.Fatal(err)
	}
	if len(crt) != 64+16+8192 {
		t.Fatalf("bad len %d", len(crt))
	}
	if string(crt[0:16]) != "C64 CARTRIDGE   " {
		t.Error("bad signature")
	}
	if crt[0x16] != 0 || crt[0x17] != 0 {
		t.Error("bad type")
	}
	if crt[0x18] != 0 || crt[0x19] != 1 {
		t.Error("bad ex/g for 8K")
	}
	if string(crt[64:68]) != "CHIP" {
		t.Error("bad chip")
	}
	// load addr BE 8000, size 2000
	if crt[0x4C] != 0x80 || crt[0x4D] != 0x00 || crt[0x4E] != 0x20 || crt[0x4F] != 0x00 {
		t.Error("bad chip load/size")
	}
}

func TestCartridge_BadSize(t *testing.T) {
	_, err := NewRawCartridge(CRTNormal8K, "DEMO", make([]byte, 20000))
	if err == nil {
		t.Error("expected error")
	}
}

func TestNewRawCartridge_Ultimax16K(t *testing.T) {
	// Provide 16 KiB (16384 bytes) of ROM data.
	// It should split it into two 8 KiB CHIP packets at $8000 and $E000.
	data := make([]byte, 16384)
	for i := range data {
		data[i] = byte(i & 0xFF)
	}

	crt, err := NewRawCartridge(CRTUltimax, "UltiTest", data)
	if err != nil {
		t.Fatalf("Failed to build Ultimax 16K: %v", err)
	}

	// 64 (global header) + 16 (chip 1 header) + 8192 (chip 1) + 16 (chip 2 header) + 8192 (chip 2) = 16480
	expectedLen := 64 + 16 + 8192 + 16 + 8192
	if len(crt) != expectedLen {
		t.Fatalf("Expected size %d, got %d", expectedLen, len(crt))
	}

	// Global header validation
	if string(crt[0:16]) != "C64 CARTRIDGE   " {
		t.Error("bad signature")
	}
	if crt[0x18] != 1 || crt[0x19] != 0 {
		t.Errorf("expected EXROM=1 GAME=0 for Ultimax, got EXROM=%d GAME=%d", crt[0x18], crt[0x19])
	}
	if !bytes.Contains(crt[0x20:0x40], []byte("ULTITEST")) {
		t.Errorf("expected uppercase name, got %q", string(crt[0x20:0x40]))
	}

	// Chip 1 packet at $8000
	chip1Offset := 64
	if string(crt[chip1Offset:chip1Offset+4]) != "CHIP" {
		t.Error("chip 1 signature incorrect")
	}
	// Load Address BE at offset + 12 (0x0C)
	load1 := uint16(crt[chip1Offset+12])<<8 | uint16(crt[chip1Offset+13])
	if load1 != 0x8000 {
		t.Errorf("expected chip 1 load address $8000, got $%04X", load1)
	}

	// Chip 2 packet at $E000
	chip2Offset := 64 + 16 + 8192
	if string(crt[chip2Offset:chip2Offset+4]) != "CHIP" {
		t.Error("chip 2 signature incorrect")
	}
	load2 := uint16(crt[chip2Offset+12])<<8 | uint16(crt[chip2Offset+13])
	if load2 != 0xE000 {
		t.Errorf("expected chip 2 load address $E000, got $%04X", load2)
	}
}

func TestNewXIPCartridge(t *testing.T) {
	// User assembly that sets border color and loops
	userAsm := `
	* = $8040
	lda #$02
	sta $d020
loop:
	jmp loop
`
	prog, err := Assemble(userAsm)
	if err != nil {
		t.Fatalf("Failed to assemble test program: %v", err)
	}

	crt, err := NewXIPCartridge(CRTNormal8K, "XIP_TEST", prog)
	if err != nil {
		t.Fatalf("NewXIPCartridge failed: %v", err)
	}

	expectedLen := 64 + 16 + 8192
	if len(crt) != expectedLen {
		t.Fatalf("expected len %d, got %d", expectedLen, len(crt))
	}

	// The payload is mapped at $8000
	payload := crt[80:]
	// The RESET vector at $8000 should point to cold_start ($8009)
	resetVector := uint16(payload[0]) | uint16(payload[1])<<8
	if resetVector != 0x8009 {
		t.Errorf("expected RESET vector $8009, got $%04X", resetVector)
	}
	// CBM80 signature
	sig := payload[4:9]
	if !bytes.Equal(sig, []byte{0xC3, 0xC2, 0xCD, 0x38, 0x30}) {
		t.Errorf("expected CBM80 signature, got %v", sig)
	}

	// Verify bootstrap jump target points to $8040
	jmpOp := payload[24]
	jmpTarget := uint16(payload[25]) | uint16(payload[26])<<8
	if jmpOp != 0x4C || jmpTarget != 0x8040 {
		t.Errorf("expected bootstrap jump to $8040 (0x4C, 0x40, 0x80), got 0x%02X, $%04X", jmpOp, jmpTarget)
	}

	// Test overlap detection
	badProg, err := Assemble("* = $8030\n nop")
	if err == nil {
		_, err = NewXIPCartridge(CRTNormal8K, "BAD", badProg)
		if err == nil {
			t.Error("Expected error for overlapping load address")
		}
	}
}

func TestNewRAMCartridge(t *testing.T) {
	// Build a standard program compiled at $0801 with BASICHeader
	asm := `
		* = $0801
		BASICHeader(entry)
	entry:
		lda #$42
		sta $c000
		rts
`
	prog, err := Assemble(asm)
	if err != nil {
		t.Fatalf("Failed to assemble test program: %v", err)
	}

	// Create RAM relocation cartridge
	crt, err := NewRAMCartridge(CRTNormal8K, "RAM_TEST", prog)
	if err != nil {
		t.Fatalf("NewRAMCartridge failed: %v", err)
	}

	expectedLen := 64 + 16 + 8192
	if len(crt) != expectedLen {
		t.Fatalf("expected len %d, got %d", expectedLen, len(crt))
	}

	// 1. Test Ultimax rejection in NewRAMCartridge
	_, err = NewRAMCartridge(CRTUltimax, "RAM_TEST_ULTI", prog)
	if err == nil {
		t.Error("Expected error when using CRTUltimax with NewRAMCartridge")
	}

	// 2. Test dynamic entry point parsing from custom SYS command
	// e.g. 10 SYS 2064 (which is $0810)
	customAsm := `
		* = $0801
		.word next
		.word 10
		.byte $9E ; SYS token
		.text " 2064"
		.byte 0 ; end of line
	next:
		.word 0 ; end of basic
		.org $0810
	entry:
		lda #$ff
		rts
	`
	customProg, err := Assemble(customAsm)
	if err != nil {
		t.Fatalf("Failed to assemble custom program: %v", err)
	}
	// We expect the bootstrap code to jump to $0810
	crtCustom, err := NewRAMCartridge(CRTNormal8K, "RAM_CUSTOM", customProg)
	if err != nil {
		t.Fatalf("NewRAMCartridge with custom program failed: %v", err)
	}
	// The jmp target is at the end of the bootstrap. Let's find it.
	// Since we disassembled and know the layout: the jmp instruction (opcode $4C)
	// should have target $0810.
	payload := crtCustom[80:]
	// Let's find $4C followed by $10 $08 in the payload
	found := false
	for i := 0; i < len(payload)-2; i++ {
		if payload[i] == 0x4C && payload[i+1] == 0x10 && payload[i+2] == 0x08 {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected to find jump to entry point $0810 in RAM cartridge bootstrap")
	}

	// 3. Test program loaded at $0801 but WITHOUT a BASIC header
	rawAsm := `
		* = $0801
		lda #$42
		sta $c000
		rts
	`
	rawProg, err := Assemble(rawAsm)
	if err != nil {
		t.Fatalf("Failed to assemble raw program: %v", err)
	}
	crtRaw, err := NewRAMCartridge(CRTNormal8K, "RAM_RAW", rawProg)
	if err != nil {
		t.Fatalf("NewRAMCartridge with raw program failed: %v", err)
	}
	payloadRaw := crtRaw[80:]
	// We expect the bootstrap code to jump to $0801 (since there is no BASIC header)
	foundRaw := false
	for i := 0; i < len(payloadRaw)-2; i++ {
		if payloadRaw[i] == 0x4C && payloadRaw[i+1] == 0x01 && payloadRaw[i+2] == 0x08 {
			foundRaw = true
			break
		}
	}
	if !foundRaw {
		t.Error("Expected to find jump to entry point $0801 in RAM cartridge bootstrap for program without BASIC header")
	}
}

func TestCartridge_SizeChecksAndPadding(t *testing.T) {
	// 1. 8K size check
	tooLarge8K := make([]byte, 9000)
	_, err := NewRawCartridge(CRTNormal8K, "LARGE8K", tooLarge8K)
	if err == nil {
		t.Error("Expected error when raw data size is > 8 KiB for CRTNormal8K")
	}

	// 2. 16K normal padding
	small16K := make([]byte, 4096)
	crt16K, err := NewRawCartridge(CRTNormal16K, "PAD16K", small16K)
	if err != nil {
		t.Fatalf("Failed to create 16K normal cartridge: %v", err)
	}
	// Header (64) + Chip Header (16) + Padded Data (16384) = 16464
	expectedLen := 64 + 16 + 16384
	if len(crt16K) != expectedLen {
		t.Fatalf("Expected size %d for 16K padded normal cartridge, got %d", expectedLen, len(crt16K))
	}
}


package e2e

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestE2E_MemoryDMA(t *testing.T) {
	client, ctx := setupE2E(t)
	rebootAndReady(ctx, t, client)

	// Write a test pattern to the Cassette Buffer ($0340)
	addr := uint16(0x0340)
	want := []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0xAA, 0x55, 0xFF}
	if err := client.Machine.WriteMemory(ctx, addr, want); err != nil {
		t.Fatalf("WriteMemory failed: %v", err)
	}

	got, err := client.Machine.ReadMemory(ctx, addr, len(want))
	if err != nil {
		t.Fatalf("ReadMemory failed: %v", err)
	}

	if !bytes.Equal(got, want) {
		t.Errorf("ReadMemory = %v, want %v", got, want)
	}

	// Test Peek/Poke
	pokeAddr := uint16(0x0348)
	if err := client.Machine.Poke(ctx, pokeAddr, 0xBC); err != nil {
		t.Fatalf("Poke failed: %v", err)
	}

	peeked, err := client.Machine.Peek(ctx, pokeAddr)
	if err != nil {
		t.Fatalf("Peek failed: %v", err)
	}

	if peeked != 0xBC {
		t.Errorf("Peek = $%02X, want $BC", peeked)
	}
}

func TestE2E_Interactive_Joystick(t *testing.T) {
	client, _ := setupE2E(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	rebootAndReady(ctx, t, client)

	asm := `
* = $c100
    jsr print_msg

wait_input:
    jsr $ffe4   ; GETIN
    cmp #$4e    ; 'N'
    beq pressed_n
    cmp #$6e    ; 'n'
    beq pressed_n

    lda $dc00   ; CIA1 Port A (Joystick 2)
    and #$10    ; bit 4 is fire (active low)
    beq pressed_fire
    jmp wait_input

pressed_fire:
    lda #$01
    sta $c000
    rts

pressed_n:
    lda #$02
    sta $c000
    rts

print_msg:
    ldy #$00
print_loop:
    lda msg,Y
    beq print_done
    jsr $ffd2   ; BSOUT
    iny
    jmp print_loop
print_done:
    rts

msg:
    .encoding "petscii_upper"
    .text "\n\nplease press fire button on\njoystick 2 (pass) or press n (fail): "
    .byte 0
`

	t.Log("Press fire on Joy 2 (Pass) or press 'n' (Fail).")
	statusVal, err := run6502(ctx, t, client, asm, 0xC000)
	if err != nil {
		t.Fatalf("Joystick interactive test failed: %v", err)
	}

	switch statusVal {
	case 0x01:
		t.Log("Joystick Fire detected successfully!")
	case 0x02:
		t.Error("Joystick verification failed: user reported failure or skipped.")
	default:
		t.Errorf("Unexpected status code: $%02X", statusVal)
	}
}

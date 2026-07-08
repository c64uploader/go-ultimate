package e2e

import (
	"fmt"
	"testing"
	"time"

	"github.com/c64uploader/go-ultimate/c64"
)

func TestE2E_BASICProgramAndRun(t *testing.T) {
	client, ctx := setupE2E(t)
	rebootAndReady(ctx, t, client)

	progText := "10 for i=1 to 10\n20 print \"go-ultimate\"; i\n30 next i\nrun\n"
	if err := client.Keyboard.Type(ctx, progText); err != nil {
		t.Fatalf("Type failed: %v", err)
	}

	verifyScreenContains(ctx, t, client, []string{"GO-ULTIMATE 10"})
}

func TestE2E_KeyboardPressCIA(t *testing.T) {
	client, ctx := setupE2E(t)
	rebootAndReady(ctx, t, client)

	asm := `
* = $c100
    sei
    lda #$ff
    sta $dc02
    lda #$00
    sta $dc03
poll:
    lda #$7f
    sta $dc00
    lda $dc01
    and #$10    ; isolate row 4 bit (Space row)
    beq done    ; bit 4 low = space pressed
    jmp poll
done:
    lda #$37
    sta $c000
hang:
    jmp hang
`
	prog, err := c64.Assemble(asm)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	if err := client.Machine.Poke(ctx, 0xC000, 0); err != nil {
		t.Fatalf("Poke: %v", err)
	}
	if err := client.Machine.Inject(ctx, prog); err != nil {
		t.Fatalf("Inject: %v", err)
	}
	if err := client.Keyboard.Type(ctx, fmt.Sprintf("sys %d\n", prog.LoadAddress())); err != nil {
		t.Fatalf("Type: %v", err)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if err := client.Keyboard.Press(ctx, c64.KeySpace); err != nil {
			t.Fatalf("Press: %v", err)
		}
		v, err := client.Machine.Peek(ctx, 0xC000)
		if err != nil {
			t.Fatalf("Peek: %v", err)
		}
		if v == 0x37 {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(150 * time.Millisecond):
		}
	}
	t.Fatal("timeout")
}

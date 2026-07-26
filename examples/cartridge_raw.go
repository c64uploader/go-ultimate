//go:build ignore

// Run: go run examples/cartridge_raw.go
package main

import (
	"context"

	"github.com/c64uploader/go-ultimate"
	"github.com/c64uploader/go-ultimate/c64"
)

func main() {
	client, _ := ultimate.New("c64u")
	ctx := context.Background()

	// NewRawCartridge takes complete, raw ROM bytes.
	// You must manually supply the cartridge header at $8000 using CartridgeHeader(boot).
	userAsm := `
    * = $8000
    CartridgeHeader(boot)

boot:
    sei
    cld
    jsr $ff84                    ; IOINIT: Initialize CIA chips
    jsr $ff87                    ; RAMTAS: Initialize RAM/ZP
    jsr $ff8a                    ; RESTOR: Restore KERNAL vectors
    jsr $ff81                    ; CINT: Initialize screen/VIC-II
    cli

    lda #$07                     ; Color #7 = Yellow
    sta $d020                    ; Set border color

    ldx #0
print_loop:
    lda message,x
    beq hold
    jsr $ffd2                    ; Print character via KERNAL CHROUT
    inx
    jmp print_loop

hold:
    jmp hold                     ; Infinite loop

message:
    .encoding "petscii_upper"
    .text "HELLO FROM RAW CARTRIDGE!"
    .byte 13, 0
`

	prog, _ := c64.Assemble(userAsm)

	// Wrap raw ROM bytes directly into an 8 KiB cartridge image without adding helper code.
	crt, _ := c64.NewRawCartridge(c64.CRTNormal8K, "RAW DEMO", prog.Code())

	// Upload and boot the cartridge on the Commodore 64.
	_ = client.Runners.RunCRTBytes(ctx, crt)
}

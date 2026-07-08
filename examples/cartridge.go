//go:build ignore

// Run: go run examples/cartridge.go
package main

import (
	"context"

	"github.com/c64uploader/go-ultimate"
	"github.com/c64uploader/go-ultimate/c64"
)

func main() {
	client, _ := ultimate.New("c64u")
	ctx := context.Background()

	// NewRAMCartridge writes a C64 program compiled for RAM (starting at $0801), with or without a BASIC SYS header.
	// The cartridge bootstrap automatically copies the program from ROM to RAM and runs it.
	userAsm := `
    * = $0801

entry:
    lda #$05
    sta $d020                    ; border color $05 = green

    ldx #0
print_loop:
    lda message,x
    beq hold
    jsr $ffd2                    ; print character via KERNAL CHROUT
    inx
    jmp print_loop
hold:
    jmp hold                     ; loop forever; cart stays active

message:
    .encoding "petscii_upper"
    .text "HELLO FROM CARTRIDGE!"
    .byte 13, 0                  ; carriage return and null terminator
`

	// Compile and package into a .CRT cartridge image.
	prog, _ := c64.Assemble(userAsm)
	crt, _ := c64.NewRAMCartridge(c64.CRTNormal8K, "DEMO", prog)

	// Upload and run — like plugging the cartridge in and resetting the C64.
	_ = client.Runners.RunCRTBytes(ctx, crt)
}

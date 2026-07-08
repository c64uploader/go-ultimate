//go:build ignore

// Run: go run examples/assembler.go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/c64uploader/go-ultimate"
	"github.com/c64uploader/go-ultimate/c64"
)

func main() {
	client, _ := ultimate.New("c64u")
	ctx := context.Background()

	// Assemble a simple program that sets the border green and returns to BASIC.
	program, _ := c64.Assemble(`
		* = $1000
		lda #$05
		sta $d020
		rts
	`)

	// Upload the program into C64 RAM without resetting.
	_ = client.Machine.Inject(ctx, program)

	// Type the SYS command to run it.
	_ = client.Keyboard.Type(ctx, fmt.Sprintf("sys %d\n", program.LoadAddress()))
	time.Sleep(3 * time.Second)

	// Read back the program from RAM and disassemble it.
	mem, _ := client.Machine.ReadMemory(ctx, program.LoadAddress(), program.Size())
	d := c64.NewProgram(mem, program.LoadAddress()).Disassemble()
	for _, inst := range d {
		fmt.Printf("0x%04x: %s\n", inst.Address, inst.Code)
	}
}

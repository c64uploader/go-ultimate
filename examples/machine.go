//go:build ignore

// Run: go run examples/machine.go
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

	// Power and run state

	_ = client.Machine.Reset(ctx) // hardware C64 reset
	time.Sleep(3 * time.Second)

	_ = client.Machine.Pause(ctx) // freeze the C64
	time.Sleep(2 * time.Second)

	_ = client.Machine.Resume(ctx) // let it run again
	time.Sleep(1 * time.Second)

	// Toggle the cartridge menu on, then off.
	_ = client.Machine.MenuButton(ctx)
	time.Sleep(2 * time.Second)
	_ = client.Machine.MenuButton(ctx)

	// client.Machine.Reboot(ctx)   // restart firmware — disruptive
	// client.Machine.PowerOff(ctx) // powers off the device

	// DMA memory access

	// Write to screen memory and read it back.
	_ = client.Machine.WriteMemory(ctx, 0x0400, c64.EncodeScreen("hello, ultimate!"))
	mem, _ := client.Machine.ReadMemory(ctx, 0x0400, 16)
	// EncodeScreen produces codes 1-26 for letters. After reset the C64
	// is in uppercase charset, so these render as uppercase on screen.
	fmt.Println(c64.DecodeScreen(mem, c64.CharsetUppercase))

	// Peek/poke: read border color, then turn it green.
	b, _ := client.Machine.Peek(ctx, 0xd020)
	fmt.Printf("Border color: %02X\n", b)
	_ = client.Machine.Poke(ctx, 0xd020, 0x05) // green
	time.Sleep(3 * time.Second)

	// Upload and inject a program at its load address (no reset).
	program, _ := c64.Assemble(`
		* = $1000
		lda #$05
		sta $d020
		rts
	`)
	_ = client.Machine.Inject(ctx, program)

	// Debug register (Ultimate 64)

	val, _ := client.Machine.ReadDebugRegister(ctx)
	fmt.Printf("debug register: %02X\n", val)
	_, _ = client.Machine.WriteDebugRegister(ctx, 0x42)

	// Bus measurement

	trace, _ := client.Machine.MeasureBus(ctx)
	fmt.Printf("VCD trace: %d bytes\n", len(trace))
}

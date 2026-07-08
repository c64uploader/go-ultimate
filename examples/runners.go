//go:build ignore

// Run: go run examples/runners.go
package main

import (
	"context"
	"time"

	"github.com/c64uploader/go-ultimate"
	"github.com/c64uploader/go-ultimate/c64"
	"github.com/c64uploader/go-ultimate/examples/utils"
)

func main() {
	client, _ := ultimate.New("c64u")
	ctx := context.Background()

	// This program sets the border color to green and returns to BASIC.
	// BASICHeader macro emits the "10 SYS nnnn" stub so that RUN will
	// jump to the label.
	program, _ := c64.Assemble(`
		* = $0801
		BASICHeader(entry)

		entry:
			lda #$05
			sta $d020
			rts
	`)

	// Run is a wrapper for RunPRGBytes that takes a *Program (from c64.Assemble),
	// resets the C64, uploads the program, and starts it.
	_ = client.Runners.Run(ctx, program)
	time.Sleep(3 * time.Second)

	// PlaySIDBytes uploads and plays a .SID music file from bytes (resets the C64 first).
	sid, _ := utils.HTTPGet("https://hvsc.perff.dk/MUSICIANS/H/Hubbard_Rob/Commando.sid")
	_ = client.Runners.PlaySIDBytes(ctx, sid, 0)
	time.Sleep(5 * time.Second)

	// RunPRGBytes uploads and runs a .PRG file from bytes (resets the C64 first).
	demo, _ := utils.HTTPGet("https://csdb.dk/getinternalfile.php/238376/tapesnake.prg")
	_ = client.Runners.RunPRGBytes(ctx, demo)

	// Run a .PRG or .CRT from the device filesystem:
	// _ = client.Runners.RunPRG(ctx, "/usb0/games/demo.prg")
	// _ = client.Runners.RunCRT(ctx, "/usb0/games/demo.crt")
}

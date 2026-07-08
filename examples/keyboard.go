//go:build ignore

// Run: go run examples/keyboard.go
package main

import (
	"context"
	"time"

	"github.com/c64uploader/go-ultimate"
	"github.com/c64uploader/go-ultimate/c64"
)

func main() {
	client, _ := ultimate.New("c64u")
	ctx := context.Background()

	client.Machine.Reboot(ctx)

	// 1. Type injects text into the KERNAL keyboard buffer ($0277).
	_ = client.Keyboard.Type(ctx, "PRINT \"HELLO\"\n")
	time.Sleep(2 * time.Second)

	// 2. Press uses the CIA #1 registers to simulate a key press, including multi-key chords.
	_ = client.Keyboard.Press(ctx, c64.KeyRunStop, c64.KeyLeftShift, c64.KeyA)
}

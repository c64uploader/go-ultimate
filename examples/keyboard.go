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
	// Works at the BASIC READY prompt or programs reading input via GETIN/CHRIN.
	_ = client.Keyboard.Type(ctx, "PRINT \"HELLO\"\n")
	time.Sleep(2 * time.Second)

	// 2. Press simulates holding keys down via KERNAL decode hooks and CIA #1 register overrides.
	// WARNING: Best-effort feature. Will fail in games or software that bypass KERNAL or use custom IRQ/input loops.
	_ = client.Keyboard.Press(ctx, c64.KeyRunStop, c64.KeyLeftShift, c64.KeyA)
}

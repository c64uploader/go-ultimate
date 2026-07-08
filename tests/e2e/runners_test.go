package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/c64uploader/go-ultimate/c64"
)

const songTimeout = 10 * time.Second

func TestE2E_RunnersRun(t *testing.T) {
	client, ctx := setupE2E(t)
	rebootAndReady(ctx, t, client)

	asm := `
		* = $0801
		BASICHeader(entry)

		entry:
			lda #$42
			sta $c000
			rts
	`
	prog, err := c64.Assemble(asm)
	if err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}

	if err := client.Runners.Run(ctx, prog); err != nil {
		t.Fatalf("Runners.Run failed: %v", err)
	}

	limit := time.Now().Add(10 * time.Second)
	for time.Now().Before(limit) {
		val, err := client.Machine.Peek(ctx, 0xC000)
		if err == nil && val == 0x42 {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}

	t.Errorf("Timeout waiting for program run to set $C000 to $42")
}

func TestE2E_Interactive_RunnersPlaySID(t *testing.T) {
	client, _ := setupE2E(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	rebootAndReady(ctx, t, client)

	t.Log("Downloading SID from Mod Archive...")
	sidBytes, err := downloadFile(ctx, t, "https://hvsc.csdb.dk/MUSICIANS/D/DRAX/Tristesse.sid")
	if err != nil {
		t.Fatalf("Failed to download SID: %v", err)
	}

	t.Log("Interactive SID Test: Starting SID playback...")
	err = client.Runners.PlaySIDBytes(ctx, sidBytes, 0)
	if err != nil {
		t.Fatalf("PlaySIDBytes failed: %v", err)
	}

	t.Logf("Interactive SID Test: Pausing for %v seconds to let user hear the music...", songTimeout.Seconds())
	select {
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	case <-time.After(songTimeout):
	}

	t.Log("Interactive SID Test: Returning back to C64...")
	rebootAndReady(ctx, t, client)

	passed, err := runInteractivePrompt(ctx, t, client, "\n\ndid you hear the sid playback?")
	if err != nil {
		t.Fatalf("Interactive prompt failed: %v", err)
	}

	if !passed {
		t.Error("SID audio verification failed: user reported not hearing the SID audio.")
	}
}

func TestE2E_Interactive_RunnersPlayMOD(t *testing.T) {
	client, _ := setupE2E(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	t.Log("Downloading MOD from Mod Archive...")
	modBytes, err := downloadFile(ctx, t, "https://api.modarchive.org/downloads.php?moduleid=164672")
	if err != nil {
		t.Fatalf("Failed to download MOD: %v", err)
	}

	// MOD player fails to start if running this test more than once:
	//   "Detecting Audio Module... FAIL"
	//
	// Workaround before the actual test:
	// The registers at $DF20+ (sample player)
	// keep old values after reboot. Direct writes only reach the hardware
	// after we start the thing that maps them.
	// So we start it once (to map), clear the registers, reboot, then do the real test.
	_ = client.Runners.PlayMODBytes(ctx, modBytes)
	_ = client.Machine.WriteMemory(ctx, 0xDF20, make([]byte, 512))
	rebootAndReady(ctx, t, client)

	t.Log("Interactive MOD Test: Starting Amiga MOD playback...")
	err = client.Runners.PlayMODBytes(ctx, modBytes)
	if err != nil {
		t.Fatalf("PlayMODBytes failed: %v", err)
	}

	t.Logf("Interactive MOD Test: Pausing for %v seconds to let user hear the music...", songTimeout.Seconds())
	select {
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	case <-time.After(songTimeout):
	}

	t.Log("Interactive MOD Test: Returning back to C64...")
	rebootAndReady(ctx, t, client)

	passed, err := runInteractivePrompt(ctx, t, client, "\n\ndid you hear the mod playback?")
	if err != nil {
		t.Fatalf("Interactive prompt failed: %v", err)
	}

	if !passed {
		t.Error("MOD audio verification failed: user reported not hearing the MOD audio.")
	}
}

func TestE2E_RunnersRunCRT(t *testing.T) {
	client, ctx := setupE2E(t)
	rebootAndReady(ctx, t, client)

	// We only need to provide the main application logic;
	// the vectors, signature, and system init are handled by the builder.
	asm := `
		* = $8040
		lda #$42
		sta $c000
		lda #$05
		sta $d020
	hold:
		jmp hold
	`

	t.Log("Building XIP cartridge...")
	prog, err := c64.Assemble(asm)
	if err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}
	crt, err := c64.NewXIPCartridge(c64.CRTNormal8K, "E2E_TEST", prog)
	if err != nil {
		t.Fatalf("NewXIPCartridge failed: %v", err)
	}

	_ = client.Machine.Poke(ctx, 0xC000, 0)

	t.Log("Running CRT cartridge...")
	if err := client.Runners.RunCRTBytes(ctx, crt); err != nil {
		t.Fatalf("RunCRTBytes failed: %v", err)
	}

	limit := time.Now().Add(10 * time.Second)
	for time.Now().Before(limit) {
		v, err := client.Machine.Peek(ctx, 0xC000)
		if err == nil && v == 0x42 {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
	t.Error("Timeout waiting for cartridge to set $C000 to $42")
}

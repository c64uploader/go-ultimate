// Inject keystrokes: Type via KERNAL buffer, Press via CIA matrix.

package ultimate

import (
	"context"
	"time"

	"github.com/c64uploader/go-ultimate/c64"
)

// keyHoldTime: how long the phantom row stays active when scnkey is not
// available. Long enough for polling programs (~2 PAL frames).
const keyHoldTime = 40 * time.Millisecond

// hookAddr: scratch RAM for the KERNAL decode hook. Inside the cassette
// buffer ($0380), safe when no tape is running.
const hookAddr = 0x0380

// KeyboardService injects keystrokes into the C64.
//
// WARNING:
// Keystroke injection is a best-effort software feature. Remote DMA access cannot
// bridge physical hardware matrix pins. Injection works reliably for BASIC and
// standard KERNAL software, but WILL FAIL in software that bypasses the KERNAL
// or uses custom keyboard scanning logic.
type KeyboardService struct {
	Mem MemoryReaderWriter // memory backend; defaults to Machine
}

type typeConfig struct {
	caseMode c64.KeysCase
}

// TypeOption configures Type.
type TypeOption func(*typeConfig)

// Literal makes Type distinguish 'a' from 'A' (lowercase vs uppercase on screen).
// Default folds case so both produce uppercase.
func Literal() TypeOption {
	return func(c *typeConfig) { c.caseMode = c64.LiteralCase }
}

// Type enqueues ASCII text into the KERNAL keyboard buffer ($0277).
//
// Use at the BASIC READY prompt or with programs reading input via KERNAL
// routines (GETIN/CHRIN).
//
// WARNING: Type does NOT work in software that reads hardware registers directly.
func (s *KeyboardService) Type(ctx context.Context, text string, opts ...TypeOption) error {
	cfg := typeConfig{caseMode: c64.FoldCase}
	for _, opt := range opts {
		opt(&cfg)
	}
	return s.feed(ctx, c64.EncodeKeys(text, cfg.caseMode))
}

// Press simulates holding one or more keyboard keys down.
//
// WARNING:
// Press is NOT reliable across all C64 software and WILL FAIL in many games, demos,
// and programs with custom IRQ handlers or non-standard input loops.
//
// How Press works:
//  1. KERNAL Decode Vector Hook ($028F): Intercepts the KERNAL SCNKEY routine during
//     IRQ scans to force the decoded matrix index ($CB).
//  2. CIA #1 DDRB Override ($DC03): Temporarily sets Port B pins as outputs driven low
//     for direct CIA reads.
//
// Why Press fails in non-KERNAL software:
//   - Disabled / Custom IRQs: If software turns off interrupts (SEI) or replaces the IRQ
//     handler ($0314), the $028F hook never runs. Press times out waiting for the key index
//     to register and restores CIA registers without the key being detected.
//   - Per-frame CIA register resets: Custom input loops that reset CIA DDRB ($DC03 = $00)
//     every frame immediately overwrite the injected key state.
//   - Column-specific matrix scanning: Custom routines querying matrix columns individually
//     expect row pins to go low only for specific column queries, which a static DDRB output
//     override cannot replicate.
func (s *KeyboardService) Press(ctx context.Context, keys ...c64.Key) error {
	if len(keys) == 0 {
		return nil
	}

	_, row := c64.CombineKeys(keys...)
	idx, shift := hookKey(keys)

	savedCIA, err := s.Mem.ReadMemory(ctx, c64.AddrCIA1PortA, 4)
	if err != nil {
		return err
	}

	procPort, _ := s.Mem.ReadMemory(ctx, 0x0001, 1)
	isKernalMapped := len(procPort) > 0 && (procPort[0]&0x07) >= 5

	var savedKeylog []byte

	// ── Path 1: KERNAL SCNKEY Hook via $028F ───────────────────────────
	if isKernalMapped {
		keylog, err := s.Mem.ReadMemory(ctx, c64.AddrKeylogVector, 2)
		if err == nil {
			savedKeylog = keylog

			// Assembly Trampoline at $0380 (Cassette Buffer):
			// 1. Force $028D (shift flags) and $CB (matrix index)
			// 2. Jump to original $028F vector target ($EB48)
			hook := []byte{
				0xA9, shift, // LDA #shift
				0x8D, byte(c64.AddrShiftFlag & 0xFF), byte(c64.AddrShiftFlag >> 8), // STA $028D
				0xA9, idx, // LDA #idx
				0x85, byte(c64.AddrMatrixIndex & 0xFF), // STA $CB
				0x4C, keylog[0], keylog[1], // JMP original_keylog
			}

			if err := s.Mem.WriteMemory(ctx, hookAddr, hook); err == nil {
				// Clear $C5 ($64 = No Key) to reset KERNAL debouncing.
				// This guarantees SCNKEY sees this as a new keypress event.
				_ = s.Mem.WriteMemory(ctx, c64.AddrLastKeyIndex, []byte{c64.NoKeyIndex})

				// Redirect $028F to our trampoline
				_ = s.Mem.WriteMemory(ctx, c64.AddrKeylogVector, []byte{byte(hookAddr & 0xFF), byte(hookAddr >> 8)})
			}
		}
	}

	// ── Path 2: CIA Direct / RAM Override ──────────────────────────────
	if len(procPort) > 0 && (procPort[0]&0x04) == 0 {
		// I/O mapped out: CPU reads RAM at $DC01 directly
		_ = s.Mem.WriteMemory(ctx, c64.AddrCIA1PortB, []byte{savedCIA[1] & row})
	} else {
		// I/O mapped in: Set DDRB bits as outputs to simulate phantom row
		activeBits := byte(^row & 0xFF)
		_ = s.Mem.WriteMemory(ctx, c64.AddrCIA1PortB, []byte{savedCIA[1] & row})
		_ = s.Mem.WriteMemory(ctx, c64.AddrCIA1DDRB, []byte{savedCIA[3] | activeBits})
	}

	// ── Hold & Release ──────────────────────────────────────────────────
	// Wait at least 2 full raster frames (40ms) so SCNKEY captures the state
	if err := s.holdUntilRegistered(ctx, idx, savedKeylog != nil); err != nil {
		return err
	}

	// Restore original $028F vector & CIA registers
	restoreCIA := s.Mem.WriteMemory(ctx, c64.AddrCIA1PortA, savedCIA)
	if savedKeylog != nil {
		_ = s.Mem.WriteMemory(ctx, c64.AddrKeylogVector, savedKeylog)
	}

	return restoreCIA
}

// holdUntilRegistered blocks until the key is registered or a timeout.
//
// With KERNAL hook installed: polls $C5 until it matches our matrix index.
// Without hook, or for modifier-only presses: waits a fixed time so
// polling programs can observe the phantom row.
func (s *KeyboardService) holdUntilRegistered(ctx context.Context, idx byte, hooked bool) error {
	if !hooked || idx == c64.NoKeyIndex {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(keyHoldTime):
			return nil
		}
	}
	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		b, err := s.Mem.ReadMemory(ctx, c64.AddrLastKeyIndex, 1)
		if err == nil && b[0] == idx {
			return nil
		}
		if time.Now().After(deadline) {
			return nil // give up waiting; release anyway
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Millisecond):
		}
	}
}

// hookKey splits a chord: shift flag from modifier keys, matrix index
// from the first non-modifier key.
func hookKey(keys []c64.Key) (idx, shift byte) {
	idx = c64.NoKeyIndex
	for _, k := range keys {
		if flag, ok := c64.ModifierFlag(k); ok {
			shift |= flag
		} else if idx == c64.NoKeyIndex {
			idx = c64.KeyMatrixIndex(k)
		}
	}
	return idx, shift
}

func (s *KeyboardService) feed(ctx context.Context, petscii []byte) error {
	for i := 0; i < len(petscii); i += c64.KernalKeyBufMax {
		end := min(i+c64.KernalKeyBufMax, len(petscii))
		chunk := petscii[i:end]

		for {
			lenBuf, err := s.Mem.ReadMemory(ctx, c64.AddrKernalKeyBufLen, 1)
			if err != nil {
				return err
			}
			if lenBuf[0] == 0 {
				break
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(20 * time.Millisecond):
			}
		}

		if err := s.Mem.WriteMemory(ctx, c64.AddrKernalKeyBuf, chunk); err != nil {
			return err
		}
		if err := s.Mem.WriteMemory(ctx, c64.AddrKernalKeyBufLen, []byte{byte(len(chunk))}); err != nil {
			return err
		}
	}
	return nil
}

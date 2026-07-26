// Inject keystrokes: Type via KERNAL buffer, Press via CIA matrix.

package ultimate

import (
	"context"
	"time"

	"github.com/c64uploader/go-ultimate/c64"
)

// keyHoldTime: how long the CIA matrix override is held for polling programs (~2 PAL frames).
const keyHoldTime = 40 * time.Millisecond

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

// matrixToPETSCII maps 8x8 key matrix index (0..63) to PETSCII key codes.
var matrixToPETSCII = [64]byte{
	0: 0x14, 1: 0x0D, 2: 0x1D, 3: 0x8A, 4: 0x85, 5: 0x87, 6: 0x89, 7: 0x91, // DEL, RET, CRSR L/R, F7, F1, F3, F5, CRSR D/U
	8: '3', 9: 'W', 10: 'A', 11: '4', 12: 'Z', 13: 'S', 14: 'E', 15: 0, // 15=LShift
	16: '5', 17: 'R', 18: 'D', 19: '6', 20: 'C', 21: 'F', 22: 'T', 23: 'X',
	24: '7', 25: 'Y', 26: 'G', 27: '8', 28: 'B', 29: 'H', 30: 'U', 31: 'V',
	32: '9', 33: 'I', 34: 'J', 35: '0', 36: 'M', 37: 'K', 38: 'O', 39: 'N',
	40: '+', 41: 'P', 42: 'L', 43: '-', 44: '.', 45: ':', 46: '@', 47: ',',
	48: 0x5C, 49: '*', 50: ';', 51: 0x13, 52: 0, 53: '=', 54: '^', 55: '/', // 48=£, 51=HOME, 52=RShift
	56: '1', 57: 0x5F, 58: 0, 59: '2', 60: ' ', 61: 0, 62: 'Q', 63: 0x03, // 57=←, 58=Ctrl, 60=Space, 61=Cbm, 63=RunStop
}

// Press simulates holding one or more keyboard keys down.
//
// The Ultimate REST API does not provide a built-in feature for simulating
// keyboard key presses. To work around this, Press attempts to simulate key presses
// in software using two different methods depending on the environment:
//
//  1. Standard C64 environment (BASIC prompt):
//     When the C64 KERNAL operating system is scanning the keyboard in the background,
//     Press writes the key's character code directly into the KERNAL keyboard buffer ($0277).
//  2. Custom software (Interrupts disabled):
//     When background IRQ scanning is disabled, Press modifies the CIA #1 hardware chip
//     registers ($DC00-$DC03) to simulate an active key matrix signal for 40ms before
//     restoring the original register values.
//
// Why this workaround may fail:
//   - Programs that reset CIA direction registers ($DC02/$DC03) every frame will overwrite
//     the injected key state.
//   - Routines that scan matrix columns individually expect selective row responses,
//     which a static CIA register write cannot replicate across all column scans.
func (s *KeyboardService) Press(ctx context.Context, keys ...c64.Key) error {
	if len(keys) == 0 {
		return nil
	}

	column, row := c64.CombineKeys(keys...)
	idx, shift := hookKey(keys)

	savedCIA, err := s.Mem.ReadMemory(ctx, c64.AddrCIA1PortA, 4)
	if err != nil {
		return err
	}

	// Check if KERNAL IRQ is actively running by observing jiffy clock ($00A2)
	jiffy1, err1 := s.Mem.ReadMemory(ctx, 0x00A2, 1)
	time.Sleep(20 * time.Millisecond)
	jiffy2, err2 := s.Mem.ReadMemory(ctx, 0x00A2, 1)
	irqRunning := err1 == nil && err2 == nil && jiffy1[0] != jiffy2[0]

	if irqRunning {
		// KERNAL SCNKEY is active: inject into KERNAL buffer ($0277) to avoid CIA register conflicts
		if idx != c64.NoKeyIndex {
			if petscii := matrixToPETSCII[idx]; petscii != 0 {
				if shift != 0 && petscii >= 'a' && petscii <= 'z' {
					petscii = petscii - 'a' + 'A'
				}
				_ = s.feed(ctx, []byte{petscii})
			}
		}
		return nil
	}

	// Interrupts disabled or non-KERNAL environment: drive CIA #1 registers directly
	if err := s.Mem.WriteMemory(ctx, c64.AddrCIA1DDRA, []byte{0xFF, 0xFF}); err != nil {
		return err
	}
	if err := s.Mem.WriteMemory(ctx, c64.AddrCIA1PortA, []byte{column, row}); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(keyHoldTime):
	}

	return s.Mem.WriteMemory(ctx, c64.AddrCIA1PortA, savedCIA)
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

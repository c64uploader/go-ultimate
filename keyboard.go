// Inject keystrokes: Type via KERNAL buffer, Press via CIA matrix.

package ultimate

import (
	"context"
	"errors"
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
// Use at the BASIC READY prompt or whenever the program reads via GETIN/CHRIN.
func (s *KeyboardService) Type(ctx context.Context, text string, opts ...TypeOption) error {
	cfg := typeConfig{caseMode: c64.FoldCase}
	for _, opt := range opts {
		opt(&cfg)
	}
	return s.feed(ctx, c64.EncodeKeys(text, cfg.caseMode))
}

// Press simulates holding one or more keyboard keys down.
//
// Programs read the keyboard in two ways:
//
//  1. CIA port reads
//     Write a column mask to Port A ($DC00), read rows from Port B
//     ($DC01). The KERNAL's scnkey routine does this every frame
//     from the IRQ handler and decodes the result into the keyboard
//     buffer ($0277) for CHRIN/GETIN.
//
//  2. Direct CIA reads (no KERNAL)
//     Demos, games, and cartridge software often cannot use the
//     KERNAL. Common reasons are:
//       - KERNAL ROM is banked out to free up $E000-$FFFF for RAM.
//       - Interrupts are disabled for stable raster effects, so
//         scnkey never fires.
//       - The program runs from cartridge in Ultimax mode, where
//         KERNAL is not mapped.
//     In all these cases the program must scan the CIA matrix
//     itself by writing to Port A / reading Port B directly.
//
// You cannot fake a press by writing to Port B — row pins are
// inputs. Press() uses two tricks to cover both paths:
//
//   - Phantom row (path 2): reconfigure row pins as outputs via
//     DDRB ($DC03) and drive them low. The row reads active in
//     every column.
//
//   - Decode hook (path 1): the KERNAL scnkey routine calls
//     through vector $028F between its matrix scan and its decode
//     step. Press() replaces this vector with a tiny routine that
//     writes the correct matrix index to $CB, then jumps back.
//     scnkey's own decode runs normally after that.
//
// Both tricks run at the same time, so software using either path
// sees the key press.
func (s *KeyboardService) Press(ctx context.Context, keys ...c64.Key) error {
	if len(keys) == 0 {
		return nil
	}

	// ── Prepare ──────────────────────────────────────────────────────────

	_, row := c64.CombineKeys(keys...)

	saved, err := s.Mem.ReadMemory(ctx, c64.AddrCIA1PortA, 4)
	if err != nil {
		return err
	}

	idx, shift := hookKey(keys)

	// ── KERNAL path: decode hook (scnkey) ───────────────────────────────

	// Only install the hook if the KERNAL IRQ handler ($EA31) is still
	// active. If a custom raster interrupt replaced $0314, scnkey is not
	// running and the hook would do nothing.
	var savedKeylog []byte
	if vec, err := s.Mem.ReadMemory(ctx, c64.AddrIRQVector, 2); err == nil &&
		vec[0] == byte(c64.KernalIRQEntry&0xFF) && vec[1] == byte(c64.KernalIRQEntry>>8) {
		keylog, err := s.Mem.ReadMemory(ctx, c64.AddrKeylogVector, 2)
		if err == nil {
			savedKeylog = keylog

			// Trampoline at $0380 (cassette buffer). When scnkey calls
			// $028F, this runs: set shflag + sfdx, then JMP original.
			hook := []byte{
				// LDA  #shift
				0xA9, shift,
				// STA  $028D        ; shflag = modifier bits
				0x8D, byte(c64.AddrShiftFlag&0xFF), byte(c64.AddrShiftFlag>>8),
				// LDA  #idx
				0xA9, idx,
				// STA  $CB          ; sfdx = matrix index for decode
				0x85, byte(c64.AddrMatrixIndex&0xFF),
				// JMP  original
				0x4C, keylog[0], keylog[1],
			}
			if err := s.Mem.WriteMemory(ctx, hookAddr, hook); err != nil {
				return err
			}

			// Clear last key index so scnkey treats this as a fresh press.
			_ = s.Mem.WriteMemory(ctx, c64.AddrLastKeyIndex, []byte{c64.NoKeyIndex})

			// Redirect $028F to our trampoline.
			err = s.Mem.WriteMemory(ctx, c64.AddrKeylogVector, []byte{byte(hookAddr & 0xFF), byte(hookAddr >> 8)})
			if err != nil {
				return err
			}
		}
	}

	// ── No-KERNAL path: phantom row (direct CIA reads) ──────────────────

	// Reconfigure row pins as outputs (DDRB) and drive them low (Port B).
	// Programs scanning columns see the row active in every column.
	if err := s.Mem.WriteMemory(ctx, c64.AddrCIA1PortB, []byte{saved[1] & row}); err != nil {
		return err
	}
	if err := s.Mem.WriteMemory(ctx, c64.AddrCIA1DDRB, []byte{saved[3] | ^row}); err != nil {
		return err
	}

	// ── Wait ─────────────────────────────────────────────────────────────

	if err := s.holdUntilRegistered(ctx, idx, savedKeylog != nil); err != nil {
		return err
	}

	// ── Release ──────────────────────────────────────────────────────────

	restore := s.Mem.WriteMemory(ctx, c64.AddrCIA1PortA, saved)
	if savedKeylog != nil {
		err = s.Mem.WriteMemory(ctx, c64.AddrKeylogVector, savedKeylog)
	}
	return errors.Join(restore, err)
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

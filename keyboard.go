// Inject keystrokes: Type via KERNAL buffer, Press via CIA matrix.

package ultimate

import (
	"context"
	"time"

	"github.com/c64uploader/go-ultimate/c64"
)

const keyHoldTime = 25 * time.Millisecond

// KeyboardService injects keystrokes into the C64.
type KeyboardService struct {
	client   *Client
	Mem      MemoryReaderWriter // memory backend; defaults to Machine
	savedCIA []byte             // $DC00-$DC03 during Press
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

// Press closes one or more matrix keys simultaneously via CIA #1 ($DC00/$DC01).
// Use for demos and games that read the keyboard matrix directly.
func (s *KeyboardService) Press(ctx context.Context, keys ...c64.Key) error {
	if len(keys) == 0 {
		return nil
	}
	column, row := c64.CombineKeys(keys...)
	saved, err := s.Mem.ReadMemory(ctx, c64.AddrCIA1PortA, 4)
	if err != nil {
		return err
	}
	s.savedCIA = saved
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
	return s.releaseCIA(ctx)
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

func (s *KeyboardService) releaseCIA(ctx context.Context) error {
	if len(s.savedCIA) != 4 {
		return nil
	}
	defer func() { s.savedCIA = nil }()
	return s.Mem.WriteMemory(ctx, c64.AddrCIA1PortA, s.savedCIA)
}

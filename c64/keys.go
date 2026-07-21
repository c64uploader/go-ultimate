// PETSCII encoding for KERNAL keyboard buffer and CHROUT.

package c64

import "github.com/c64uploader/go-ultimate/c64/codec"

// KeysCase controls how ASCII letters map to PETSCII keystrokes.
type KeysCase int

const (
	// FoldCase maps both 'a' and 'A' to uppercase on screen (default C64 mode).
	FoldCase KeysCase = iota
	// LiteralCase maps 'A' to uppercase and 'a' to lowercase on screen.
	LiteralCase
)

// EncodeKeys converts ASCII text to PETSCII bytes for the KERNAL keyboard buffer or CHROUT.
func EncodeKeys(s string, c KeysCase) []byte {
	out := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		out[i] = encodeKeysByte(s[i], c)
	}
	return out
}

func encodeKeysByte(b byte, c KeysCase) byte {
	switch {
	case b == '\r' || b == '\n':
		return 0x0D
	case b == '\b' || b == 0x7F:
		return 0x14
	case b >= 'a' && b <= 'z':
		if c == LiteralCase {
			return b - 'a' + 0x61
		}
		return b - 'a' + 'A'
	case b >= 'A' && b <= 'Z':
		return b
	default:
		if c == LiteralCase {
			return codec.PETSCII.EncodeByte(b)
		}
		return b
	}
}

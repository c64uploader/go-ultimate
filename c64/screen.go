// Screen code encoding for direct screen RAM ($0400) access.

package c64

import "strings"

// CharsetMode selects which half of the C64 Character ROM to decode against.
//
// The C64 Character ROM is 4K and holds two 2K character sets. $D018 bit 3
// selects which one is active. A screen-code byte renders as different glyphs
// depending on the choice.
//
//   CharsetLowercase ($D018 bit 3 = 1):
//     Screen codes 1-26  → lowercase a-z
//     Screen codes 65-90 → uppercase A-Z
//     97-122 + 128       → reverse-video variants
//
//   CharsetUppercase ($D018 bit 3 = 0):
//     Screen codes 1-26  → uppercase A-Z
//     Screen codes 65-90 → graphics glyphs (line-drawing, card suits, etc.)
//     97-122 + 128       → reverse-video variants
type CharsetMode int

const (
	CharsetLowercase CharsetMode = iota
	CharsetUppercase
)

// EncodeScreen converts ASCII text to C64 screen codes for direct screen RAM.
func EncodeScreen(s string) []byte {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b >= 'a' && b <= 'z' {
			b = b - 'a' + 'A'
		}
		switch {
		case b >= 'A' && b <= 'Z':
			out = append(out, b-'A'+1)
		case b == ' ':
			out = append(out, 32)
		case b >= 33 && b <= 63:
			out = append(out, b)
		default:
			out = append(out, b)
		}
	}
	return out
}

// DecodeScreen converts C64 screen codes to ASCII text using the given charset.
func DecodeScreen(data []byte, cs CharsetMode) string {
	var b strings.Builder
	b.Grow(len(data))
	for _, c := range data {
		b.WriteByte(screenCodeToChar(c, cs))
	}
	return b.String()
}

// screenCodeToChar maps a single C64 screen code to its ASCII character.
func screenCodeToChar(c byte, cs CharsetMode) byte {
	switch {
	case c == 0:
		return '@'
	case c >= 1 && c <= 26:
		if cs == CharsetLowercase {
			return 'a' + c - 1
		}
		return 'A' + c - 1
	case c == 27:
		return '['
	case c == 28:
		return '/'
	case c == 29:
		return ']'
	case c == 30:
		return '^'
	case c == 31:
		return '_'
	case c == 32:
		return ' '
	case c >= 33 && c <= 63:
		return c
	case c == 64:
		return '-'
	case c >= 65 && c <= 90:
		if cs == CharsetLowercase {
			return 'A' + c - 65
		}
		return '?' // graphics glyphs in uppercase charset
	case c == 91:
		return ':'
	case c == 92:
		return '/'
	case c == 93:
		return '='
	case c == 94:
		return '^'
	case c == 95:
		return '_'
	case c >= 97 && c <= 122:
		if cs == CharsetLowercase {
			return 'a' + c - 97
		}
		return 'A' + c - 97
	// Reverse video counterparts (c + 128)
	case c == 128:
		return '@'
	case c >= 129 && c <= 154:
		if cs == CharsetLowercase {
			return 'a' + c - 129
		}
		return 'A' + c - 129
	case c == 155:
		return '['
	case c == 156:
		return '/'
	case c == 157:
		return ']'
	case c == 158:
		return '^'
	case c == 159:
		return '_'
	case c == 160:
		return ' '
	case c >= 161 && c <= 191:
		return c - 128
	case c == 192:
		return '-'
	case c >= 193 && c <= 218:
		if cs == CharsetLowercase {
			return 'a' + c - 193
		}
		return 'A' + c - 193
	case c == 219:
		return ':'
	case c == 220:
		return '/'
	case c == 221:
		return '='
	case c == 222:
		return '^'
	case c == 223:
		return '_'
	case c >= 225 && c <= 250:
		if cs == CharsetLowercase {
			return 'a' + c - 225
		}
		return 'A' + c - 225
	default:
		return '.'
	}
}

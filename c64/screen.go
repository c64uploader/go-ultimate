// Screen code encoding for direct screen RAM ($0400) access.

package c64

import "strings"

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

// DecodeScreen converts C64 screen codes to ASCII text.
func DecodeScreen(data []byte) string {
	var b strings.Builder
	b.Grow(len(data))
	for _, c := range data {
		b.WriteByte(screenCodeToChar(c))
	}
	return b.String()
}

// screenCodeToChar maps a single C64 screen code to its nearest ASCII character.
func screenCodeToChar(c byte) byte {
	switch {
	case c == 0:
		return '@'
	case c >= 1 && c <= 26:
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
		return 'A' + c - 65
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
		return 'A' + c - 97
	// Reverse video counterparts (c + 128)
	case c == 128:
		return '@'
	case c >= 129 && c <= 154:
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
		return 'A' + c - 225
	default:
		return '.'
	}
}

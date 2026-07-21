// Package codec maps between C64 single-byte encodings: ASCII, PETSCII, and
// screen codes. Each pre-defined Codec stores a pair of conversion functions.
// Use Decode/Encode for batch conversion; DecodeByte/EncodeByte for single bytes.
package codec

// Codec converts between a source and target single-byte encoding.
type Codec struct {
	name   string
	decode func(byte) byte // maps source byte -> target byte
	encode func(byte) byte // maps target byte -> source byte (ASCII input)
}

// DecodeByte converts one byte from the source encoding to the target encoding.
func (c *Codec) DecodeByte(b byte) byte { return c.decode(b) }

// EncodeByte converts one byte from the target encoding to the source encoding.
func (c *Codec) EncodeByte(b byte) byte { return c.encode(b) }

// Decode fills dst with the decoded values of src. Returns bytes written.
func (c *Codec) Decode(dst, src []byte) int {
	for i, b := range src {
		dst[i] = c.decode(b)
	}
	return len(src)
}

// Encode fills dst with the encoded values of ASCII string s. Returns bytes written.
func (c *Codec) Encode(dst []byte, s string) int {
	for i := 0; i < len(s); i++ {
		dst[i] = c.encode(s[i])
	}
	return len(s)
}

// DecodeString decodes src to a string.
func (c *Codec) DecodeString(src []byte) string {
	b := make([]byte, len(src))
	for i, v := range src {
		b[i] = c.decode(v)
	}
	return string(b)
}

// EncodeString converts ASCII string s to the source encoding, returning a new slice.
func (c *Codec) EncodeString(s string) []byte {
	dst := make([]byte, len(s))
	c.Encode(dst, s)
	return dst
}

// String returns the codec name.
func (c *Codec) String() string { return c.name }

// CharsetMode selects which half of the C64 Character ROM is active.
//
// The C64 Character ROM is 4K and holds two 2K character sets. $D018 bit 1
// selects which one is active. A screen-code byte renders different glyphs
// depending on the charset.
//
//	CharsetLowercase ($D018 bit1 = 1):
//	  Screen codes 1-26  -> lowercase a-z
//	  Screen codes 65-90 -> uppercase A-Z
//
//	CharsetUppercase ($D018 bit1 = 0):
//	  Screen codes 1-26  -> uppercase A-Z
//	  Screen codes 65-90 -> graphics glyphs
type CharsetMode int

const (
	CharsetLowercase CharsetMode = iota
	CharsetUppercase
)

var (
	// PETSCII maps between PETSCII and ASCII with the standard case-swap
	// that the C64 KERNAL uses (BSOUT/CHROUT). PETSCII 65-90 (uppercase
	// A-Z) -> ASCII 97-122 (lowercase a-z). PETSCII 97-122 (lowercase
	// a-z) -> ASCII 65-90 (uppercase A-Z). CR and DEL are also mapped.
	//
	//   Decode: PETSCII -> ASCII
	//   Encode: ASCII  -> PETSCII
	PETSCII = &Codec{
		name:   "PETSCII",
		decode: decodePETSCII,
		encode: encodePETSCII,
	}

	// PETSCIIUpper maps between PETSCII and ASCII with no case-swapping
	// and no special-character translation. Used for filename encoding
	// (D64 headers) and uppercase-only string literals in assembly.
	//
	//   Decode: PETSCII -> ASCII
	//   Encode: ASCII  -> PETSCII (uppercase only)
	PETSCIIUpper = &Codec{
		name:   "PETSCII (uppercase)",
		decode: decodePETSCIIUpper,
		encode: encodePETSCIIUpper,
	}

	// ScreenLowercase maps between screen codes and ASCII assuming the
	// lowercase/uppercase charset ($D018 bit1=1). Screen codes 1-26
	// display lowercase a-z, screen codes 65-90 display uppercase A-Z.
	//
	//   Decode: screen -> ASCII
	//   Encode: ASCII  -> screen
	ScreenLowercase = &Codec{
		name:   "screen (lowercase charset)",
		decode: screenToASCIILowercase,
		encode: asciiToScreenLowercasewercase,
	}

	// ScreenUppercase maps between screen codes and ASCII assuming the
	// uppercase/graphics charset ($D018 bit1=0). Screen codes 1-26
	// display uppercase A-Z, screen codes 65-90 display graphics.
	//
	//   Decode: screen -> ASCII
	//   Encode: ASCII  -> screen
	ScreenUppercase = &Codec{
		name:   "screen (uppercase charset)",
		decode: screenToASCIIUppercase,
		encode: asciiToScreenUppercasepercase,
	}

	// ScreenPET maps between screen codes and PETSCII (charset-independent).
	// This is the reverse of the KERNAL's BSOUT PETSCII->screen-code conversion.
	// The reverse-video flag (bit 7) is preserved.
	//
	//   Decode: screen  -> PETSCII
	//   Encode: PETSCII -> screen
	ScreenPET = &Codec{
		name:   "screen <-> PETSCII",
		decode: screenCodeToPETSCII,
		encode: petsciiToScreenCode,
	}
)

// PETSCII <-> ASCII mapping.

func decodePETSCII(petscii byte) byte {
	switch {
	case petscii == 0x0D:
		return '\n'
	case petscii == 0x14:
		return '\b'
	case petscii >= 0x41 && petscii <= 0x5A: // 65-90 -> lowercase
		return petscii - 0x41 + 'a'
	case petscii >= 0x61 && petscii <= 0x7A: // 97-122 -> uppercase
		return petscii - 0x61 + 'A'
	default:
		return petscii
	}
}

func encodePETSCII(ascii byte) byte {
	switch {
	case ascii == '\r' || ascii == '\n':
		return 0x0D
	case ascii == '\b' || ascii == 0x7F:
		return 0x14
	case ascii >= 'a' && ascii <= 'z':
		return ascii - 'a' + 0x41
	case ascii >= 'A' && ascii <= 'Z':
		return ascii - 'A' + 0x61
	default:
		return ascii
	}
}

// Screen <-> ASCII mapping (lowercase charset).

func screenToASCIILowercase(c byte) byte { return screenCodeToChar(c, CharsetLowercase) }

func asciiToScreenLowercasewercase(a byte) byte {
	switch {
	case a >= 'a' && a <= 'z':
		return a - 'a' + 1 // screen code 1-26 (lowercase glyphs)
	case a >= 'A' && a <= 'Z':
		return a - 'A' + 65 // screen code 65-90 (uppercase glyphs)
	default:
		return asciiToScreenCommon(a)
	}
}

// Screen <-> ASCII mapping (uppercase charset).

func screenToASCIIUppercase(c byte) byte { return screenCodeToChar(c, CharsetUppercase) }

func asciiToScreenUppercasepercase(a byte) byte {
	switch {
	case a >= 'a' && a <= 'z', a >= 'A' && a <= 'Z':
		return a&0xDF - 'A' + 1 // both cases -> screen code 1-26 (uppercase glyphs)
	default:
		return asciiToScreenCommon(a)
	}
}

// asciiToScreenCommon handles punctuation and symbols (same mapping for both charsets).
func asciiToScreenCommon(a byte) byte {
	switch {
	case a == '@':
		return 0
	case a == '[':
		return 27
	case a == '/':
		return 28
	case a == ']':
		return 29
	case a == '^':
		return 30
	case a == '_':
		return 31
	case a == ' ':
		return 32
	case a >= 33 && a <= 63:
		return a
	case a == '-':
		return 64
	case a == ':':
		return 91
	case a == '=':
		return 93
	default:
		return a
	}
}

// Screen <-> PETSCII mapping (charset-independent).

// screenCodeToPETSCII converts a screen code to its PETSCII byte.
// This is the reverse of the KERNAL's BSOUT PETSCII->screen-code conversion.
func screenCodeToPETSCII(c byte) byte {
	low := c & 0x7F
	var petscii byte

	switch {
	case low == 0:
		petscii = 0
	case low >= 1 && low <= 26:
		petscii = low + 64 // 1->65 (A), 26->90 (Z)
	case low >= 27 && low <= 31:
		petscii = low + 64 // 27->91, 31->95
	case low == 32:
		petscii = 32
	case low >= 33 && low <= 63:
		petscii = low
	case low == 64:
		petscii = 64
	case low >= 65 && low <= 90:
		petscii = low + 32 // 65->97 (a), 90->122 (z)
	case low >= 91 && low <= 95:
		petscii = low + 32 // 91->123, 95->127
	case low >= 96 && low <= 127:
		petscii = low
	default:
		petscii = low
	}

	if c&0x80 != 0 {
		petscii |= 0x80
	}
	return petscii
}

// petsciiToScreenCode converts a PETSCII byte to its screen code.
// This is the same as the KERNAL's BSOUT PETSCII->screen-code conversion.
func petsciiToScreenCode(p byte) byte {
	low := p & 0x7F
	var sc byte

	switch {
	case low >= 65 && low <= 90: // PETSCII uppercase A-Z
		sc = low - 64 // 65->1, 90->26
	case low >= 97 && low <= 122: // PETSCII lowercase a-z
		sc = low - 32 // 97->65, 122->90
	case low >= 91 && low <= 95:
		sc = low - 64 // 91->27, 95->31
	case low >= 123 && low <= 127:
		sc = low - 32 // 123->91, 127->95
	default:
		sc = low
	}

	if p&0x80 != 0 {
		sc |= 0x80
	}
	return sc
}

// PETSCII (uppercase-only) <-> ASCII mapping.

func decodePETSCIIUpper(p byte) byte { return p }

func encodePETSCIIUpper(a byte) byte {
	switch {
	case a == '\r' || a == '\n':
		return 0x0D
	case a == '\b' || a == 0x7F:
		return 0x14
	case a >= 'a' && a <= 'z':
		return a - 'a' + 'A'
	default:
		return a
	}
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

// PETSCII ↔ ASCII single-byte conversion.

package c64

// EncodePETSCII maps one ASCII byte to PETSCII (swaps letter case, maps CR/BS).
func EncodePETSCII(ascii byte) byte {
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

// EncodePETSCIIUpper maps one ASCII byte to standard uppercase PETSCII (no case-swapping).
func EncodePETSCIIUpper(ascii byte) byte {
	switch {
	case ascii == '\r' || ascii == '\n':
		return 0x0D
	case ascii == '\b' || ascii == 0x7F:
		return 0x14
	case ascii >= 'a' && ascii <= 'z':
		return ascii - 'a' + 'A'
	default:
		return ascii
	}
}

// DecodePETSCII maps one PETSCII byte to ASCII.
func DecodePETSCII(petscii byte) byte {
	switch {
	case petscii == 0x0D:
		return '\n'
	case petscii == 0x14:
		return '\b'
	case petscii >= 0x41 && petscii <= 0x5A:
		return petscii - 0x41 + 'a'
	case petscii >= 0x61 && petscii <= 0x7A:
		return petscii - 0x61 + 'A'
	default:
		return petscii
	}
}

package c64

// Program is a C64 .PRG file: 2-byte little-endian load address plus payload.
// Use Bytes() for upload/run APIs; Code() for cartridge ROM or DMA inject.
type Program struct {
	prg []byte
}

// NewProgram creates a Program from code bytes loaded at the given address.
func NewProgram(code []byte, address uint16) *Program {
	prg := make([]byte, 2+len(code))
	prg[0] = byte(address)
	prg[1] = byte(address >> 8)
	copy(prg[2:], code)
	return &Program{prg: prg}
}

// NewProgramFromPRG parses raw PRG file bytes (2-byte header + payload) into a Program.
// Data shorter than 2 bytes yields an empty program.
func NewProgramFromPRG(data []byte) *Program {
	if len(data) < 2 {
		return &Program{}
	}
	p := make([]byte, len(data))
	copy(p, data)
	return &Program{prg: p}
}

// Bytes returns the full PRG (load address + payload).
func (p *Program) Bytes() []byte {
	if p == nil || len(p.prg) == 0 {
		return nil
	}
	out := make([]byte, len(p.prg))
	copy(out, p.prg)
	return out
}

// Code returns the payload after the 2-byte load-address header.
func (p *Program) Code() []byte {
	if p == nil || len(p.prg) <= 2 {
		return nil
	}
	out := make([]byte, len(p.prg)-2)
	copy(out, p.prg[2:])
	return out
}

// Size returns the code length in bytes (excluding the PRG header).
func (p *Program) Size() int {
	if p == nil || len(p.prg) <= 2 {
		return 0
	}
	return len(p.prg) - 2
}

// LoadAddress returns the 16-bit load address from the PRG header.
func (p *Program) LoadAddress() uint16 {
	if p == nil || len(p.prg) < 2 {
		return 0
	}
	return uint16(p.prg[0]) | uint16(p.prg[1])<<8
}

// Disassemble decodes the program payload starting at LoadAddress.
func (p *Program) Disassemble() []Instruction {
	code := p.Code()
	if len(code) == 0 {
		return nil
	}
	return Disassemble(code, p.LoadAddress())
}


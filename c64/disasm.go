// 6502 instruction decoding.

package c64

import (
	"fmt"
)

// Instruction is one decoded 6502 instruction at a memory address.
type Instruction struct {
	Address uint16 `json:"address"` // memory address of the opcode
	Bytes   []byte `json:"bytes"`   // raw instruction bytes (1–3)
	Code    string `json:"code"`    // assembler-style text (e.g. "LDA #$01")
}

// opInfo describes a 6502 opcode: its mnemonic, addressing mode, and byte size.
type opInfo struct {
	mnemonic string
	mode     string
	size     int
}

// Disassemble decodes data as 6502 instructions starting at startAddr.
// Unknown or truncated bytes are emitted as ".byte $XX" pseudo-instructions.
func Disassemble(data []byte, startAddr uint16) []Instruction {
	var instructions []Instruction
	pos := 0

	for pos < len(data) {
		addr := startAddr + uint16(pos)
		op := data[pos]
		info := opcodes[op]
		size := info.size
		if size == 0 {
			size = 1
		}

		// Too few bytes remain for this instruction — emit the rest as .byte entries.
		if pos+size > len(data) {
			for pos < len(data) {
				instructions = append(instructions, Instruction{
					Address: startAddr + uint16(pos),
					Bytes:   []byte{data[pos]},
					Code:    fmt.Sprintf(".byte $%02X", data[pos]),
				})
				pos++
			}
			break
		}

		instBytes := data[pos : pos+size]
		var code string

		if info.mnemonic == "" {
			code = fmt.Sprintf(".byte $%02X", op)
		} else {
			switch info.mode {
			case "acc":
				code = fmt.Sprintf("%s A", info.mnemonic)
			case "impl":
				code = info.mnemonic
			case "imm":
				code = fmt.Sprintf("%s #$%02X", info.mnemonic, instBytes[1])
			case "zp":
				code = fmt.Sprintf("%s $%02X", info.mnemonic, instBytes[1])
			case "zpx":
				code = fmt.Sprintf("%s $%02X,X", info.mnemonic, instBytes[1])
			case "zpy":
				code = fmt.Sprintf("%s $%02X,Y", info.mnemonic, instBytes[1])
			case "abs":
				val := uint16(instBytes[1]) | uint16(instBytes[2])<<8
				code = fmt.Sprintf("%s $%04X", info.mnemonic, val)
			case "absx":
				val := uint16(instBytes[1]) | uint16(instBytes[2])<<8
				code = fmt.Sprintf("%s $%04X,X", info.mnemonic, val)
			case "absy":
				val := uint16(instBytes[1]) | uint16(instBytes[2])<<8
				code = fmt.Sprintf("%s $%04X,Y", info.mnemonic, val)
			case "ind":
				val := uint16(instBytes[1]) | uint16(instBytes[2])<<8
				code = fmt.Sprintf("%s ($%04X)", info.mnemonic, val)
			case "indx":
				code = fmt.Sprintf("%s ($%02X,X)", info.mnemonic, instBytes[1])
			case "indy":
				code = fmt.Sprintf("%s ($%02X),Y", info.mnemonic, instBytes[1])
			case "rel":
				offset := int8(instBytes[1])
				target := uint16(int32(addr) + 2 + int32(offset))
				code = fmt.Sprintf("%s $%04X", info.mnemonic, target)
			}
		}

		instructions = append(instructions, Instruction{
			Address: addr,
			Bytes:   instBytes,
			Code:    code,
		})
		pos += size
	}

	return instructions
}

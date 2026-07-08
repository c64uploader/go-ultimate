package c64

import (
	"reflect"
	"testing"
)

func TestDisassemble(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		startAddr uint16
		want      []Instruction
	}{
		{
			name:      "LDA immediate and STA absolute",
			data:      []byte{0xA9, 0x01, 0x8D, 0x20, 0xD0, 0x60},
			startAddr: 0x1000,
			want: []Instruction{
				{Address: 0x1000, Bytes: []byte{0xA9, 0x01}, Code: "LDA #$01"},
				{Address: 0x1002, Bytes: []byte{0x8D, 0x20, 0xD0}, Code: "STA $D020"},
				{Address: 0x1005, Bytes: []byte{0x60}, Code: "RTS"},
			},
		},
		{
			name:      "BNE relative branch",
			data:      []byte{0xD0, 0xFB}, // offset -5 → target = $1003+2-5 = $1000
			startAddr: 0x1003,
			want: []Instruction{
				{Address: 0x1003, Bytes: []byte{0xD0, 0xFB}, Code: "BNE $1000"},
			},
		},
		{
			name:      "Incomplete and Unknown opcodes",
			data:      []byte{0xEA, 0xFF, 0xA9}, // NOP, unknown 0xFF, incomplete LDA imm
			startAddr: 0x1000,
			want: []Instruction{
				{Address: 0x1000, Bytes: []byte{0xEA}, Code: "NOP"},
				{Address: 0x1001, Bytes: []byte{0xFF}, Code: ".byte $FF"},
				{Address: 0x1002, Bytes: []byte{0xA9}, Code: ".byte $A9"},
			},
		},

		// ── Bug #10 fix: accumulator mode disassembly ─────────────────────────────────
		{
			name:      "Accumulator mode instructions ASL LSR ROL ROR",
			data:      []byte{0x0A, 0x4A, 0x2A, 0x6A},
			startAddr: 0x1000,
			want: []Instruction{
				{Address: 0x1000, Bytes: []byte{0x0A}, Code: "ASL A"},
				{Address: 0x1001, Bytes: []byte{0x4A}, Code: "LSR A"},
				{Address: 0x1002, Bytes: []byte{0x2A}, Code: "ROL A"},
				{Address: 0x1003, Bytes: []byte{0x6A}, Code: "ROR A"},
			},
		},

		// ── Addressing mode coverage ──────────────────────────────────────────────────
		{
			name:      "Zero-page and zero-page indexed X/Y",
			data:      []byte{0xA5, 0x10, 0xB5, 0x20, 0xB6, 0x30},
			startAddr: 0x1000,
			want: []Instruction{
				{Address: 0x1000, Bytes: []byte{0xA5, 0x10}, Code: "LDA $10"},
				{Address: 0x1002, Bytes: []byte{0xB5, 0x20}, Code: "LDA $20,X"},
				{Address: 0x1004, Bytes: []byte{0xB6, 0x30}, Code: "LDX $30,Y"},
			},
		},
		{
			name:      "Absolute and absolute indexed X/Y",
			data:      []byte{0xAD, 0x00, 0xC0, 0xBD, 0x00, 0xC0, 0xB9, 0x00, 0xC0},
			startAddr: 0x1000,
			want: []Instruction{
				{Address: 0x1000, Bytes: []byte{0xAD, 0x00, 0xC0}, Code: "LDA $C000"},
				{Address: 0x1003, Bytes: []byte{0xBD, 0x00, 0xC0}, Code: "LDA $C000,X"},
				{Address: 0x1006, Bytes: []byte{0xB9, 0x00, 0xC0}, Code: "LDA $C000,Y"},
			},
		},
		{
			name:      "Indirect indexed X and indirect indexed Y",
			data:      []byte{0xA1, 0x10, 0xB1, 0x20},
			startAddr: 0x1000,
			want: []Instruction{
				{Address: 0x1000, Bytes: []byte{0xA1, 0x10}, Code: "LDA ($10,X)"},
				{Address: 0x1002, Bytes: []byte{0xB1, 0x20}, Code: "LDA ($20),Y"},
			},
		},
		{
			name:      "JMP absolute and JMP indirect",
			data:      []byte{0x4C, 0x00, 0x10, 0x6C, 0x00, 0x20},
			startAddr: 0x1000,
			want: []Instruction{
				{Address: 0x1000, Bytes: []byte{0x4C, 0x00, 0x10}, Code: "JMP $1000"},
				{Address: 0x1003, Bytes: []byte{0x6C, 0x00, 0x20}, Code: "JMP ($2000)"},
			},
		},
		{
			name:      "JSR absolute",
			data:      []byte{0x20, 0x00, 0xFF},
			startAddr: 0x1000,
			want: []Instruction{
				{Address: 0x1000, Bytes: []byte{0x20, 0x00, 0xFF}, Code: "JSR $FF00"},
			},
		},
		{
			name:      "Positive relative branch forward",
			data:      []byte{0xF0, 0x05}, // BEQ +5 → target = addr+2+5
			startAddr: 0x1000,
			want: []Instruction{
				{Address: 0x1000, Bytes: []byte{0xF0, 0x05}, Code: "BEQ $1007"},
			},
		},

		// ── Edge cases ────────────────────────────────────────────────────────────────
		{
			name:      "Empty input returns nil",
			data:      []byte{},
			startAddr: 0x1000,
			want:      nil,
		},
		{
			name:      "Single NOP",
			data:      []byte{0xEA},
			startAddr: 0x0000,
			want: []Instruction{
				{Address: 0x0000, Bytes: []byte{0xEA}, Code: "NOP"},
			},
		},
		{
			name:      "Trailing incomplete 3-byte instruction emitted as two .byte entries",
			data:      []byte{0xEA, 0xAD, 0x00}, // NOP, then incomplete LDA abs (needs 3 bytes)
			startAddr: 0x1000,
			want: []Instruction{
				{Address: 0x1000, Bytes: []byte{0xEA}, Code: "NOP"},
				{Address: 0x1001, Bytes: []byte{0xAD}, Code: ".byte $AD"},
				{Address: 0x1002, Bytes: []byte{0x00}, Code: ".byte $00"},
			},
		},
		{
			name:      "startAddr near $FFFF wraps correctly (6502 behaviour)",
			data:      []byte{0xEA, 0xEA},
			startAddr: 0xFFFE,
			want: []Instruction{
				{Address: 0xFFFE, Bytes: []byte{0xEA}, Code: "NOP"},
				{Address: 0xFFFF, Bytes: []byte{0xEA}, Code: "NOP"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Disassemble(tc.data, tc.startAddr)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got:  %+v\nwant: %+v", got, tc.want)
			}
		})
	}
}

func TestOpcodeTableConsistency(t *testing.T) {
	seen := map[string]byte{}
	for op, info := range opcodes {
		if info.mnemonic == "" {
			if info.size != 0 {
				t.Errorf("opcode $%02X: unknown opcode has size %d", op, info.size)
			}
			continue
		}
		if info.size < 1 || info.size > 3 {
			t.Errorf("opcode $%02X (%s/%s): invalid size %d", op, info.mnemonic, info.mode, info.size)
		}
		key := info.mnemonic + "/" + info.mode
		if prev, ok := seen[key]; ok {
			t.Errorf("duplicate %s: $%02X and $%02X", key, prev, op)
			continue
		}
		seen[key] = byte(op)
		if rev, ok := lookupOpcode(info.mnemonic, info.mode); !ok || rev != byte(op) {
			t.Errorf("opcode $%02X (%s/%s): lookupOpcode returned $%02X, ok=%v", op, info.mnemonic, info.mode, rev, ok)
		}
	}
}

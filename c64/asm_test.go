package c64

import (
	"reflect"
	"strings"
	"testing"
)

func TestAssemble(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		want    []byte
		wantErr bool
	}{
		{
			name: "Simple LDA and STA with origin",
			source: `
				* = $1000
				LDA #$01
				STA $D020
				RTS
			`,
			want: []byte{
				0x00, 0x10, // PRG load address $1000
				0xA9, 0x01, // LDA #$01
				0x8D, 0x20, 0xD0, // STA $D020
				0x60, // RTS
			},
		},
		{
			name: "Constants and labels with relative branch",
			source: `
				border = $D020
				* = $1000
				LDX #$05
				loop:
				DEX
				BNE loop
				STX border
				RTS
			`,
			want: []byte{
				0x00, 0x10, // PRG load address $1000
				0xA2, 0x05, // LDX #$05
				// loop PC is $1002
				0xCA,       // DEX (PC $1002, size 1)
				0xD0, 0xFD, // BNE loop: offset $1002-($1003+2)=-3=$FD
				0x8E, 0x20, 0xD0, // STX border ($D020) — abs mode, border>$FF
				0x60, // RTS
			},
		},
		{
			name:    "Invalid constant syntax",
			source:  "label = invalid",
			wantErr: true,
		},
		{
			name: "Relative branch out of range",
			source: `
				* = $1000
				loop:
				.org $1100
				BNE loop
			`,
			wantErr: true,
		},

		// ── Bug #10 fix: accumulator mode ─────────────────────────────────────────────
		{
			name: "Accumulator mode explicit A suffix",
			source: `
				* = $1000
				ASL A
				LSR A
				ROL A
				ROR A
			`,
			want: []byte{
				0x00, 0x10, // load $1000
				0x0A, // ASL A
				0x4A, // LSR A
				0x2A, // ROL A
				0x6A, // ROR A
			},
		},
		{
			name: "Accumulator mode implicit (no A suffix)",
			source: `
				* = $1000
				ASL
				LSR
				ROL
				ROR
			`,
			want: []byte{
				0x00, 0x10,
				0x0A, 0x4A, 0x2A, 0x6A,
			},
		},

		// ── Bug #1/#6 fix: zero-page mode for backward label references ───────────────
		{
			name: "Backward label resolved to zero page addressing",
			source: `
				zp_var = $42
				* = $1000
				LDA zp_var
				STA zp_var
			`,
			want: []byte{
				0x00, 0x10,
				0xA5, 0x42, // LDA zp   (NOT 0xAD for abs)
				0x85, 0x42, // STA zp
			},
		},
		{
			name: "Non-zero-page backward label uses absolute addressing",
			source: `
				abs_var = $D020
				* = $1000
				LDA abs_var
			`,
			want: []byte{
				0x00, 0x10,
				0xAD, 0x20, 0xD0, // LDA abs
			},
		},

		// ── Bug #4 fix: data directives ───────────────────────────────────────────────
		{
			name: ".byte directive",
			source: `
				* = $1000
				.byte $01, $02, $03
				RTS
			`,
			want: []byte{
				0x00, 0x10,
				0x01, 0x02, 0x03,
				0x60,
			},
		},
		{
			name: ".byte directive with label",
			source: `
				* = $1000
				msg: .byte $48, $49
			`,
			want: []byte{
				0x00, 0x10,
				0x48, 0x49,
			},
		},
		{
			name: ".word directive",
			source: `
				* = $1000
				.word $1234, $5678
			`,
			want: []byte{
				0x00, 0x10,
				0x34, 0x12, // $1234 LE
				0x78, 0x56, // $5678 LE
			},
		},
		{
			name: ".word with label reference",
			source: `
				* = $1000
				.word target
				NOP
				target: RTS
			`,
			// .word = 2 bytes at $1000-$1001, NOP at $1002, target: at $1003
			want: []byte{
				0x00, 0x10,
				0x03, 0x10, // word $1003
				0xEA, // NOP at $1002
				0x60, // RTS at $1003
			},
		},

		// ── Bug #3 fix: low/high byte extraction operators ────────────────────────────
		{
			name: "Low/high byte extraction #< and #>",
			source: `
				target = $1234
				* = $1000
				LDA #<target
				LDA #>target
			`,
			want: []byte{
				0x00, 0x10,
				0xA9, 0x34, // LDA #$34 (low byte of $1234)
				0xA9, 0x12, // LDA #$12 (high byte of $1234)
			},
		},
		{
			name: "Low byte extraction makes immediate ZP-sized",
			source: `
				addr = $D020
				* = $1000
				LDA #<addr
			`,
			want: []byte{
				0x00, 0x10,
				0xA9, 0x20, // LDA #$20
			},
		},

		// ── Multi-.org with gap padding ───────────────────────────────────────────────
		{
			name: "Multi .org with gap padding",
			source: `
				* = $1000
				NOP
				* = $1003
				RTS
			`,
			want: []byte{
				0x00, 0x10,
				0xEA,       // NOP at $1000
				0x00, 0x00, // padding $1001–$1002
				0x60, // RTS at $1003
			},
		},

		// ── Duplicate label errors ────────────────────────────────────────────────────
		{
			name:    "Duplicate constant definition",
			source:  "foo = $10\nfoo = $20",
			wantErr: true,
		},
		{
			name: "Duplicate code label",
			source: `
				* = $1000
				loop: NOP
				loop: RTS
			`,
			wantErr: true,
		},

		// ── Indirect addressing modes ─────────────────────────────────────────────────
		{
			name: "Indirect indexed X and Y addressing",
			source: `
				* = $1000
				LDA ($10,X)
				LDA ($20),Y
			`,
			want: []byte{
				0x00, 0x10,
				0xA1, 0x10, // LDA ($10,X)
				0xB1, 0x20, // LDA ($20),Y
			},
		},
		{
			name: "JMP indirect",
			source: `
				* = $1000
				JMP ($1234)
			`,
			want: []byte{
				0x00, 0x10,
				0x6C, 0x34, 0x12, // JMP ($1234)
			},
		},

		// ── Case-insensitive index registers ──────────────────────────────────────────
		{
			name: "Label indexed with lowercase x and y",
			source: `
				* = $1000
				msg:
				.byte $41, 0
				ldy #$00
			loop:
				lda msg,y
				beq done
				iny
				bne loop
			done:
				ldx #$00
				lda msg,x
				rts
			`,
			want: []byte{
				0x00, 0x10,
				0x41, 0x00, // msg
				0xA0, 0x00, // LDY #$00 at $1002
				0xB9, 0x00, 0x10, // LDA msg,Y
				0xF0, 0x03, // BEQ done
				0xC8,       // INY
				0xD0, 0xF8, // BNE loop
				0xA2, 0x00, // LDX #$00
				0xBD, 0x00, 0x10, // LDA msg,X
				0x60, // RTS
			},
		},
		{
			name: "Indirect indexed with lowercase x and y",
			source: `
				* = $1000
				lda ($10,x)
				lda ($20),y
			`,
			want: []byte{
				0x00, 0x10,
				0xA1, 0x10, // LDA ($10,X)
				0xB1, 0x20, // LDA ($20),Y
			},
		},

		// ── Whitespace normalisation in operands ──────────────────────────────────────
		{
			name: "Operand with extra whitespace normalised",
			source: `
				* = $1000
				LDA ( $10 , X )
				LDA ( $20 ) , Y
			`,
			want: []byte{
				0x00, 0x10,
				0xA1, 0x10, // LDA ($10,X)
				0xB1, 0x20, // LDA ($20),Y
			},
		},

		// ── Numeric literal formats ───────────────────────────────────────────────────
		{
			name: "Binary literal operand",
			source: `
				* = $1000
				LDA #%00000001
			`,
			want: []byte{0x00, 0x10, 0xA9, 0x01},
		},
		{
			name: "Hex 0x prefix literal",
			source: `
				* = $1000
				LDA #0x42
			`,
			want: []byte{0x00, 0x10, 0xA9, 0x42},
		},

		// ── Edge cases ────────────────────────────────────────────────────────────────
		{
			name:   "Empty source produces PRG header only",
			source: "",
			want:   []byte{0x01, 0x08}, // default load address $0801
		},
		{
			name:   "Comment-only source",
			source: "; just a comment\n; another comment",
			want:   []byte{0x01, 0x08},
		},
		{
			name: ".org directive syntax",
			source: `
				.org $2000
				NOP
			`,
			want: []byte{0x00, 0x20, 0xEA},
		},

		// ── BASICHeader ───────────────────────────────────────────────────────────
		{
			name: "BASICHeader produces classic 10 SYS stub at $0801",
			source: `
				* = $0801
				BASICHeader(entry)
				entry:
					lda #$05
					sta $d020
					rts
			`,
			// Load $0801 + 12-byte stub (SYS 2061) + 6 bytes code
			want: []byte{
				0x01, 0x08, // PRG header $0801
				// stub for SYS 2061 ($080d)
				0x0b, 0x08, 0x0a, 0x00, 0x9e, '2', '0', '6', '1', 0x00, 0x00, 0x00,
				// code at $080d
				0xa9, 0x05, 0x8d, 0x20, 0xd0, 0x60,
			},
		},

		// ── Case-insensitive labels and constants ───────────────────────────────────
		{
			name: "Case-insensitive label and constant references",
			source: `
				Target = $1234
				* = $1000
				JMP TARGET
			Loop:
				NOP
				BNE loop
				JMP Done
			Done:
				RTS
			`,
			want: []byte{
				0x00, 0x10,
				0x4C, 0x34, 0x12, // JMP $1234 (TARGET)
				0xEA,       // NOP at Loop ($1003)
				0xD0, 0xFD, // BNE loop (back to $1003)
				0x4C, 0x09, 0x10, // JMP Done (forward label case-insensitive)
				0x60, // RTS at Done ($1009)
			},
		},

		// ── String literals in data directives ──────────────────────────────────────
		{
			name: ".text uses default screencode_mixed encoding",
			source: `
				* = $1000
				.text "Hello"
			`,
			want: []byte{
				0x00, 0x10,
				72, 5, 12, 12, 15, // H=65+7=72, e=5, l=12, l=12, o=15 (mixed: A-Z→65-90, a-z→1-26)
			},
		},
		{
			name: ".text screencode_upper folds case",
			source: `
				* = $1000
				.encoding "screencode_upper"
				.text "Hello"
			`,
			want: []byte{
				0x00, 0x10,
				8, 5, 12, 12, 15, // all folded to uppercase screen codes 1-26
			},
		},
		{
			name: ".text screencode_mixed preserves case",
			source: `
				* = $1000
				.encoding "screencode_mixed"
				.text "Hi"
			`,
			want: []byte{
				0x00, 0x10,
				72, 9, // H='A'-'A'+65=72, i='i'-'a'+1=9
			},
		},
		{
			name: ".text petscii_upper encodes for KERNAL",
			source: `
				* = $1000
				.encoding "petscii_upper"
				.text "hello\n"
			`,
			want: []byte{
				0x00, 0x10,
				'H', 'E', 'L', 'L', 'O', 0x0D,
			},
		},
		{
			name: ".text petscii_mixed swaps case",
			source: `
				* = $1000
				.encoding "petscii_mixed"
				.text "Hi"
			`,
			want: []byte{
				0x00, 0x10,
				0x68, 0x49, // 'H'→0x61+7=0x68, 'i'→0x41+8=0x49
			},
		},
		{
			name: ".byte uses current encoding for strings",
			source: `
				* = $1000
				.encoding "screencode_upper"
				.byte "AB", 0
			`,
			want: []byte{0x00, 0x10, 1, 2, 0},
		},
		{
			name: ".byte mixed strings and numbers",
			source: `
				* = $1000
				.encoding "screencode_upper"
				.byte $41, "BC", $43, 0
			`,
			want: []byte{0x00, 0x10, 0x41, 2, 3, 0x43, 0x00},
		},
		{
			name: ".encoding switches mid-file",
			source: `
				* = $1000
				.encoding "screencode_upper"
				.text "A"
				.encoding "petscii_upper"
				.text "A"
			`,
			want: []byte{0x00, 0x10, 1, 'A'},
		},
		{
			name: "string with comma inside quotes",
			source: `
				* = $1000
				.encoding "screencode_upper"
				.byte "a,b", 0
			`,
			want: []byte{0x00, 0x10, 1, 0x2C, 2, 0}, // a→1, ','→0x2C, b→2
		},
		{
			name: "quoted string with spaces preserved in operand",
			source: `
				* = $1000
				.encoding "screencode_upper"
				.text "hello world"
			`,
			want: []byte{0x00, 0x10, 8, 5, 12, 12, 15, 32, 23, 15, 18, 12, 4},
		},
		{
			name: "single quoted string",
			source: `
				* = $1000
				.encoding "screencode_upper"
				.byte 'A', "B"
			`,
			want: []byte{0x00, 0x10, 1, 2},
		},
		{
			name: ".word rejects string literal",
			source: `
				* = $1000
				.word "ab"
			`,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Assemble(tc.source)
			if (err != nil) != tc.wantErr {
				t.Fatalf("Assemble() error = %v, wantErr = %v", err, tc.wantErr)
			}
			if !tc.wantErr && !reflect.DeepEqual(got.Bytes(), tc.want) {
				t.Errorf("got  %02X\nwant %02X", got.Bytes(), tc.want)
			}
		})
	}
}

// TestAssembleDisassembleRoundtrip verifies that code assembled from source
// disassembles back to the expected canonical mnemonics, covering the
// assemble→disassemble path end-to-end.
func TestAssembleDisassembleRoundtrip(t *testing.T) {
	source := `
		* = $1000
		LDA #$01
		STA $D020
		LDX #$05
		loop:
		DEX
		BNE loop
		ASL A
		RTS
	`
	// $1000: LDA #$01   (2)
	// $1002: STA $D020  (3)
	// $1005: LDX #$05   (2)
	// $1007: DEX         (1)  ← loop
	// $1008: BNE loop   (2) → target $1007
	// $100A: ASL A       (1)
	// $100B: RTS         (1)

	prg, err := Assemble(source)
	if err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}

	insts := Disassemble(prg.Code(), 0x1000)

	expectedCodes := []string{
		"LDA #$01",
		"STA $D020",
		"LDX #$05",
		"DEX",
		"BNE $1007",
		"ASL A",
		"RTS",
	}

	if len(insts) != len(expectedCodes) {
		t.Fatalf("expected %d instructions, got %d:\n%+v", len(expectedCodes), len(insts), insts)
	}
	for i, want := range expectedCodes {
		if insts[i].Code != want {
			t.Errorf("instruction %d: got %q, want %q", i, insts[i].Code, want)
		}
	}
}

func TestAssembleIndexedAddressingErrors(t *testing.T) {
	t.Run("invalid index register", func(t *testing.T) {
		_, err := Assemble(`* = $1000
			lda foo,Z
		`)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "indexed addressing") || !strings.Contains(err.Error(), ",X or ,Y") {
			t.Fatalf("expected indexed addressing hint, got: %v", err)
		}
	})

	t.Run("undefined label in indexed mode", func(t *testing.T) {
		_, err := Assemble(`* = $1000
			lda nosuch,X
		`)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "nosuch") || !strings.Contains(err.Error(), "indexed addressing") {
			t.Fatalf("expected undefined symbol in indexed mode, got: %v", err)
		}
	})
}

func TestSplitAsmTokens(t *testing.T) {
	got := splitAsmTokens(`.byte "hello world", 0`)
	want := []string{`.byte`, `"hello world",`, `0`}
	if len(got) != len(want) {
		t.Fatalf("splitAsmTokens = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("token %d = %q, want %q (all=%#v)", i, got[i], want[i], got)
		}
	}
}

// TestAssembleErrorContext verifies that errors include the original source line
// for easier debugging (improved error quality).
func TestAssembleErrorContext(t *testing.T) {
	_, err := Assemble(`* = $1000
		.byte "unterminated
	`)
	if err == nil {
		t.Fatal("expected error for unterminated string")
	}
	if !strings.Contains(err.Error(), "unterminated string literal") {
		t.Errorf("expected unterminated message, got: %v", err)
	}
	if !strings.Contains(err.Error(), `.byte "unterminated`) {
		t.Errorf("error should contain source line snippet, got: %v", err)
	}

	_, err = Assemble("foo = badval")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "foo = badval") {
		t.Errorf("constant error should quote the source line, got: %v", err)
	}
}

func TestParseDataListQuotedSpaces(t *testing.T) {
	items, err := parseDataList(`"hello world", 0`)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || !items[0].isString || items[0].text != "hello world" {
		t.Fatalf("items = %#v", items)
	}
}

func TestParseLinePreservesQuotedSpaces(t *testing.T) {
	_, _, operand := parseLine(`.byte "hello world", 0`)
	if operand != `"hello world", 0` {
		t.Fatalf("operand = %q", operand)
	}
}

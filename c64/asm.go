// 6502 assembler producing C64 .PRG output.

package c64

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"

	"github.com/c64uploader/go-ultimate/c64/codec"
)

// defaultLoadAddress is the PRG load address ($0801) — start of BASIC area on a stock C64.
const defaultLoadAddress = 0x0801

// textEncoding selects the string encoding used by .text / .byte directives.
type textEncoding int

const (
	encScreencodeMixed textEncoding = iota // Kick Assembler default
	encScreencodeUpper
	encPETSCIIUpper
	encPETSCIIMixed
)

// dataItem is one element in a .byte / .word / .text list.
// Either a string literal or a numeric/label expression.
type dataItem struct {
	isString bool
	text     string // content of string literal (quotes stripped)
	expr     string // expression to resolve (label or number)
}

// parsedInst is one assembled instruction or data directive from Pass 1.
type parsedInst struct {
	pc               uint16
	mnemonic         string
	operand          string
	mode             string
	size             int
	lineNo           int
	sourceLine       string     // original line, for error messages
	dataItems        []dataItem // set for .byte / .word / .text
	dataWord         bool       // true -> .word (2 bytes each)
	encoding         textEncoding
	basicHeaderLabel string // for BASICHeader pseudo-directive: target label for the SYS
}

// encodeStringByte encodes one ASCII byte using the given text encoding.
func encodeStringByte(b byte, enc textEncoding) byte {
	switch enc {
	case encScreencodeUpper:
		if b >= 'a' && b <= 'z' {
			b = b - 'a' + 'A'
		}
		switch {
		case b >= 'A' && b <= 'Z':
			return b - 'A' + 1
		case b == ' ':
			return 32
		default:
			return b
		}
	case encScreencodeMixed:
		switch {
		case b >= 'a' && b <= 'z':
			return b - 'a' + 1
		case b >= 'A' && b <= 'Z':
			return b - 'A' + 65
		case b == ' ':
			return 32
		default:
			return b
		}
	case encPETSCIIUpper:
		return codec.PETSCIIUpper.EncodeByte(b)
	case encPETSCIIMixed:
		return codec.PETSCII.EncodeByte(b)
	}
	return b
}

// Assemble compiles 6502 source into a PRG Program.
//
// Supports standard 6502 mnemonics and addressing modes, labels, NAME = constants,
// * = / .org origins, .byte/.word/.text data (with quoted strings), #< / #> byte
// extraction, $/%/decimal literals, and ; comments.
//
// BASICHeader <label> emits the "10 SYS nnnn" at the current PC (normally after * = $0801).
// This causes jump to the label, allowing machine code to start with RUN.
//
// Text encoding (Kick Assembler compatible):
//   - .encoding "screencode_upper" — screen codes, case folded (A-Z -> 1-26)
//   - .encoding "screencode_mixed" — screen codes, mixed case (default)
//   - .encoding "petscii_upper"    — PETSCII, case folded
//   - .encoding "petscii_mixed"    — PETSCII, mixed case
//
// Zero-page vs absolute mode is chosen from backward label references;
// forward references default to absolute. Multiple .org segments are
// emitted with zero-filled gaps.
func Assemble(source string) (*Program, error) {
	labels := make(map[string]uint16)

	var instructions []parsedInst
	var startAddress uint16 = defaultLoadAddress
	var currentPC uint16 = defaultLoadAddress
	firstOriginSet := false
	currentEncoding := encScreencodeMixed

	// Pass 1: parse lines, collect labels, determine instruction sizes.
	scanner := bufio.NewScanner(strings.NewReader(source))
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		rawLine := scanner.Text()

		// Strip comment; keep rawLine for error reporting.
		line := rawLine
		if idx := strings.Index(line, ";"); idx >= 0 {
			line = line[:idx]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Constant: NAME = value  (not * = addr)
		if strings.Contains(line, "=") && !strings.HasPrefix(line, "*") {
			parts := strings.SplitN(line, "=", 2)
			lbl := normalizeLabel(parts[0])
			valStr := strings.TrimSpace(parts[1])
			val, err := parseValue(valStr)
			if err != nil {
				return nil, fmt.Errorf("line %d: invalid constant value %q: %w\n  %s", lineNo, valStr, err, rawLine)
			}
			if _, exists := labels[lbl]; exists {
				return nil, fmt.Errorf("line %d: duplicate label %q\n  %s", lineNo, lbl, rawLine)
			}
			labels[lbl] = val
			continue
		}

		// Origin: * = $addr
		if strings.HasPrefix(line, "*") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				valStr := strings.TrimSpace(parts[1])
				val, err := parseValue(valStr)
				if err != nil {
					return nil, fmt.Errorf("line %d: invalid origin address %q: %w\n  %s", lineNo, valStr, err, rawLine)
				}
				if !firstOriginSet {
					startAddress = val
					firstOriginSet = true
				}
				currentPC = val
				continue
			}
		}

		// Origin: .org $addr
		if strings.HasPrefix(strings.ToLower(line), ".org ") {
			valStr := strings.TrimSpace(line[5:])
			val, err := parseValue(valStr)
			if err != nil {
				return nil, fmt.Errorf("line %d: invalid .org address %q: %w\n  %s", lineNo, valStr, err, rawLine)
			}
			if !firstOriginSet {
				startAddress = val
				firstOriginSet = true
			}
			currentPC = val
			continue
		}

		// Support both "BASICHeader label" and "BASICHeader(label)" style.
		upperLine := strings.ToUpper(line)
		if strings.HasPrefix(upperLine, "BASICHEADER") {
			// extract argument inside () or after space
			arg := ""
			if idx := strings.Index(line, "("); idx != -1 {
				end := strings.Index(line[idx:], ")")
				if end != -1 {
					arg = strings.TrimSpace(line[idx+1 : idx+end])
				}
			} else {
				// BASICHeader foo
				parts := strings.Fields(line)
				if len(parts) > 1 {
					arg = strings.TrimSpace(parts[1])
				}
			}
			if arg == "" {
				return nil, fmt.Errorf("line %d: BASICHeader requires a label\n  %s", lineNo, rawLine)
			}
			instructions = append(instructions, parsedInst{
				pc:               currentPC,
				lineNo:           lineNo,
				sourceLine:       rawLine,
				basicHeaderLabel: arg,
				size:             12,
			})
			currentPC += 12
			continue
		}

		lbl, mnemonic, operand := parseLine(line)

		if lbl != "" {
			norm := normalizeLabel(lbl)
			if _, exists := labels[norm]; exists {
				return nil, fmt.Errorf("line %d: duplicate label %q\n  %s", lineNo, norm, rawLine)
			}
			labels[norm] = currentPC
		}

		if mnemonic == "" {
			continue
		}

		// Data directives.
		switch mnemonic {
		case ".ENCODING":
			enc := strings.Trim(operand, "\"'")
			switch strings.ToLower(enc) {
			case "screencode_upper":
				currentEncoding = encScreencodeUpper
			case "screencode_mixed":
				currentEncoding = encScreencodeMixed
			case "petscii_upper":
				currentEncoding = encPETSCIIUpper
			case "petscii_mixed":
				currentEncoding = encPETSCIIMixed
			default:
				return nil, fmt.Errorf("line %d: unknown encoding %q\n  %s", lineNo, enc, rawLine)
			}
			continue

		case ".TEXT", ".BYTE":
			items, err := parseDataList(operand)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w\n  %s", lineNo, err, rawLine)
			}
			size := dataItemsByteLen(items)
			instructions = append(instructions, parsedInst{
				pc:         currentPC,
				lineNo:     lineNo,
				sourceLine: rawLine,
				dataItems:  items,
				encoding:   currentEncoding,
				size:       size,
			})
			currentPC += uint16(size)
			continue

		case ".WORD":
			items, err := parseDataList(operand)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w\n  %s", lineNo, err, rawLine)
			}
			for _, it := range items {
				if it.isString {
					return nil, fmt.Errorf("line %d: string literals not allowed in .word\n  %s", lineNo, rawLine)
				}
			}
			instructions = append(instructions, parsedInst{
				pc:         currentPC,
				lineNo:     lineNo,
				sourceLine: rawLine,
				dataItems:  items,
				dataWord:   true,
				size:       len(items) * 2,
			})
			currentPC += uint16(len(items) * 2)
			continue
		}

		// Normalize operand: collapse whitespace so "( $10 , X )" -> "($10,X)",
		// then uppercase index registers so label,x -> label,X.
		operand = normalizeIndexRegisters(strings.Join(strings.Fields(operand), ""))

		mode := parseAddressingMode(mnemonic, operand, labels)
		size := getInstructionSize(mnemonic, mode)

		instructions = append(instructions, parsedInst{
			pc:         currentPC,
			mnemonic:   mnemonic,
			operand:    operand,
			mode:       mode,
			size:       size,
			lineNo:     lineNo,
			sourceLine: rawLine,
		})
		currentPC += uint16(size)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan source: %w", err)
	}

	// Pass 2: emit bytes.
	var prg bytes.Buffer

	// PRG header: 2-byte little-endian load address.
	writeUint16LE(&prg, startAddress)

	emitPC := startAddress

	for _, inst := range instructions {
		// Fill gaps between .org segments with zeros.
		if inst.pc > emitPC {
			prg.Write(make([]byte, inst.pc-emitPC))
			emitPC = inst.pc
		} else if inst.pc < emitPC {
			return nil, fmt.Errorf("line %d: overlapping segment at $%04X (emitted up to $%04X)\n  %s",
				inst.lineNo, inst.pc, emitPC, inst.sourceLine)
		}

		// Data directive (.byte / .word / .text).
		if inst.dataItems != nil {
			for _, item := range inst.dataItems {
				if item.isString {
					for i := 0; i < len(item.text); i++ {
						prg.WriteByte(encodeStringByte(item.text[i], inst.encoding))
					}
					emitPC += uint16(len(item.text))
					continue
				}
				val, err := resolveExpr(strings.TrimSpace(item.expr), labels, "")
				if err != nil {
					return nil, fmt.Errorf("line %d: %w\n  %s", inst.lineNo, err, inst.sourceLine)
				}
				if inst.dataWord {
					writeUint16LE(&prg, val)
					emitPC += 2
				} else {
					if val > 0xFF {
						return nil, fmt.Errorf("line %d: .byte value $%04X exceeds 8 bits\n  %s", inst.lineNo, val, inst.sourceLine)
					}
					prg.WriteByte(byte(val))
					emitPC++
				}
			}
			continue
		}

		// Emits the 12-byte "10 SYS <addr>" stub so RUN will execute at the label.
		if inst.basicHeaderLabel != "" {
			addr, err := resolveExpr(inst.basicHeaderLabel, labels, "")
			if err != nil {
				return nil, fmt.Errorf("line %d: %w\n  %s", inst.lineNo, err, inst.sourceLine)
			}
			stub := basicHeader(addr)
			prg.Write(stub)
			emitPC += uint16(len(stub))
			continue
		}

		// Regular instruction.
		opcode, found := lookupOpcode(inst.mnemonic, inst.mode)
		if !found {
			return nil, fmt.Errorf("line %d: unknown instruction %s %s\n  %s", inst.lineNo, inst.mnemonic, inst.mode, inst.sourceLine)
		}

		prg.WriteByte(opcode)
		emitPC++

		if inst.size > 1 {
			val, err := resolveExpr(stripModeMarkers(inst.operand, inst.mode), labels, inst.mode)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w\n  %s", inst.lineNo, err, inst.sourceLine)
			}

			switch inst.size {
			case 2:
				if inst.mode == "rel" {
					offset := int32(val) - (int32(inst.pc) + 2)
					if offset < -128 || offset > 127 {
						return nil, fmt.Errorf("line %d: branch out of range: offset %d\n  %s", inst.lineNo, offset, inst.sourceLine)
					}
					prg.WriteByte(byte(int8(offset)))
				} else {
					if val > 0xFF {
						return nil, fmt.Errorf("line %d: 8-bit operand limit exceeded: $%04X\n  %s", inst.lineNo, val, inst.sourceLine)
					}
					prg.WriteByte(byte(val))
				}
				emitPC++
			case 3:
				writeUint16LE(&prg, val)
				emitPC += 2
			}
		}
	}

	return NewProgramFromPRG(prg.Bytes()), nil
}

// dataItemsByteLen returns the total byte length of a .byte/.text item list.
func dataItemsByteLen(items []dataItem) int {
	n := 0
	for _, it := range items {
		if it.isString {
			n += len(it.text)
		} else {
			n++
		}
	}
	return n
}

// indexRegisterReplacer uppercases ,x / ,y so indexed addressing is case-insensitive.
var indexRegisterReplacer = strings.NewReplacer(",x", ",X", ",y", ",Y")

// normalizeIndexRegisters uppercases ,X / ,Y so indexed addressing is case-insensitive.
func normalizeIndexRegisters(operand string) string {
	return indexRegisterReplacer.Replace(operand)
}

// stripModeMarkers removes addressing-mode syntax from an operand,
// leaving only the raw value or label name.
func stripModeMarkers(operand, mode string) string {
	switch mode {
	case "imm":
		operand = strings.TrimPrefix(operand, "#")
	case "indx":
		operand = strings.TrimPrefix(operand, "(")
		operand = strings.TrimSuffix(operand, ",X)")
	case "indy":
		operand = strings.TrimPrefix(operand, "(")
		operand = strings.TrimSuffix(operand, "),Y")
	case "ind":
		operand = strings.TrimPrefix(operand, "(")
		operand = strings.TrimSuffix(operand, ")")
	case "absx", "zpx":
		operand = strings.TrimSuffix(operand, ",X")
	case "absy", "zpy":
		operand = strings.TrimSuffix(operand, ",Y")
	}
	return operand
}

// resolveExpr evaluates an operand expression.
// Supports < (low byte) and > (high byte) prefixes, label lookups, and numeric literals.
func resolveExpr(expr string, labels map[string]uint16, mode string) (uint16, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return 0, fmt.Errorf("empty expression")
	}

	loByte := strings.HasPrefix(expr, "<")
	hiByte := strings.HasPrefix(expr, ">")
	if loByte || hiByte {
		expr = expr[1:]
	}

	var val uint16
	if lv, ok := labels[normalizeLabel(expr)]; ok {
		val = lv
	} else {
		var err error
		val, err = parseValue(expr)
		if err != nil {
			return 0, formatResolveError(expr, err, labels, mode)
		}
	}

	switch {
	case loByte:
		return val & 0x00FF, nil
	case hiByte:
		return (val >> 8) & 0x00FF, nil
	default:
		return val, nil
	}
}

func formatResolveError(expr string, err error, labels map[string]uint16, mode string) error {
	if hint := indexedAddressingHint(expr, labels); hint != "" {
		return fmt.Errorf("unresolved symbol or invalid value %q: %w%s", expr, err, hint)
	}
	if isIndexedMode(mode) {
		if _, ok := labels[normalizeLabel(expr)]; !ok {
			return fmt.Errorf("unresolved symbol %q in %s indexed addressing", expr, mode)
		}
	}
	return fmt.Errorf("unresolved symbol or invalid value %q: %w", expr, err)
}

var indexedModes = map[string]bool{
	"absx": true, "absy": true, "zpx": true,
	"zpy": true, "indx": true, "indy": true,
}

func isIndexedMode(mode string) bool { return indexedModes[mode] }

// indexedAddressingHint returns extra context when an operand looks like a
// mistyped indexed address (e.g. "foo,z" instead of "foo,X").
func indexedAddressingHint(expr string, labels map[string]uint16) string {
	comma := strings.LastIndex(expr, ",")
	if comma <= 0 || comma >= len(expr)-1 {
		return ""
	}
	base := strings.TrimSpace(expr[:comma])
	reg := strings.TrimSpace(expr[comma+1:])
	if base == "" {
		return ""
	}
	switch strings.ToUpper(reg) {
	case "X", "Y":
		if _, ok := labels[normalizeLabel(base)]; !ok {
			return fmt.Sprintf(" (indexed addressing %q: symbol %q is undefined)", expr, base)
		}
	default:
		if len(reg) == 1 && ((reg[0] >= 'a' && reg[0] <= 'z') || (reg[0] >= 'A' && reg[0] <= 'Z')) {
			return fmt.Sprintf(" (operand %q looks like indexed addressing — use ,X or ,Y)", expr)
		}
	}
	return ""
}

// normalizeLabel uppercases and trims a label name for case-insensitive lookup.
func normalizeLabel(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

// splitAsmTokens splits a source line on whitespace without breaking quoted strings.
func splitAsmTokens(line string) []string {
	var tokens []string
	var cur strings.Builder
	inQuote := false
	var quote byte

	for i := 0; i < len(line); i++ {
		c := line[i]
		if inQuote {
			cur.WriteByte(c)
			if c == '\\' && i+1 < len(line) {
				i++
				cur.WriteByte(line[i])
				continue
			}
			if c == quote {
				inQuote = false
			}
			continue
		}
		switch c {
		case '"', '\'':
			if cur.Len() > 0 {
				tokens = append(tokens, cur.String())
				cur.Reset()
			}
			inQuote = true
			quote = c
			cur.WriteByte(c)
		case ' ', '\t':
			if cur.Len() > 0 {
				tokens = append(tokens, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteByte(c)
		}
	}
	if cur.Len() > 0 {
		tokens = append(tokens, cur.String())
	}
	return tokens
}

// parseLine splits a source line into (label, mnemonic, operand).
// Labels may be colon-terminated ("loop:") or bare non-mnemonic tokens.
func parseLine(line string) (label string, mnemonic string, operand string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}

	parts := splitAsmTokens(line)
	if len(parts) == 0 {
		return
	}

	first := parts[0]
	isLabel := false
	if strings.HasSuffix(first, ":") {
		label = first[:len(first)-1]
		isLabel = true
	} else {
		upper := strings.ToUpper(first)
		// Tokens beginning with "." or "!" are directives, not labels.
		if !knownMnemonic(upper) && upper != "*" &&
			!strings.HasPrefix(upper, ".") &&
			!strings.HasPrefix(upper, "!") {
			label = first
			isLabel = true
		}
	}

	if isLabel {
		if len(parts) > 1 {
			mnemonic = strings.ToUpper(parts[1])
			operand = strings.Join(parts[2:], " ")
		}
	} else {
		mnemonic = strings.ToUpper(parts[0])
		operand = strings.Join(parts[1:], " ")
	}

	operand = strings.TrimSpace(operand)
	return
}

// parseAddressingMode determines the addressing mode for a mnemonic+operand pair.
// Uses the labels map to resolve backward references for zero-page vs absolute decisions.
// Forward references default to absolute mode.
func parseAddressingMode(mnemonic string, operand string, labels map[string]uint16) string {
	if operand == "" {
		switch mnemonic {
		case "ASL", "LSR", "ROL", "ROR":
			return "acc"
		}
		return "impl"
	}

	// Explicit accumulator: ASL A, LSR A, ROL A, ROR A.
	if operand == "A" {
		switch mnemonic {
		case "ASL", "LSR", "ROL", "ROR":
			return "acc"
		}
	}

	// Branch instructions always use relative mode.
	if isBranch(mnemonic) {
		return "rel"
	}

	// Immediate: #value, #<label, #>label.
	if strings.HasPrefix(operand, "#") {
		return "imm"
	}

	// Indirect: ($xx,X), ($xx),Y, or ($xxxx) for JMP.
	if strings.HasPrefix(operand, "(") {
		if strings.HasSuffix(operand, ",X)") {
			return "indx"
		}
		if strings.HasSuffix(operand, "),Y") {
			return "indy"
		}
		return "ind"
	}

	// Indexed X.
	if before, ok := strings.CutSuffix(operand, ",X"); ok {
		if isZP(before, labels) {
			return "zpx"
		}
		return "absx"
	}

	// Indexed Y.
	if before, ok := strings.CutSuffix(operand, ",Y"); ok {
		if isZP(before, labels) {
			return "zpy"
		}
		return "absy"
	}

	// Direct address.
	if isZP(operand, labels) {
		return "zp"
	}
	return "abs"
}

var branchMnemonics = map[string]bool{
	"BCC": true, "BCS": true, "BEQ": true, "BNE": true,
	"BPL": true, "BMI": true, "BVC": true, "BVS": true,
}

func isBranch(mnemonic string) bool { return branchMnemonics[mnemonic] }

// isZP reports whether the operand resolves to a zero-page address (≤ $FF).
// < and > byte-extraction operators always produce an 8-bit result.
// Forward references and unknown labels default to absolute (false).
func isZP(s string, labels map[string]uint16) bool {
	if strings.HasPrefix(s, "<") || strings.HasPrefix(s, ">") {
		return true
	}
	if val, ok := labels[normalizeLabel(s)]; ok {
		return val <= 0xFF
	}
	val, err := parseValue(s)
	if err != nil {
		return false
	}
	return val <= 0xFF
}

// parseValue parses a numeric literal: hex ($xx / 0x), binary (%), or decimal.
// Negative values are returned as their 16-bit two's-complement form.
func parseValue(s string) (uint16, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty value")
	}

	// $ prefix -> hex.
	if strings.HasPrefix(s, "$") {
		val, err := strconv.ParseUint(s[1:], 16, 16)
		return uint16(val), err
	}

	// Binary: %xxxxxxxx.
	if strings.HasPrefix(s, "%") {
		val, err := strconv.ParseUint(s[1:], 2, 16)
		return uint16(val), err
	}

	// Negative decimal (e.g. branch offsets).
	if strings.HasPrefix(s, "-") {
		val, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return 0, err
		}
		if val < -32768 || val > 32767 {
			return 0, fmt.Errorf("value %d out of 16-bit range", val)
		}
		return uint16(val), nil
	}

	// 0x/0X hex and plain decimal.
	val, err := strconv.ParseUint(s, 0, 16)
	return uint16(val), err
}

// getInstructionSize returns the byte size (opcode + operand) for a mnemonic+mode pair.
// Uses a heuristic during Pass 1 when some labels are not yet resolved.
func getInstructionSize(mnemonic string, mode string) int {
	if op, ok := lookupOpcode(mnemonic, mode); ok {
		return opcodes[op].size
	}
	switch mode {
	case "rel", "zp", "zpx", "zpy", "indx", "indy", "imm":
		return 2
	case "impl", "acc":
		return 1
	}
	return 3
}

// writeUint16LE writes a 16-bit value in little-endian order.
func writeUint16LE(buf *bytes.Buffer, v uint16) {
	buf.Write(binary.LittleEndian.AppendUint16(nil, v))
}

// basicHeader emits the "10 SYS nnnn".
func basicHeader(entry uint16) []byte {
	s := strconv.Itoa(int(entry))
	d := len(s)
	next := uint16(0x0801) + 6 + uint16(d)
	b := make([]byte, 6+d+2)
	b[0] = byte(next)
	b[1] = byte(next >> 8)
	b[2] = 0x0a // line number 10
	b[3] = 0x00
	b[4] = 0x9e // SYS token
	copy(b[5:5+d], []byte(s))
	b[5+d] = 0x00 // end of line
	b[6+d] = 0x00 // end of BASIC program
	b[7+d] = 0x00
	return b
}

// parseDataList parses the operand of .byte / .word / .text into a list of items.
// Items are either quoted string literals or bare expressions, separated by commas.
// Supports basic backslash escapes (\n, \r, \t, \", \', \\) inside strings.
func parseDataList(s string) ([]dataItem, error) {
	var items []dataItem
	i := 0
	n := len(s)

	for i < n {
		// Skip whitespace and commas between items.
		for i < n && (s[i] == ' ' || s[i] == '\t' || s[i] == ',') {
			i++
		}
		if i >= n {
			break
		}

		if c := s[i]; c == '"' || c == '\'' {
			// String literal.
			q := c
			i++ // consume opening quote
			var buf strings.Builder
			closed := false
			for i < n {
				c = s[i]
				if c == '\\' && i+1 < n {
					i++
					switch s[i] {
					case 'n':
						buf.WriteByte('\n')
					case 'r':
						buf.WriteByte('\r')
					case 't':
						buf.WriteByte('\t')
					default:
						buf.WriteByte(s[i])
					}
					i++
					continue
				}
				if c == q {
					i++ // consume closing quote
					closed = true
					break
				}
				buf.WriteByte(c)
				i++
			}
			if !closed {
				return nil, fmt.Errorf("unterminated string literal")
			}
			items = append(items, dataItem{isString: true, text: buf.String()})
			continue
		}

		// Expression: accumulate until comma or end.
		start := i
		for i < n && s[i] != ',' {
			i++
		}
		if expr := strings.TrimSpace(s[start:i]); expr != "" {
			items = append(items, dataItem{expr: expr})
		}
	}

	return items, nil
}

package c64

import (
	"encoding/binary"
	"testing"
)

// makeT64 creates a minimal valid T64 archive with one PRG entry.
// The T64 stores raw code (without PRG 2-byte header) and uses the
// entry's startAddr as the load address. contentLen = endAddr - startAddr.
func makeT64(t *testing.T, name string, loadAddr uint16, code []byte) []byte {
	t.Helper()

	// Header: 64 bytes
	hdr := make([]byte, 64)
	sig := []byte("C64 tape image file")
	copy(hdr[0:], sig)
	// Version: 0x0100
	hdr[32] = 0x00
	hdr[33] = 0x01
	// Max entries: 1
	binary.LittleEndian.PutUint16(hdr[34:36], 1)
	// Used entries: 1
	binary.LittleEndian.PutUint16(hdr[36:38], 1)

	// Entry record: 32 bytes at offset 64
	rec := make([]byte, 32)
	rec[0] = 1                     // entry type: normal tape file
	rec[1] = 0x82                  // file type: PRG closed
	binary.LittleEndian.PutUint16(rec[2:4], loadAddr) // start address (load address)
	endAddr := loadAddr + uint16(len(code))
	binary.LittleEndian.PutUint16(rec[4:6], endAddr) // end address + 1

	// Content offset (right after header + entry record)
	contentOffset := uint32(64 + 32)
	binary.LittleEndian.PutUint32(rec[8:12], contentOffset)

	// CBM file type name (padded)
	nameBytes := []byte(name)
	copy(rec[16:], nameBytes)

	// Assemble: header + entry record + raw code (no PRG header)
	result := make([]byte, 0, contentOffset+uint32(len(code)))
	result = append(result, hdr...)
	result = append(result, rec...)
	result = append(result, code...)
	return result
}

func TestParseT64_OK(t *testing.T) {
	code := []byte{0x00, 0x00, 0x9e, 0x32, 0x30, 0x36, 0x31, 0x00} // BASIC: next-line ptr + line# + SYS + "061"
	data := makeT64(t, "HELLO", 0x0801, code)
	entries, err := ParseT64(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.EntryType != 1 {
		t.Errorf("entry type = %d, want 1", entry.EntryType)
	}
	if entry.FileType != 0x82 {
		t.Errorf("file type = 0x%02x, want 0x82", entry.FileType)
	}
	if entry.StartAddr != 0x0801 {
		t.Errorf("start addr = $%04X, want $0801", entry.StartAddr)
	}
	if len(entry.Data) != 8 {
		t.Errorf("data len = %d, want 8", len(entry.Data))
	}

	// Program() uses T64 metadata load address ($0801), not first 2 bytes of data
	prog := entry.Program()
	if prog.LoadAddress() != 0x0801 {
		t.Errorf("Program() load = $%04X, want $0801", prog.LoadAddress())
	}
	// Size = len(Data) — wraps raw code with PRG header using StartAddr
	if prog.Size() != 8 {
		t.Errorf("Program() size = %d, want 8", prog.Size())
	}
	// The raw code is preserved (first 2 bytes are BASIC line link, not PRG header)
	if len(prog.Code()) != 8 {
		t.Errorf("code length = %d, want 8", len(prog.Code()))
	}
	// Verify the PRG bytes include the correct load address header
	prgBytes := prog.Bytes()
	if len(prgBytes) != 10 {
		t.Errorf("PRG bytes length = %d, want 10", len(prgBytes))
	}
	if prgBytes[0] != 0x01 || prgBytes[1] != 0x08 {
		t.Errorf("PRG header = $%02X%02X, want $0801", prgBytes[1], prgBytes[0])
	}
}

func TestParseT64_MultipleEntries(t *testing.T) {
	// Two entries: RTS at $0801, and border color change at $0900
	code1 := []byte{0x00, 0x00, 0x60}                         // 3 bytes
	code2 := []byte{0x00, 0x00, 0x00, 0xa9, 0x05, 0x8d, 0x20, 0xd0, 0x60} // 9 bytes

	loadAddr1 := uint16(0x0801)
	loadAddr2 := uint16(0x0900)

	// Content starts after header (64) + 2 records (32+32) = 128
	contentOff1 := uint32(64 + 32 + 32)
	contentOff2 := contentOff1 + uint32(len(code1))

	// Header
	hdr := make([]byte, 64)
	copy(hdr[0:], []byte("C64 tape image file"))
	binary.LittleEndian.PutUint16(hdr[34:36], 2) // max entries
	binary.LittleEndian.PutUint16(hdr[36:38], 2) // used entries

	// Entry 1 record
	rec1 := make([]byte, 32)
	rec1[0] = 1
	rec1[1] = 0x82
	binary.LittleEndian.PutUint16(rec1[2:4], loadAddr1)
	binary.LittleEndian.PutUint16(rec1[4:6], loadAddr1+uint16(len(code1)))
	binary.LittleEndian.PutUint32(rec1[8:12], contentOff1)
	copy(rec1[16:], "PART1")

	// Entry 2 record
	rec2 := make([]byte, 32)
	rec2[0] = 1
	rec2[1] = 0x82
	binary.LittleEndian.PutUint16(rec2[2:4], loadAddr2)
	binary.LittleEndian.PutUint16(rec2[4:6], loadAddr2+uint16(len(code2)))
	binary.LittleEndian.PutUint32(rec2[8:12], contentOff2)
	copy(rec2[16:], "PART2")

	// Assemble
	var total []byte
	total = append(total, hdr...)
	total = append(total, rec1...)
	total = append(total, rec2...)
	total = append(total, code1...)
	total = append(total, code2...)

	entries, err := ParseT64(total)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Name != "PART1" {
		t.Errorf("entry 0 name = %q, want PART1", entries[0].Name)
	}
	if entries[1].Name != "PART2" {
		t.Errorf("entry 1 name = %q, want PART2", entries[1].Name)
	}
	if len(entries[0].Data) != 3 {
		t.Errorf("entry 0 data len = %d, want 3", len(entries[0].Data))
	}
	if len(entries[1].Data) != 9 {
		t.Errorf("entry 1 data len = %d, want 9", len(entries[1].Data))
	}
}

func TestParseT64_SkipFreeEntry(t *testing.T) {
	// First entry is free (type 0), second is valid
	hdr := make([]byte, 64)
	copy(hdr[0:], []byte("C64 tape image file"))
	binary.LittleEndian.PutUint16(hdr[34:36], 2)
	binary.LittleEndian.PutUint16(hdr[36:38], 2)

	rec1 := make([]byte, 32)
	rec1[0] = 0 // free entry — should be skipped

	rec2 := make([]byte, 32)
	rec2[0] = 1
	rec2[1] = 0x82
	binary.LittleEndian.PutUint16(rec2[2:4], 0x0801)
	binary.LittleEndian.PutUint16(rec2[4:6], 0x0803) // 2 bytes of code
	binary.LittleEndian.PutUint32(rec2[8:12], 64+32+32)
	copy(rec2[16:], "ONLYONE")

	code := []byte{0x00, 0x00, 0x60} // RTS at $0801

	data := make([]byte, 0, 64+32+32+len(code))
	data = append(data, hdr...)
	data = append(data, rec1...)
	data = append(data, rec2...)
	data = append(data, code...)

	entries, err := ParseT64(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (skipped free), got %d", len(entries))
	}
	if entries[0].Name != "ONLYONE" {
		t.Errorf("name = %q, want ONLYONE", entries[0].Name)
	}
}

func TestParseT64_Invalid(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", nil},
		{"too short", []byte{0, 1, 2}},
		{"bad sig", make([]byte, 64)},
		{"no entries", func() []byte {
			hdr := make([]byte, 64)
			copy(hdr[0:], []byte("C64 tape image file"))
			binary.LittleEndian.PutUint16(hdr[34:36], 0)
			binary.LittleEndian.PutUint16(hdr[36:38], 0)
			return hdr
		}()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseT64(tt.data)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestParseT64_AltSignature(t *testing.T) {
	// Test the alternative "C64S tape file" signature
	hdr := make([]byte, 64)
	copy(hdr[0:], []byte("C64S tape file"))
	binary.LittleEndian.PutUint16(hdr[34:36], 1)
	binary.LittleEndian.PutUint16(hdr[36:38], 1)

	rec := make([]byte, 32)
	rec[0] = 1
	rec[1] = 0x82
	binary.LittleEndian.PutUint16(rec[2:4], 0x0801)
	binary.LittleEndian.PutUint16(rec[4:6], 0x0803)
	binary.LittleEndian.PutUint32(rec[8:12], 64+32)
	copy(rec[16:], "C64S")

	code := []byte{0x00, 0x00, 0x60}

	full := make([]byte, 0, 64+32+len(code))
	full = append(full, hdr...)
	full = append(full, rec...)
	full = append(full, code...)

	entries, err := ParseT64(full)
	if err != nil {
		t.Fatal("C64S signature should be accepted:", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
}

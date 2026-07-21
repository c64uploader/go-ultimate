// T64 tape archive reader.
//
// T64 is an archive format developed for the C64S emulator.
// Unlike .TAP (raw tape waveform), T64 stores one or more files
// as raw code bytes with metadata (load address, filename, size).

package c64

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/c64uploader/go-ultimate/c64/codec"
)

// T64Entry is one file entry from a T64 tape archive.
type T64Entry struct {
	Name      string // Filename decoded from PETSCII (padded $A0 stripped)
	RawName   [16]byte // Raw PETSCII filename bytes
	EntryType byte   // 1 = normal tape file
	FileType  byte   // C64 file type (e.g. 0x82 = PRG closed)
	StartAddr uint16 // Load address from T64 metadata
	EndAddr   uint16 // End address + 1 from T64 metadata
	Data      []byte // Raw code bytes (no PRG header)
}

// Program converts this T64 entry into a *Program suitable for run/load/CRT APIs.
// It uses the T64 metadata StartAddr as the load address and prepends it
// as a 2-byte PRG header. This is the correct approach since T64 stores
// raw code without an embedded PRG load-address header.
func (e *T64Entry) Program() *Program {
	return NewProgram(e.Data, e.StartAddr)
}

// String returns a human-readable summary of the entry.
func (e *T64Entry) String() string {
	return fmt.Sprintf("%-16s  $%04X-%04X  (%d bytes)", e.Name, e.StartAddr, e.EndAddr, len(e.Data))
}

// ParseT64 parses a T64 tape archive and returns all file entries.
// It validates the header and skips free/unused entries.
func ParseT64(data []byte) ([]T64Entry, error) {
	if len(data) < 64 {
		return nil, fmt.Errorf("t64: data too short (%d bytes, need at least 64)", len(data))
	}

	// Validate signature: "C64 tape image file" or original "C64S tape file"
	sig := string(data[0:20])
	if !strings.HasPrefix(sig, "C64 tape image file") &&
		!strings.HasPrefix(sig, "C64S tape file") {
		return nil, fmt.Errorf("t64: invalid signature %q", sig)
	}

	numEntries := binary.LittleEndian.Uint16(data[34:36])
	_ = binary.LittleEndian.Uint16(data[36:38]) // used entries

	// File records start at offset 64, each is 32 bytes
	recordOffset := 64
	neededSize := recordOffset + int(numEntries)*32
	if len(data) < neededSize {
		return nil, fmt.Errorf("t64: data too short for %d entries (%d bytes, need %d)",
			numEntries, len(data), neededSize)
	}

	var entries []T64Entry

	for i := uint16(0); i < numEntries; i++ {
		rec := data[recordOffset+int(i)*32 : recordOffset+int(i)*32+32]

		entryType := rec[0]
		if entryType == 0 {
			// Free entry — skip
			continue
		}

		fileType := rec[1]
		startAddr := binary.LittleEndian.Uint16(rec[2:4])
		endAddr := binary.LittleEndian.Uint16(rec[4:6])
		contentOffset := binary.LittleEndian.Uint32(rec[8:12])

		var rawName [16]byte
		copy(rawName[:], rec[16:32])

		// Decode PETSCII name and strip trailing padding ($A0 / space / NUL)
		name := codec.PETSCIIUpper.DecodeString(bytes.TrimRight(rawName[:], "\x00 \xa0"))

		// Read file content
		co := int(contentOffset)
		if co >= len(data) {
			return nil, fmt.Errorf("t64: entry %d content offset %d beyond file size %d",
				i, co, len(data))
		}

		contentLen := int(endAddr) - int(startAddr)
		if contentLen <= 0 || co+contentLen > len(data) {
			// Read until end of file as fallback
			contentLen = len(data) - co
		}
		if contentLen < 0 {
			contentLen = 0
		}

		content := make([]byte, contentLen)
		copy(content, data[co:co+contentLen])

		entries = append(entries, T64Entry{
			Name:      name,
			RawName:   rawName,
			EntryType: entryType,
			FileType:  fileType,
			StartAddr: startAddr,
			EndAddr:   endAddr,
			Data:      content,
		})
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("t64: no valid entries found")
	}

	return entries, nil
}

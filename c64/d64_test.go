package c64

import (
	"bytes"
	"testing"
)

func TestDisk_D64(t *testing.T) {
	diskName := "TESTDISK"
	fileName := "HELLO"
	fileData := []byte("HELLO COMMODORE WORLD!")

	img, err := NewDiskImage(D64).
		WithDiskName(diskName).
		AddFile(fileName, fileData).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	const expectedD64Size = 174848
	if len(img) != expectedD64Size {
		t.Fatalf("expected D64 size %d, got %d", expectedD64Size, len(img))
	}

	// Verify BAM (Track 18, Sector 0)
	bamOffset := getSectorOffset(18, 0, D64)
	if bamOffset != 91392 {
		t.Fatalf("expected BAM offset 91392, got %d", bamOffset)
	}

	// Next directory sector pointer (Track 18, Sector 1)
	if img[bamOffset] != 18 || img[bamOffset+1] != 1 {
		t.Errorf("BAM link points to %d/%d, expected 18/1", img[bamOffset], img[bamOffset+1])
	}

	// Format version
	if img[bamOffset+2] != 0x41 {
		t.Errorf("BAM format version is %02x, expected 0x41 ('A')", img[bamOffset+2])
	}

	// Disk Name (16 bytes padded with 0xA0)
	expectedDiskName := encodeHeaderName(diskName)
	if !bytes.Equal(img[bamOffset+144:bamOffset+160], expectedDiskName[:]) {
		t.Errorf("expected disk name %q, got %q", expectedDiskName, img[bamOffset+144:bamOffset+160])
	}

	// Verify Directory (Track 18, Sector 1)
	dirOffset := getSectorOffset(18, 1, D64)
	if dirOffset != 91648 {
		t.Fatalf("expected directory offset 91648, got %d", dirOffset)
	}

	// End of directory chain pointer
	if img[dirOffset] != 0 || img[dirOffset+1] != 0xFF {
		t.Errorf("expected directory end link 0/255, got %d/%d", img[dirOffset], img[dirOffset+1])
	}

	// First directory entry (offset 0)
	// File Type at offset 2 (should be 0x82 for closed PRG)
	if img[dirOffset+2] != 0x82 {
		t.Errorf("expected first file type 0x82, got %02x", img[dirOffset+2])
	}

	// First block pointer (should point to Track 1, Sector 0)
	if img[dirOffset+3] != 1 || img[dirOffset+4] != 0 {
		t.Errorf("expected first data block 1/0, got %d/%d", img[dirOffset+3], img[dirOffset+4])
	}

	// Filename (16 bytes padded with 0xA0)
	expectedFileName := encodeHeaderName(fileName)
	if !bytes.Equal(img[dirOffset+5:dirOffset+21], expectedFileName[:]) {
		t.Errorf("expected filename %q, got %q", expectedFileName, img[dirOffset+5:dirOffset+21])
	}

	// Blocks used (should be 1)
	blocksCount := int(img[dirOffset+32]) | int(img[dirOffset+33])<<8
	if blocksCount != 1 {
		t.Errorf("expected blocks count 1, got %d", blocksCount)
	}

	// Verify file data sector (Track 1, Sector 0)
	fileOffset := getSectorOffset(1, 0, D64)
	if fileOffset != 0 {
		t.Fatalf("expected Track 1 Sector 0 offset 0, got %d", fileOffset)
	}

	// Last block link (Track = 0, Sector = len(data) + 1)
	if img[fileOffset] != 0 {
		t.Errorf("expected file end link track 0, got %d", img[fileOffset])
	}
	expectedLenByte := byte(len(fileData) + 1)
	if img[fileOffset+1] != expectedLenByte {
		t.Errorf("expected file last block length byte %d, got %d", expectedLenByte, img[fileOffset+1])
	}

	// File contents
	if !bytes.Equal(img[fileOffset+2:fileOffset+2+len(fileData)], fileData) {
		t.Errorf("expected data %q, got %q", fileData, img[fileOffset+2:fileOffset+2+len(fileData)])
	}
}

func TestDisk_D71(t *testing.T) {
	diskName := "DEMO71"
	fileName := "HELLO"
	fileData := make([]byte, 1000) // Needs multiple blocks (~4 blocks)
	for i := range fileData {
		fileData[i] = byte(i % 256)
	}

	img, err := NewDiskImage(D71).
		WithDiskName(diskName).
		AddFile(fileName, fileData).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	const expectedD71Size = 349696
	if len(img) != expectedD71Size {
		t.Fatalf("expected D71 size %d, got %d", expectedD71Size, len(img))
	}

	// Track 53 Sector 0 offset (BAM side 1)
	bamOffsetS1 := getSectorOffset(53, 0, D71)
	if bamOffsetS1 != 266240 {
		t.Fatalf("expected Track 53 Sector 0 offset 266240, got %d", bamOffsetS1)
	}
}

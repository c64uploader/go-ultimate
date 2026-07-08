package ultimate

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"testing"
)

func TestAVIMuxer_BGRSwap(t *testing.T) {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	muxer := &aviMuxer{w: writer}

	err := muxer.writeHeader()
	if err != nil {
		t.Fatalf("writeHeader failed: %v", err)
	}

	headerBytes := buf.Bytes()
	if !bytes.Contains(headerBytes, []byte("RIFF")) {
		t.Errorf("header missing RIFF")
	}
	if !bytes.Contains(headerBytes, []byte("AVI ")) {
		t.Errorf("header missing AVI ")
	}

	// Clean buffer for writing video chunk
	buf.Reset()
	writer.Reset(&buf)
	testFrame := []byte{1, 2, 3, 4, 5, 6} // R=1, G=2, B=3; R=4, G=5, B=6
	testFrameCopy := make([]byte, len(testFrame))
	copy(testFrameCopy, testFrame)

	// Swap RGB to BGR in-place like the copy worker will do:
	for i := 0; i < len(testFrameCopy); i += 3 {
		testFrameCopy[i], testFrameCopy[i+2] = testFrameCopy[i+2], testFrameCopy[i]
	}

	err = muxer.writeVideoChunk(testFrameCopy)
	if err != nil {
		t.Fatalf("writeVideoChunk failed: %v", err)
	}

	chunkBytes := buf.Bytes()
	// Chunk format: "00db" (4 bytes) + length (4 bytes) + data
	if string(chunkBytes[0:4]) != "00db" {
		t.Errorf("expected chunk header '00db', got %q", chunkBytes[0:4])
	}
	length := binary.LittleEndian.Uint32(chunkBytes[4:8])
	if length != 6 {
		t.Errorf("expected chunk length 6, got %d", length)
	}
	writtenData := chunkBytes[8 : 8+length]
	expectedData := []byte{3, 2, 1, 6, 5, 4}
	if !bytes.Equal(writtenData, expectedData) {
		t.Errorf("expected BGR written data %v, got %v", expectedData, writtenData)
	}
}

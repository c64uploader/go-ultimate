package ultimate

import (
	"bufio"
	"encoding/binary"
	"sync"
)

type aviMuxer struct {
	w   *bufio.Writer
	mu  sync.Mutex
	err error
}

func (m *aviMuxer) writeHeader() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var err error
	writeBytes := func(b []byte) {
		if err != nil {
			return
		}
		_, err = m.w.Write(b)
	}
	writeString := func(s string) {
		writeBytes([]byte(s))
	}
	writeUint32 := func(v uint32) {
		if err != nil {
			return
		}
		var buf [4]byte
		binary.LittleEndian.PutUint32(buf[:], v)
		_, err = m.w.Write(buf[:])
	}
	writeUint16 := func(v uint16) {
		if err != nil {
			return
		}
		var buf [2]byte
		binary.LittleEndian.PutUint16(buf[:], v)
		_, err = m.w.Write(buf[:])
	}

	// RIFF header
	writeString("RIFF")
	writeUint32(0xFFFFFFFF) // Unknown stream size (streaming AVI)
	writeString("AVI ")

	// LIST hdrl
	writeString("LIST")
	writeUint32(290)
	writeString("hdrl")

	// avih
	writeString("avih")
	writeUint32(56)
	writeUint32(20000)      // dwMicroSecPerFrame (50 FPS)
	writeUint32(0)          // dwMaxBytesPerSec
	writeUint32(0)          // dwPaddingGranularity
	writeUint32(0)          // dwFlags
	writeUint32(0)          // dwTotalFrames
	writeUint32(0)          // dwInitialFrames
	writeUint32(2)          // dwStreams
	writeUint32(313344)     // dwSuggestedBufferSize
	writeUint32(384)        // dwWidth
	writeUint32(272)        // dwHeight
	writeUint32(0)          // dwReserved[0]
	writeUint32(0)          // dwReserved[1]
	writeUint32(0)          // dwReserved[2]
	writeUint32(0)          // dwReserved[3]

	// LIST strl (Video)
	writeString("LIST")
	writeUint32(116)
	writeString("strl")

	// strh (Video)
	writeString("strh")
	writeUint32(56)
	writeString("vids")
	writeString("DIB ")
	writeUint32(0)          // dwFlags
	writeUint16(0)          // wPriority
	writeUint16(0)          // wLanguage
	writeUint32(0)          // dwInitialFrames
	writeUint32(1)          // dwScale
	writeUint32(50)         // dwRate (50 FPS)
	writeUint32(0)          // dwStart
	writeUint32(0)          // dwLength
	writeUint32(313344)     // dwSuggestedBufferSize
	writeUint32(4294967295) // dwQuality
	writeUint32(0)          // dwSampleSize
	writeUint16(0)          // rcFrame left
	writeUint16(0)          // rcFrame top
	writeUint16(384)        // rcFrame right
	writeUint16(272)        // rcFrame bottom

	// strf (Video)
	writeString("strf")
	writeUint32(40)
	writeUint32(40)         // biSize
	writeUint32(384)        // biWidth
	writeUint32(0xFFFFFEF0) // biHeight (-272 for top-down)
	writeUint16(1)          // biPlanes
	writeUint16(24)         // biBitCount
	writeUint32(0)          // biCompression
	writeUint32(313344)     // biSizeImage
	writeUint32(0)          // biXPelsPerMeter
	writeUint32(0)          // biYPelsPerMeter
	writeUint32(0)          // biClrUsed
	writeUint32(0)          // biClrImportant

	// LIST strl (Audio)
	writeString("LIST")
	writeUint32(94)
	writeString("strl")

	// strh (Audio)
	writeString("strh")
	writeUint32(56)
	writeString("auds")
	writeUint32(0)          // fccHandler (0)
	writeUint32(0)          // dwFlags
	writeUint16(0)          // wPriority
	writeUint16(0)          // wLanguage
	writeUint32(0)          // dwInitialFrames
	writeUint32(1)          // dwScale
	writeUint32(48000)      // dwRate (48kHz)
	writeUint32(0)          // dwStart
	writeUint32(0)          // dwLength
	writeUint32(4096)       // dwSuggestedBufferSize
	writeUint32(4294967295) // dwQuality
	writeUint32(4)          // dwSampleSize (2 channels * 2 bytes/sample)
	writeUint16(0)          // rcFrame left
	writeUint16(0)          // rcFrame top
	writeUint16(0)          // rcFrame right
	writeUint16(0)          // rcFrame bottom

	// strf (Audio)
	writeString("strf")
	writeUint32(18)
	writeUint16(1)          // wFormatTag (WAVE_FORMAT_PCM)
	writeUint16(2)          // nChannels (stereo)
	writeUint32(48000)      // nSamplesPerSec
	writeUint32(192000)     // nAvgBytesPerSec
	writeUint16(4)          // nBlockAlign
	writeUint16(16)         // wBitsPerSample
	writeUint16(0)          // cbSize

	// LIST movi
	writeString("LIST")
	writeUint32(0xFFFFFFFF) // Unknown LIST size
	writeString("movi")

	if err == nil {
		err = m.w.Flush()
	}
	m.err = err
	return err
}

func (m *aviMuxer) writeVideoChunk(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.err != nil {
		return m.err
	}

	var buf [8]byte
	copy(buf[0:4], "00db")
	binary.LittleEndian.PutUint32(buf[4:8], uint32(len(data)))

	if _, err := m.w.Write(buf[:]); err != nil {
		m.err = err
		return err
	}
	if _, err := m.w.Write(data); err != nil {
		m.err = err
		return err
	}

	if len(data)%2 != 0 {
		if _, err := m.w.Write([]byte{0}); err != nil {
			m.err = err
			return err
		}
	}

	if err := m.w.Flush(); err != nil {
		m.err = err
		return err
	}
	return nil
}

func (m *aviMuxer) writeAudioChunk(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.err != nil {
		return m.err
	}

	var buf [8]byte
	copy(buf[0:4], "01wb")
	binary.LittleEndian.PutUint32(buf[4:8], uint32(len(data)))

	if _, err := m.w.Write(buf[:]); err != nil {
		m.err = err
		return err
	}
	if _, err := m.w.Write(data); err != nil {
		m.err = err
		return err
	}

	if len(data)%2 != 0 {
		if _, err := m.w.Write([]byte{0}); err != nil {
			m.err = err
			return err
		}
	}

	if err := m.w.Flush(); err != nil {
		m.err = err
		return err
	}
	return nil
}

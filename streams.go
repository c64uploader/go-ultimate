// UDP streams for video and audio from the device.
// The receivers return data you can pipe straight to ffmpeg.

package ultimate

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
)

// StreamsService controls starting the streams and gives you readers
// for the data.
type StreamsService struct {
	client *Client
}

// Start tells the device to begin sending the stream to the given IP:port.
func (s *StreamsService) Start(ctx context.Context, name StreamName, ip string) error {
	q := url.Values{}
	q.Set("ip", ip)
	path := "/v1/streams/" + string(name) + ":start?" + q.Encode()
	return s.client.getJSON(ctx, http.MethodPut, path, nil, "", nil)
}

// Stop tells the device to stop the stream.
func (s *StreamsService) Stop(ctx context.Context, name StreamName) error {
	path := "/v1/streams/" + string(name) + ":stop"
	return s.client.getJSON(ctx, http.MethodPut, path, nil, "", nil)
}

const (
	videoWidth  = 384
	videoHeight = 272
)

var c64Palette = [16][3]byte{
	{0x00, 0x00, 0x00}, {0xEF, 0xEF, 0xEF}, {0x8D, 0x2F, 0x34}, {0x6A, 0xD4, 0xCD},
	{0x98, 0x35, 0xA4}, {0x4C, 0xB4, 0x42}, {0x2C, 0x29, 0xB1}, {0xEF, 0xEF, 0x5D},
	{0x98, 0x4E, 0x20}, {0x5B, 0x38, 0x00}, {0xD1, 0x67, 0x6D}, {0x4A, 0x4A, 0x4A},
	{0x7B, 0x7B, 0x7B}, {0x9F, 0xEF, 0x93}, {0x6D, 0x6A, 0xEF}, {0xB2, 0xB2, 0xB2},
}

// Audio returns a reader with clean 48kHz stereo PCM.
// Pipe it to ffmpeg or another tool.
func (s *StreamsService) Audio(ctx context.Context, port int) (io.ReadCloser, error) {
	conn, err := s.listenUDP(port, 2*1024*1024)
	if err != nil {
		return nil, err
	}

	pr, pw := io.Pipe()

	go func() {
		defer func() { _ = conn.Close() }()
		defer func() { _ = pw.Close() }()

		// Close conn and pipe when context is cancelled to unblock reads.
		go func() {
			<-ctx.Done()
			_ = conn.Close()
			_ = pr.Close()
		}()

		buf := make([]byte, 65536)
		for {
			n, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			if n > 2 {
				if _, err := pw.Write(buf[2:n]); err != nil {
					return
				}
			}
		}
	}()

	return pr, nil
}

// Video returns a reader with raw RGB24 frames (384x272).
// Pipe it to ffmpeg or another tool.
func (s *StreamsService) Video(ctx context.Context, port int) (io.ReadCloser, error) {
	conn, err := s.listenUDP(port, 4*1024*1024)
	if err != nil {
		return nil, err
	}

	pr, pw := io.Pipe()

	go func() {
		defer func() { _ = conn.Close() }()
		defer func() { _ = pw.Close() }()

		// Close conn and pipe when context is cancelled to unblock reads.
		go func() {
			<-ctx.Done()
			_ = conn.Close()
			_ = pr.Close()
		}()

		buf := make([]byte, 65536)
		currentFrame := make([]byte, videoWidth*videoHeight*3)

		for {
			n, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			if n < 12 {
				continue
			}

			lin := binary.LittleEndian.Uint16(buf[4:6])
			payload := buf[12:n]
			lineNo := lin & 0x7fff

			for l := range 4 {
				currentLine := int(lineNo) + l
				if currentLine < videoHeight {
					lineOffset := l * 192
					if lineOffset+192 <= len(payload) {
						for x := range 192 {
							b := payload[lineOffset+x]
							c1 := c64Palette[b&0xF]
							c2 := c64Palette[b>>4]
							off := (currentLine*videoWidth + x*2) * 3
							currentFrame[off+0] = c1[0]
							currentFrame[off+1] = c1[1]
							currentFrame[off+2] = c1[2]
							currentFrame[off+3] = c2[0]
							currentFrame[off+4] = c2[1]
							currentFrame[off+5] = c2[2]
						}
					}
				}
			}

			if lin&0x8000 != 0 {
				if _, err := pw.Write(currentFrame); err != nil {
					return
				}
				for i := range currentFrame {
					currentFrame[i] = 0
				}
			}
		}
	}()

	return pr, nil
}

func (s *StreamsService) listenUDP(port int, readBuf int) (*net.UDPConn, error) {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}

	_ = conn.SetReadBuffer(readBuf)
	return conn, nil
}

// AVISession coordinates active video and audio streams from the device.
type AVISession struct {
	Video     io.ReadCloser
	Audio     io.ReadCloser
	cancel    context.CancelFunc
	client    *Client
	videoPort int
	audioPort int
	closeOnce sync.Once
}

// Close stops the streams on C64, cancels the readers, and closes both connections.
func (s *AVISession) Close() error {
	var firstErr error
	s.closeOnce.Do(func() {
		s.cancel()
		if err := s.Video.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		if err := s.Audio.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		// Stop streams on C64
		if err := s.client.Streams.Stop(context.Background(), StreamVideo); err != nil && firstErr == nil {
			firstErr = err
		}
		if err := s.client.Streams.Stop(context.Background(), StreamAudio); err != nil && firstErr == nil {
			firstErr = err
		}
	})
	return firstErr
}

// AVISessionOptions configures an AVISession.
type AVISessionOptions struct {
	HostIP    string
	VideoPort int
	AudioPort int
	Writer    io.Writer
}

// AVISession starts video and audio streams on the device, multiplexes them into an AVI stream,
// and writes the output to the provided Writer.
// Call Close() on the returned session to cleanly stop the streams.
func (s *StreamsService) AVISession(ctx context.Context, opts AVISessionOptions) (*AVISession, error) {
	if opts.Writer == nil {
		return nil, fmt.Errorf("opts.Writer is required")
	}
	if opts.VideoPort == 0 {
		opts.VideoPort = 11000
	}
	if opts.AudioPort == 0 {
		opts.AudioPort = 11001
	}

	// 1. Start streams on C64
	if err := s.Start(ctx, StreamVideo, fmt.Sprintf("%s:%d", opts.HostIP, opts.VideoPort)); err != nil {
		return nil, fmt.Errorf("start video stream: %w", err)
	}
	if err := s.Start(ctx, StreamAudio, fmt.Sprintf("%s:%d", opts.HostIP, opts.AudioPort)); err != nil {
		_ = s.Stop(ctx, StreamVideo)
		return nil, fmt.Errorf("start audio stream: %w", err)
	}

	streamCtx, cancelStream := context.WithCancel(ctx)

	rgb, err := s.Video(streamCtx, opts.VideoPort)
	if err != nil {
		cancelStream()
		_ = s.Stop(ctx, StreamVideo)
		_ = s.Stop(ctx, StreamAudio)
		return nil, fmt.Errorf("open video reader: %w", err)
	}

	pcm, err := s.Audio(streamCtx, opts.AudioPort)
	if err != nil {
		_ = rgb.Close()
		cancelStream()
		_ = s.Stop(ctx, StreamVideo)
		_ = s.Stop(ctx, StreamAudio)
		return nil, fmt.Errorf("open audio reader: %w", err)
	}

	muxer := &aviMuxer{w: bufio.NewWriter(opts.Writer)}

	go func() {
		if err := muxer.writeHeader(); err != nil {
			_ = rgb.Close()
			_ = pcm.Close()
			cancelStream()
			_ = s.Stop(context.Background(), StreamVideo)
			_ = s.Stop(context.Background(), StreamAudio)
			if closer, ok := opts.Writer.(io.Closer); ok {
				_ = closer.Close()
			}
			return
		}

		var wg sync.WaitGroup
		wg.Add(2)

		// Video copy worker
		go func() {
			defer wg.Done()
			frameBuf := make([]byte, videoWidth*videoHeight*3)
			for {
				_, err := io.ReadFull(rgb, frameBuf)
				if err != nil {
					break
				}
				// Convert RGB to BGR in-place for AVI format (DIB expects BGR)
				for i := 0; i < len(frameBuf); i += 3 {
					frameBuf[i], frameBuf[i+2] = frameBuf[i+2], frameBuf[i]
				}
				if err := muxer.writeVideoChunk(frameBuf); err != nil {
					break
				}
			}
		}()

		// Audio copy worker
		go func() {
			defer wg.Done()
			audioBuf := make([]byte, 4096)
			for {
				n, err := pcm.Read(audioBuf)
				if n > 0 {
					if err := muxer.writeAudioChunk(audioBuf[:n]); err != nil {
						break
					}
				}
				if err != nil {
					break
				}
			}
		}()

		wg.Wait()
		_ = muxer.w.Flush()
		if closer, ok := opts.Writer.(io.Closer); ok {
			_ = closer.Close()
		}
	}()

	return &AVISession{
		Video:     rgb,
		Audio:     pcm,
		cancel:    cancelStream,
		client:    s.client,
		videoPort: opts.VideoPort,
		audioPort: opts.AudioPort,
	}, nil
}

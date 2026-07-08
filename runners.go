// Load and run C64 programs; play SID and MOD music.

package ultimate

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/c64uploader/go-ultimate/c64"
)

// RunnersService loads and runs programs or plays music files.
// Methods ending in Bytes upload data from memory; others use a path on the device.
type RunnersService struct {
	client *Client
}

// Run uploads and starts p on the C64.
func (s *RunnersService) Run(ctx context.Context, p *c64.Program) error {
	if p == nil {
		return fmt.Errorf("ultimate: program cannot be nil")
	}
	return s.RunPRGBytes(ctx, p.Bytes())
}

// Load uploads c64.Program into C64 memory without running it.
func (s *RunnersService) Load(ctx context.Context, p *c64.Program) error {
	if p == nil {
		return fmt.Errorf("ultimate: program cannot be nil")
	}
	return s.LoadPRGBytes(ctx, p.Bytes())
}

// LoadPRG loads a .PRG from the device filesystem. Resets the C64 first.
func (s *RunnersService) LoadPRG(ctx context.Context, file string) error {
	return s.client.getJSON(ctx, http.MethodPut, "/v1/runners:load_prg?file="+url.QueryEscape(file), nil, "", nil)
}

// LoadPRGBytes uploads and loads PRG bytes. Resets the C64 first.
func (s *RunnersService) LoadPRGBytes(ctx context.Context, data []byte) error {
	return s.client.getJSON(ctx, http.MethodPost, "/v1/runners:load_prg", bytes.NewReader(data), "application/octet-stream", nil)
}

// RunPRG loads and runs a .PRG from the device filesystem. Resets the C64 first.
func (s *RunnersService) RunPRG(ctx context.Context, file string) error {
	return s.client.getJSON(ctx, http.MethodPut, "/v1/runners:run_prg?file="+url.QueryEscape(file), nil, "", nil)
}

// RunPRGBytes uploads and runs PRG bytes. Resets the C64 first.
func (s *RunnersService) RunPRGBytes(ctx context.Context, data []byte) error {
	return s.client.getJSON(ctx, http.MethodPost, "/v1/runners:run_prg", bytes.NewReader(data), "application/octet-stream", nil)
}

// RunCRT runs a .CRT cartridge file from the device filesystem.
func (s *RunnersService) RunCRT(ctx context.Context, file string) error {
	return s.client.getJSON(ctx, http.MethodPut, "/v1/runners:run_crt?file="+url.QueryEscape(file), nil, "", nil)
}

// RunCRTBytes uploads and runs .CRT cartridge bytes.
func (s *RunnersService) RunCRTBytes(ctx context.Context, data []byte) error {
	return s.client.getJSON(ctx, http.MethodPost, "/v1/runners:run_crt", bytes.NewReader(data), "application/octet-stream", nil)
}

// PlaySID plays a .SID file from the device. song is the sub-tune index (zero-based).
func (s *RunnersService) PlaySID(ctx context.Context, file string, song int) error {
	path := "/v1/runners:sidplay?file=" + url.QueryEscape(file)
	if song > 0 {
		path += "&songnr=" + strconv.Itoa(song)
	}
	return s.client.getJSON(ctx, http.MethodPut, path, nil, "", nil)
}

// PlaySIDBytes uploads and plays .SID bytes. song is the sub-tune index (zero-based).
func (s *RunnersService) PlaySIDBytes(ctx context.Context, data []byte, song int) error {
	path := "/v1/runners:sidplay"
	if song > 0 {
		path += "?songnr=" + strconv.Itoa(song)
	}
	return s.client.getJSON(ctx, http.MethodPost, path, bytes.NewReader(data), "application/octet-stream", nil)
}

// PlayMOD plays a .MOD file from the device filesystem.
func (s *RunnersService) PlayMOD(ctx context.Context, file string) error {
	return s.client.getJSON(ctx, http.MethodPut, "/v1/runners:modplay?file="+url.QueryEscape(file), nil, "", nil)
}

// PlayMODBytes uploads and plays .MOD bytes.
func (s *RunnersService) PlayMODBytes(ctx context.Context, data []byte) error {
	return s.client.getJSON(ctx, http.MethodPost, "/v1/runners:modplay", bytes.NewReader(data), "application/octet-stream", nil)
}

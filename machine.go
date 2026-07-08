// C64 power control and DMA memory access.

package ultimate

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/c64uploader/go-ultimate/c64"
)

// MachineService controls C64 run state and reads/writes RAM over DMA.
type MachineService struct {
	client *Client
}

// Inject writes p into C64 RAM at its load address without resetting.
func (s *MachineService) Inject(ctx context.Context, p *c64.Program) error {
	if p == nil {
		return fmt.Errorf("ultimate: program cannot be nil")
	}
	// Write the code (without PRG header) starting at the address from the PRG header.
	return s.WriteMemory(ctx, p.LoadAddress(), p.Code())
}

// Reset performs a hardware C64 reset. Cartridge settings are kept.
func (s *MachineService) Reset(ctx context.Context) error {
	return s.client.getJSON(ctx, http.MethodPut, "/v1/machine:reset", nil, "", nil)
}

// Reboot restarts Ultimate firmware and then resets the C64.
func (s *MachineService) Reboot(ctx context.Context) error {
	return s.client.getJSON(ctx, http.MethodPut, "/v1/machine:reboot", nil, "", nil)
}

// Pause freezes the C64. The CPU stops, but RAM is preserved.
func (s *MachineService) Pause(ctx context.Context) error {
	return s.client.getJSON(ctx, http.MethodPut, "/v1/machine:pause", nil, "", nil)
}

// Resume lets the C64 run again after a Pause.
func (s *MachineService) Resume(ctx context.Context) error {
	return s.client.getJSON(ctx, http.MethodPut, "/v1/machine:resume", nil, "", nil)
}

// PowerOff shuts down the device.
func (s *MachineService) PowerOff(ctx context.Context) error {
	return s.client.getJSON(ctx, http.MethodPut, "/v1/machine:poweroff", nil, "", nil)
}

// MenuButton simulates a press of the cartridge menu button.
func (s *MachineService) MenuButton(ctx context.Context) error {
	return s.client.getJSON(ctx, http.MethodPut, "/v1/machine:menu_button", nil, "", nil)
}

// ReadMemory reads length bytes from C64 RAM starting at address.
func (s *MachineService) ReadMemory(ctx context.Context, address uint16, length int) ([]byte, error) {
	q := url.Values{}
	q.Set("address", fmt.Sprintf("%X", address))
	if length > 0 {
		q.Set("length", strconv.Itoa(length))
	}
	path := "/v1/machine:readmem?" + q.Encode()
	return s.client.getRaw(ctx, http.MethodGet, path, nil, "")
}

// WriteMemory writes data to C64 RAM.
func (s *MachineService) WriteMemory(ctx context.Context, address uint16, data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("ultimate: WriteMemory requires at least one byte")
	}
	if int(address)+len(data) > 0x10000 {
		return fmt.Errorf("ultimate: write of %d bytes at $%X exceeds $FFFF", len(data), address)
	}
	if len(data) <= 128 {
		q := url.Values{}
		q.Set("address", fmt.Sprintf("%X", address))
		q.Set("data", hex.EncodeToString(data))
		path := "/v1/machine:writemem?" + q.Encode()
		return s.client.getJSON(ctx, http.MethodPut, path, nil, "", nil)
	}
	path := "/v1/machine:writemem?address=" + fmt.Sprintf("%X", address)
	return s.client.getJSON(ctx, http.MethodPost, path, bytes.NewReader(data), "application/octet-stream", nil)
}

// ReadDebugRegister reads the Ultimate 64 debug register at $D7FF.
func (s *MachineService) ReadDebugRegister(ctx context.Context) (byte, error) {
	var resp debugRegResponse
	if err := s.client.getJSON(ctx, http.MethodGet, "/v1/machine:debugreg", nil, "", &resp); err != nil {
		return 0, err
	}
	return parseHexByte(resp.Value)
}

// WriteDebugRegister writes value to $D7FF and returns the readback.
func (s *MachineService) WriteDebugRegister(ctx context.Context, value byte) (byte, error) {
	path := "/v1/machine:debugreg?value=" + fmt.Sprintf("%02X", value)
	var resp debugRegResponse
	if err := s.client.getJSON(ctx, http.MethodPut, path, nil, "", &resp); err != nil {
		return 0, err
	}
	return parseHexByte(resp.Value)
}

// MeasureBus returns a VCD trace of cartridge bus timing.
func (s *MachineService) MeasureBus(ctx context.Context) ([]byte, error) {
	return s.client.getRaw(ctx, http.MethodGet, "/v1/machine:measure", nil, "")
}

type debugRegResponse struct {
	Value  string   `json:"value"`
	Errors []string `json:"errors"`
}

func parseHexByte(s string) (byte, error) {
	v, err := strconv.ParseUint(s, 16, 8)
	if err != nil {
		return 0, fmt.Errorf("ultimate: parse debug register value %q: %w", s, err)
	}
	return byte(v), nil
}

// Peek reads one byte from C64 RAM.
func (s *MachineService) Peek(ctx context.Context, address uint16) (byte, error) {
	data, err := s.ReadMemory(ctx, address, 1)
	if err != nil {
		return 0, err
	}
	return data[0], nil
}

// Poke writes one byte to C64 RAM.
func (s *MachineService) Poke(ctx context.Context, address uint16, value byte) error {
	return s.WriteMemory(ctx, address, []byte{value})
}

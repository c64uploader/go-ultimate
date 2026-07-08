// Low-level TCP command socket on port 64.

package ultimate

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/c64uploader/go-ultimate/c64"
)

// RawService manages connections on the device's TCP port 64.
type RawService struct {
	client *Client
}

// RawConn represents an active connection to the device's TCP port 64 command socket.
type RawConn struct {
	conn net.Conn
}

// Dial opens the TCP connection and authenticates when a password is configured.
func (s *RawService) Dial(ctx context.Context) (*RawConn, error) {
	if s.client.host == "" {
		return nil, fmt.Errorf("ultimate: hostname is required for raw socket connection")
	}

	d := net.Dialer{}
	addr := s.client.host
	if !strings.Contains(addr, ":") {
		addr = net.JoinHostPort(addr, "64")
	}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("ultimate: connect to raw socket: %w", err)
	}

	rc := &RawConn{conn: conn}

	if s.client.password != "" {
		// SOCKET_CMD_AUTHENTICATE = 0xFF1F
		if err := rc.sendCommand(0xFF1F, []byte(s.client.password)); err != nil {
			_ = rc.Close()
			return nil, fmt.Errorf("ultimate: send authentication: %w", err)
		}

		resp := make([]byte, 1)
		if _, err := conn.Read(resp); err != nil {
			_ = rc.Close()
			return nil, fmt.Errorf("ultimate: read authentication response: %w", err)
		}

		if resp[0] != 1 {
			_ = rc.Close()
			return nil, fmt.Errorf("ultimate: raw socket authentication failed")
		}
	}

	return rc, nil
}

// Close closes the raw TCP connection.
func (c *RawConn) Close() error {
	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil
	return err
}

// WriteMemory DMA-writes data to C64 RAM via the SOCKET_CMD_DMAWRITE command.
func (c *RawConn) WriteMemory(ctx context.Context, address uint16, data []byte) error {
	payload := make([]byte, 2+len(data))
	binary.LittleEndian.PutUint16(payload[0:2], address)
	copy(payload[2:], data)

	// SOCKET_CMD_DMAWRITE = 0xFF06
	return c.sendCommand(0xFF06, payload)
}

// WriteREU writes data to REU expansion RAM at reuOffset.
func (c *RawConn) WriteREU(ctx context.Context, reuOffset uint32, data []byte) error {
	payload := make([]byte, 3+len(data))
	payload[0] = byte(reuOffset & 0xFF)
	payload[1] = byte((reuOffset >> 8) & 0xFF)
	payload[2] = byte((reuOffset >> 16) & 0xFF)
	copy(payload[3:], data)

	// SOCKET_CMD_REUWRITE = 0xFF07
	return c.sendCommand(0xFF07, payload)
}

// LoadKernal uploads a replacement KERNAL ROM image.
func (c *RawConn) LoadKernal(ctx context.Context, rom []byte) error {
	// SOCKET_CMD_KERNALWRITE = 0xFF08
	return c.sendCommand(0xFF08, rom)
}

// Reset resets the C64 via SOCKET_CMD_RESET.
func (c *RawConn) Reset(ctx context.Context) error {
	// SOCKET_CMD_RESET = 0xFF04
	return c.sendCommand(0xFF04, nil)
}

// LoadAndRun DMA-loads a standard C64 PRG program (the first 2 bytes are the load address)
// and runs it automatically.
func (c *RawConn) LoadAndRun(ctx context.Context, prg []byte) error {
	// SOCKET_CMD_DMARUN = 0xFF02
	return c.sendCommand(0xFF02, prg)
}

// LoadAndJump DMA-loads a program and jumps to a specific entry point.
// prg is a standard C64 PRG (starts with 2-byte load address).
func (c *RawConn) LoadAndJump(ctx context.Context, jumpAddr uint16, prg []byte) error {
	payload := make([]byte, 2+len(prg))
	binary.LittleEndian.PutUint16(payload[0:2], jumpAddr)
	copy(payload[2:], prg)
	// SOCKET_CMD_DMAJUMP = 0xFF09
	return c.sendCommand(0xFF09, payload)
}

// RunCartridge runs a .crt cartridge image.
func (c *RawConn) RunCartridge(ctx context.Context, crt []byte) error {
	// SOCKET_CMD_RUN_CRT = 0xFF0D
	return c.sendCommand24(0xFF0D, crt)
}

// RunDiskImage mounts and runs a .d64 or other disk image.
func (c *RawConn) RunDiskImage(ctx context.Context, d64 []byte) error {
	// SOCKET_CMD_RUN_IMG = 0xFF0B
	return c.sendCommand24(0xFF0B, d64)
}

// MountDiskImage mounts a .d64 or other disk image without running it.
func (c *RawConn) MountDiskImage(ctx context.Context, d64 []byte) error {
	// SOCKET_CMD_MOUNT_IMG = 0xFF0A
	return c.sendCommand24(0xFF0A, d64)
}

// Type enqueues keystrokes directly into the C64 KERNAL keyboard buffer.
func (c *RawConn) Type(ctx context.Context, text string) error {
	// Encode text using default FoldCase
	petscii := c64.EncodeKeys(text, c64.FoldCase)
	// SOCKET_CMD_KEYB = 0xFF03
	return c.sendCommand(0xFF03, petscii)
}

// Identify requests the product title string from the device.
func (c *RawConn) Identify(ctx context.Context) (string, error) {
	// SOCKET_CMD_IDENTIFY = 0xFF0E
	if err := c.sendCommand(0xFF0E, nil); err != nil {
		return "", err
	}

	lenBuf := make([]byte, 1)
	if _, err := c.conn.Read(lenBuf); err != nil {
		return "", err
	}

	titleLen := int(lenBuf[0])
	titleBuf := make([]byte, titleLen)
	if _, err := io.ReadFull(c.conn, titleBuf); err != nil {
		return "", err
	}

	return string(titleBuf), nil
}

// Wait instructs the firmware to delay execution by the specified number of FreeRTOS ticks.
func (c *RawConn) Wait(ctx context.Context, ticks uint16) error {
	// SOCKET_CMD_WAIT = 0xFF05
	// The payload is empty, but the length field in the header holds the ticks!
	header := make([]byte, 4)
	binary.LittleEndian.PutUint16(header[0:2], 0xFF05)
	binary.LittleEndian.PutUint16(header[2:4], ticks)

	if _, err := c.conn.Write(header); err != nil {
		return err
	}
	return nil
}

// PowerOff powers off the device.
func (c *RawConn) PowerOff(ctx context.Context) error {
	// SOCKET_CMD_POWEROFF = 0xFF0C
	return c.sendCommand(0xFF0C, nil)
}

func (c *RawConn) sendCommand(cmd uint16, data []byte) error {
	header := make([]byte, 4)
	binary.LittleEndian.PutUint16(header[0:2], cmd)
	binary.LittleEndian.PutUint16(header[2:4], uint16(len(data)))

	if _, err := c.conn.Write(header); err != nil {
		return err
	}
	if len(data) > 0 {
		if _, err := c.conn.Write(data); err != nil {
			return err
		}
	}
	return nil
}

func (c *RawConn) sendCommand24(cmd uint16, data []byte) error {
	header := make([]byte, 5)
	binary.LittleEndian.PutUint16(header[0:2], cmd)
	l := len(data)
	header[2] = byte(l & 0xFF)
	header[3] = byte((l >> 8) & 0xFF)
	header[4] = byte((l >> 16) & 0xFF)

	if _, err := c.conn.Write(header); err != nil {
		return err
	}
	if len(data) > 0 {
		if _, err := c.conn.Write(data); err != nil {
			return err
		}
	}
	return nil
}

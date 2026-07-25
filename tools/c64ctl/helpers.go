package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/c64uploader/go-ultimate"
	"github.com/c64uploader/go-ultimate/c64"
)

func mountDrive(path string, drive ultimate.DriveID) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	ext := strings.ToLower(filepath.Ext(path))
	var imageType ultimate.ImageType
	switch ext {
	case ".d71":
		imageType = ultimate.ImageD71
	case ".d81":
		imageType = ultimate.ImageD81
	case ".g64":
		imageType = ultimate.ImageG64
	default:
		imageType = ultimate.ImageD64
	}
	fmt.Printf("Mounting %s (%d bytes)...\n", filepath.Base(path), len(data))
	return client.Drives.MountBytes(context.Background(), drive, data, ultimate.MountOptions{
		ImageType: imageType,
		Mode:      ultimate.MountReadOnly,
	})
}

func parseHex16(s string) (uint16, error) {
	s = strings.TrimPrefix(strings.ToUpper(s), "$")
	s = strings.TrimPrefix(s, "0X")
	v, err := strconv.ParseUint(s, 16, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid hex address %q: %w", s, err)
	}
	return uint16(v), nil
}

func parseHex8(s string) (byte, error) {
	s = strings.TrimPrefix(strings.ToUpper(s), "$")
	s = strings.TrimPrefix(s, "0X")
	v, err := strconv.ParseUint(s, 16, 8)
	if err != nil {
		return 0, fmt.Errorf("invalid hex byte %q: %w", s, err)
	}
	return byte(v), nil
}

func parseKey(name string) (c64.Key, bool) {
	name = strings.ToUpper(name)
	switch name {
	case "SPACE":
		return c64.KeySpace, true
	case "RETURN", "ENTER":
		return c64.KeyReturn, true
	case "RUN/STOP", "RUN", "STOP":
		return c64.KeyRunStop, true
	case "F1":
		return c64.KeyF1, true
	case "F3":
		return c64.KeyF3, true
	case "F5":
		return c64.KeyF5, true
	case "F7":
		return c64.KeyF7, true
	case "LEFT":
		return c64.KeyCursorLeft, true
	case "DOWN":
		return c64.KeyCursorDown, true
	case "UP":
		return c64.KeyUpArrow, true
	case "DELETE", "DEL", "INSERT", "INS":
		return c64.KeyInsertDelete, true
	case "HOME":
		return c64.KeyHome, true
	case "SHIFT":
		return c64.KeyLeftShift, true
	case "COMMODORE", "C=":
		return c64.KeyCommodore, true
	case "CTRL", "CONTROL":
		return c64.KeyControl, true
	}
	// Try single letters A-Z
	letterKeys := map[byte]c64.Key{
		'A': c64.KeyA, 'B': c64.KeyB, 'C': c64.KeyC, 'D': c64.KeyD,
		'E': c64.KeyE, 'F': c64.KeyF, 'G': c64.KeyG, 'H': c64.KeyH,
		'I': c64.KeyI, 'J': c64.KeyJ, 'K': c64.KeyK, 'L': c64.KeyL,
		'M': c64.KeyM, 'N': c64.KeyN, 'O': c64.KeyO, 'P': c64.KeyP,
		'Q': c64.KeyQ, 'R': c64.KeyR, 'S': c64.KeyS, 'T': c64.KeyT,
		'U': c64.KeyU, 'V': c64.KeyV, 'W': c64.KeyW, 'X': c64.KeyX,
		'Y': c64.KeyY, 'Z': c64.KeyZ,
	}
	if len(name) == 1 {
		if k, ok := letterKeys[name[0]]; ok {
			return k, true
		}
	}
	// Try single digits 0-9
	digitKeys := map[byte]c64.Key{
		'0': c64.Key0, '1': c64.Key1, '2': c64.Key2, '3': c64.Key3,
		'4': c64.Key4, '5': c64.Key5, '6': c64.Key6, '7': c64.Key7,
		'8': c64.Key8, '9': c64.Key9,
	}
	if len(name) == 1 {
		if k, ok := digitKeys[name[0]]; ok {
			return k, true
		}
	}
	return c64.Key{}, false
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getLocalIP(targetHost ...string) string {
	host := "c64u"
	if len(targetHost) > 0 && targetHost[0] != "" {
		host = targetHost[0]
	} else if c64Host != "" {
		host = c64Host
	}

	if !strings.Contains(host, ":") {
		host = net.JoinHostPort(host, "80")
	}

	conn, err := net.DialTimeout("udp", host, 2*time.Second)
	if err == nil {
		defer func() { _ = conn.Close() }()
		if localAddr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
			return localAddr.IP.String()
		}
	}

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			return ipnet.IP.String()
		}
	}
	return "127.0.0.1"
}

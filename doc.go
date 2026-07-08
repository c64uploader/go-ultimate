// Package ultimate is a Go client for the Commodore 64 Ultimate family of devices:
// the original Ultimate cartridge, Ultimate 64, and Ultimate-II+.
//
// These devices add a modern interface (HTTP REST API, optional TCP socket) to a
// Commodore 64. This package wraps that API so you can reset the machine, read and
// write RAM, mount disk images, run programs, stream video/audio, and inspect
// hardware state from Go code.
//
// # Quick start
//
//	client, err := ultimate.New("c64u")
//	if err != nil {
//		log.Fatal(err)
//	}
//	err = client.Machine.Reset(context.Background())
//
// # Services
//
// The Client exposes grouped services:
//
//   - Machine: reset, pause/resume, power, DMA memory access
//   - Runners: load and run .PRG/.CRT files, play .SID/.MOD music
//   - Drives: floppy drive control and disk image mounting
//   - Configs: read and write device settings
//   - Streams: start/stop UDP video, audio, or debug streams (Ultimate 64)
//   - Files: file metadata and blank disk image creation on the device
//   - Keyboard: Type (KERNAL buffer) and Press (CIA matrix chords)
//   - Debug: decode live screen text, BASIC, VIC/CIA registers, sprites, bitmaps,

//   - Raw: low-level TCP commands (port 64), including REU and KERNAL writes
//
// C64-specific data formats and decoders live in the c64 subpackage. That package
// has no network dependency and can be used on its own.
//
// # Authentication
//
// Password-protected devices accept credentials via WithPassword:
//
//	client, err := ultimate.New("c64u", ultimate.WithPassword("secret"))
//
// Device failures return *APIError. Inspect the HTTP status and firmware messages
// with errors.As:
//
//	var apiErr *ultimate.APIError
//	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
//		// resource not found
//	}
package ultimate
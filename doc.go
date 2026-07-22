// Package ultimate is a Go client for the Commodore 64 Ultimate family of devices.
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
//   - Machine: Control power, reset, and read/write RAM over DMA.
//   - Runners: Upload and run programs (.PRG, .CRT) or play music (.SID, .MOD).
//   - Drives: Mount disk images and control emulated floppy drives.
//   - Files: Query file metadata and create blank disk images.
//   - Configs: Read and write device settings.
//   - Keyboard: Inject text and keystrokes into C64 (latter is best-effort).
//   - Streams: Multiplex video and audio streams into AVI format.
//   - Debug: Read and decode C64 state: screen, registers, memory.
//   - Raw: Send binary commands to the TCP port 64 socket for lower latency than the REST API.
//
// C64-specific data formats and decoders live in the c64 subpackage.
//
// # Authentication
//
// For password-protected devices set credentials via WithPassword:
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

package e2e

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/c64uploader/go-ultimate"
	"github.com/c64uploader/go-ultimate/c64"
)

// downloadFile fetches a file from the given URL.
func downloadFile(ctx context.Context, t *testing.T, url string) ([]byte, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create download request: %w", err)
	}

	t.Logf("Downloading file from %s...", url)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download file: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download file: unexpected status %s", resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read file data: %w", err)
	}
	return data, nil
}

// runInteractivePrompt compiles, injects, and runs an interactive confirmation prompt on C64.
// It returns true if user presses 'Y'/'y' (Pass) or false if user presses 'N'/'n' (Fail).
func runInteractivePrompt(ctx context.Context, t *testing.T, client *ultimate.Client, prompt string) (bool, error) {
	t.Helper()

	prompt += "\npress y (pass) or n (fail):"
	asm := `
* = $c100
    jsr print_msg

wait_key:
    jsr $ffe4   ; GETIN
    cmp #$00
    beq wait_key

    cmp #$59    ; 'Y'
    beq pressed_y
    cmp #$79    ; 'y'
    beq pressed_y
    cmp #$4e    ; 'N'
    beq pressed_n
    cmp #$6e    ; 'n'
    beq pressed_n
    jmp wait_key

pressed_y:
    lda #$01
    sta $c000
    rts

pressed_n:
    lda #$02
    sta $c000
    rts

print_msg:
    ldy #$00
print_loop:
    lda msg,Y
    beq print_done
    jsr $ffd2   ; BSOUT
    iny
    jmp print_loop
print_done:
    rts

msg:
    .encoding "petscii_upper"
    .text ` + fmt.Sprintf("%q", prompt) + `
    .byte 0
`

	statusVal, err := run6502(ctx, t, client, asm, 0xC000)
	if err != nil {
		return false, err
	}

	switch statusVal {
	case 0x01:
		return true, nil
	case 0x02:
		return false, nil
	default:
		return false, fmt.Errorf("unexpected status code from prompt: $%02X", statusVal)
	}
}

// getLocalIPForTarget determines the local interface IP address used to communicate with targetAddr.
func getLocalIPForTarget(targetAddr string) (string, error) {
	host := targetAddr
	if h, _, err := net.SplitHostPort(targetAddr); err == nil {
		host = h
	}
	conn, err := net.Dial("udp", net.JoinHostPort(host, "80"))
	if err != nil {
		return "", err
	}
	defer func() { _ = conn.Close() }()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String(), nil
}

// setupE2E connects to the real C64 Ultimate device, checking the C64U_ADDRESS environment variable.
// If not set, it defaults to "c64u".
func setupE2E(t *testing.T) (*ultimate.Client, context.Context) {
	t.Helper()

	addr := os.Getenv("C64U_ADDRESS")
	if addr == "" {
		addr = "c64u"
	}
	password := os.Getenv("C64U_PASSWORD")

	client, err := ultimate.New(addr, ultimate.WithPassword(password))
	if err != nil {
		t.Fatalf("Failed to initialize client for %s: %v", addr, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	// Ping the machine to verify connection
	_, err = client.Version(ctx)
	if err != nil {
		t.Fatalf("Cannot reach C64 Ultimate at %s: %v", addr, err)
	}

	return client, ctx
}

// rebootAndReady resets the C64 machine and waits until the BASIC READY prompt is visible on the screen.
func rebootAndReady(ctx context.Context, t *testing.T, client *ultimate.Client) {
	t.Helper()
	t.Log("Rebooting C64...")
	if err := client.Machine.Reboot(ctx); err != nil {
		t.Fatalf("Reboot failed: %v", err)
	}

	waitReady(ctx, t, client)
}

// waitReady waits for the BASIC READY prompt to appear on screen.
func waitReady(ctx context.Context, t *testing.T, client *ultimate.Client) {
	t.Helper()

	// Wait 2 seconds for hardware reset to complete and ROMs to boot
	select {
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	case <-time.After(2 * time.Second):
	}

	// Poll screen memory to verify READY prompt is present
	t.Log("Waiting for BASIC READY prompt...")
	limit := time.Now().Add(5 * time.Second)
	for time.Now().Before(limit) {
		screen, err := client.Debug.Screen(ctx)
		if err == nil {
			for _, row := range screen.Rows {
				if strings.Contains(row, "READY.") {
					t.Log("C64 is ready.")
					return
				}
			}
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(200 * time.Millisecond):
		}
	}

	t.Fatal("Timeout waiting for BASIC READY prompt")
}

// run6502 compiles, injects, and runs a 6502 assembly routine.
// It pokes 0x00 to statusAddress before running, types sys <loadAddress> to execute,
// and polls statusAddress until a non-zero status is written (or timeout occurs).
func run6502(ctx context.Context, t *testing.T, client *ultimate.Client, asmSource string, statusAddress uint16) (byte, error) {
	t.Helper()

	prog, err := c64.Assemble(asmSource)
	if err != nil {
		return 0, fmt.Errorf("failed to assemble: %w", err)
	}

	// 2. Clear status address
	if err := client.Machine.Poke(ctx, statusAddress, 0x00); err != nil {
		return 0, fmt.Errorf("failed to clear status byte: %w", err)
	}

	// 3. Inject program to RAM
	if err := client.Machine.Inject(ctx, prog); err != nil {
		return 0, fmt.Errorf("failed to inject program: %w", err)
	}

	// 4. Run via SYS keyboard command
	sysCmd := fmt.Sprintf("sys %d\n", prog.LoadAddress())
	t.Logf("Executing: %s (address $%04X)", strings.TrimSpace(sysCmd), prog.LoadAddress())
	if err := client.Keyboard.Type(ctx, sysCmd); err != nil {
		return 0, fmt.Errorf("failed to type SYS command: %w", err)
	}

	// 5. Poll status address for completion
	limit := time.Now().Add(20 * time.Second) // larger timeout for interactive tests
	for time.Now().Before(limit) {
		status, err := client.Machine.Peek(ctx, statusAddress)
		if err == nil && status != 0 {
			return status, nil
		}
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}

	return 0, fmt.Errorf("timeout waiting for 6502 execution to signal status at $%04X", statusAddress)
}

// verifyScreenContains polls screen memory until all expected strings are found.
// Fails the test if the strings are not found within the 10-second timeout.
func verifyScreenContains(ctx context.Context, t *testing.T, client *ultimate.Client, expected []string) {
	t.Helper()
	limit := time.Now().Add(10 * time.Second)
	for time.Now().Before(limit) {
		screen, err := client.Debug.Screen(ctx)
		if err == nil {
			allFound := true
			screenText := strings.Join(screen.Rows, "\n")
			for _, exp := range expected {
				if !strings.Contains(screenText, exp) {
					allFound = false
					break
				}
			}
			if allFound {
				return
			}
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(200 * time.Millisecond):
		}
	}
	screen, err := client.Debug.Screen(ctx)
	if err == nil {
		t.Logf("Final screen content:\n%s", strings.Join(screen.Rows, "\n"))
	} else {
		t.Logf("Failed to read final screen: %v", err)
	}
	t.Fatalf("Timeout waiting for expected strings on screen: %v", expected)
}

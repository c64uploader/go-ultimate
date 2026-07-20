package ultimate

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"image"
	"net"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/c64uploader/go-ultimate/c64"
)

// mockT adapts the mock server helpers when no *testing.T is available.

func newTestClient(t *testing.T, m *mockServer) *Client {
	t.Helper()
	c, err := New("", WithBaseURL(m.server().URL))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestNewRequiresHost(t *testing.T) {
	if _, err := New(""); err == nil {
		t.Fatal("expected error when host is empty")
	}
}

func TestNewAddsScheme(t *testing.T) {
	c, err := New("192.168.1.10")
	if err != nil {
		t.Fatal(err)
	}
	if c.baseURL != "http://192.168.1.10" {
		t.Fatalf("baseURL = %q", c.baseURL)
	}
}

func TestNewKeepsScheme(t *testing.T) {
	c, _ := New("https://device:8080")
	if c.baseURL != "https://device:8080" {
		t.Fatalf("baseURL = %q", c.baseURL)
	}
}

func TestCustomHTTPClient(t *testing.T) {
	called := false
	hc := &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		called = true
		return &http.Response{StatusCode: 200, Body: http.NoBody, Header: make(http.Header)}, nil
	})}
	c, err := New("device", WithHTTPClient(hc))
	if err != nil {
		t.Fatal(err)
	}
	_, _ = c.Version(context.Background())
	if !called {
		t.Fatal("custom http client was not used")
	}
}

func TestPasswordHeader(t *testing.T) {
	m := newMockServer(t).withPassword("secret")
	c, err := New("", WithBaseURL(m.server().URL), WithPassword("secret"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Version(context.Background()); err != nil {
		t.Fatalf("Version: %v", err)
	}
	if got := m.lastRequest().Password; got != "secret" {
		t.Fatalf("password header = %q", got)
	}
}

func TestWrongPassword(t *testing.T) {
	m := newMockServer(t).withPassword("secret")
	c, _ := New("", WithBaseURL(m.server().URL), WithPassword("wrong"))
	_, err := c.Version(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusForbidden {
		t.Fatalf("err = %v, want *APIError with 403 (StatusForbidden)", err)
	}
}

func TestVersion(t *testing.T) {
	m := newMockServer(t)
	c := newTestClient(t, m)
	v, err := c.Version(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v.Version != "0.1" {
		t.Fatalf("version = %q", v.Version)
	}
}

func TestHelp(t *testing.T) {
	m := newMockServer(t)
	c := newTestClient(t, m)
	got, err := c.Help(context.Background(), "reset")
	if err != nil {
		t.Fatal(err)
	}
	if got != "Help text for reset" {
		t.Fatalf("got = %q, want 'Help text for reset'", got)
	}
}

func TestInfo(t *testing.T) {
	m := newMockServer(t)
	c := newTestClient(t, m)
	info, err := c.Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.Product != "Ultimate 64" {
		t.Fatalf("product = %q", info.Product)
	}
	if info.CoreVersion != "143" {
		t.Fatalf("core = %q", info.CoreVersion)
	}
}

func TestMachineActions(t *testing.T) {
	m := newMockServer(t)
	c := newTestClient(t, m)
	ctx := context.Background()
	for _, tc := range []struct {
		name string
		fn   func(context.Context) error
		path string
	}{
		{"Reset", c.Machine.Reset, "/v1/machine:reset"},
		{"Reboot", c.Machine.Reboot, "/v1/machine:reboot"},
		{"Pause", c.Machine.Pause, "/v1/machine:pause"},
		{"Resume", c.Machine.Resume, "/v1/machine:resume"},
		{"PowerOff", c.Machine.PowerOff, "/v1/machine:poweroff"},
		{"MenuButton", c.Machine.MenuButton, "/v1/machine:menu_button"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.fn(ctx); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if got := m.lastRequest(); got.Method != http.MethodPut || got.Path != tc.path {
				t.Fatalf("%s: method/path = %s %q, want PUT %q", tc.name, got.Method, got.Path, tc.path)
			}
		})
	}
}

func TestReadMemory(t *testing.T) {
	m := newMockServer(t)
	m.setMem(0xD020, []byte{0x05})
	c := newTestClient(t, m)
	data, err := c.Machine.ReadMemory(context.Background(), 0xD020, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 1 || data[0] != 0x05 {
		t.Fatalf("data = %v", data)
	}
	if q := m.lastRequest().FullPath; !strings.Contains(q, "address=D020") {
		t.Fatalf("path = %q", q)
	}
}

func TestReadMemoryDefaultLength(t *testing.T) {
	m := newMockServer(t)
	c := newTestClient(t, m)
	if _, err := c.Machine.ReadMemory(context.Background(), 0x0400, 0); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(m.lastRequest().FullPath, "length=") {
		t.Fatalf("length should be omitted, got %q", m.lastRequest().FullPath)
	}
}

func TestWriteMemorySmallPut(t *testing.T) {
	m := newMockServer(t)
	c := newTestClient(t, m)
	if err := c.Machine.WriteMemory(context.Background(), 0xD020, []byte{0x05, 0x04}); err != nil {
		t.Fatal(err)
	}
	r := m.lastRequest()
	if r.Method != http.MethodPut {
		t.Fatalf("method = %s, want PUT", r.Method)
	}
	if !strings.Contains(r.FullPath, "data=0504") {
		t.Fatalf("path = %q", r.FullPath)
	}
}

func TestWriteMemoryLargePost(t *testing.T) {
	m := newMockServer(t)
	c := newTestClient(t, m)
	data := bytes.Repeat([]byte{0xAA}, 200)
	if err := c.Machine.WriteMemory(context.Background(), 0x0400, data); err != nil {
		t.Fatal(err)
	}
	r := m.lastRequest()
	if r.Method != http.MethodPost {
		t.Fatalf("method = %s, want POST", r.Method)
	}
	if !r.HasBody {
		t.Fatal("expected binary body")
	}
	if !bytes.Equal(r.Body, data) {
		t.Fatalf("body mismatch")
	}
}

func TestWriteMemoryOverflow(t *testing.T) {
	m := newMockServer(t)
	c := newTestClient(t, m)
	if err := c.Machine.WriteMemory(context.Background(), 0xFF00, bytes.Repeat([]byte{1}, 300)); err == nil {
		t.Fatal("expected overflow error")
	}
}

func TestWriteMemoryEmpty(t *testing.T) {
	m := newMockServer(t)
	c := newTestClient(t, m)
	if err := c.Machine.WriteMemory(context.Background(), 0x0400, nil); err == nil {
		t.Fatal("expected error for empty write")
	}
}

func TestDebugRegister(t *testing.T) {
	m := newMockServer(t)
	m.debugReg = 0x42
	c := newTestClient(t, m)
	v, err := c.Machine.ReadDebugRegister(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v != 0x42 {
		t.Fatalf("debug reg = %02X", v)
	}
	got, err := c.Machine.WriteDebugRegister(context.Background(), 0x99)
	if err != nil {
		t.Fatal(err)
	}
	if got != 0x99 {
		t.Fatalf("debug reg = %02X", got)
	}
}

func TestMeasureBus(t *testing.T) {
	m := newMockServer(t)
	c := newTestClient(t, m)
	data, err := c.Machine.MeasureBus(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "$timescale") {
		t.Fatalf("data = %q", data)
	}
}

func TestRunners(t *testing.T) {
	m := newMockServer(t)
	c := newTestClient(t, m)
	ctx := context.Background()
	if err := c.Runners.LoadPRG(ctx, "/g/game.prg"); err != nil {
		t.Fatal(err)
	}
	if m.lastRequest().Method != http.MethodPut || !strings.Contains(m.lastRequest().FullPath, "load_prg") {
		t.Fatalf("bad request: %+v", m.lastRequest())
	}
	if err := c.Runners.RunPRG(ctx, "/g/game.prg"); err != nil {
		t.Fatal(err)
	}
	if err := c.Runners.RunCRT(ctx, "/g/cart.crt"); err != nil {
		t.Fatal(err)
	}
	if err := c.Runners.PlaySID(ctx, "/m/sid.sid", 3); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m.lastRequest().FullPath, "songnr=3") {
		t.Fatalf("expected songnr=3, got %q", m.lastRequest().FullPath)
	}
	if err := c.Runners.PlaySID(ctx, "/m/sid.sid", 0); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(m.lastRequest().FullPath, "songnr") {
		t.Fatalf("songnr should be omitted, got %q", m.lastRequest().FullPath)
	}
	if err := c.Runners.PlayMOD(ctx, "/m/mod.mod"); err != nil {
		t.Fatal(err)
	}

	// Upload variants.
	if err := c.Runners.LoadPRGBytes(ctx, []byte("PRG")); err != nil {
		t.Fatal(err)
	}
	if m.lastRequest().Method != http.MethodPost || !m.lastRequest().HasBody {
		t.Fatalf("expected POST with body")
	}
	if err := c.Runners.RunPRGBytes(ctx, []byte("PRG")); err != nil {
		t.Fatal(err)
	}
	if err := c.Runners.RunCRTBytes(ctx, []byte("CRT")); err != nil {
		t.Fatal(err)
	}
	if err := c.Runners.PlaySIDBytes(ctx, []byte("SID"), 2); err != nil {
		t.Fatal(err)
	}
	if err := c.Runners.PlayMODBytes(ctx, []byte("MOD")); err != nil {
		t.Fatal(err)
	}
}

func TestDrives(t *testing.T) {
	m := newMockServer(t)
	c := newTestClient(t, m)
	ctx := context.Background()
	d, err := c.Drives.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if d.A == nil || d.A.BusID != 8 {
		t.Fatalf("drive A = %+v", d.A)
	}
	if d.B != nil {
		t.Fatal("expected nil drive B")
	}

	if err := c.Drives.Mount(ctx, DriveA, "/disks/game.d64", MountOptions{ImageType: ImageD64, Mode: MountReadOnly}); err != nil {
		t.Fatal(err)
	}
	r := m.lastRequest()
	if !strings.Contains(r.FullPath, ":mount") || !strings.Contains(r.FullPath, "image=%2Fdisks%2Fgame.d64") || !strings.Contains(r.FullPath, "type=d64") || !strings.Contains(r.FullPath, "mode=readonly") {
		t.Fatalf("mount path = %q", r.FullPath)
	}

	if err := c.Drives.MountBytes(ctx, DriveB, []byte("D64"), MountOptions{}); err != nil {
		t.Fatal(err)
	}
	if m.lastRequest().Method != http.MethodPost {
		t.Fatal("expected POST for MountBytes")
	}

	for _, tc := range []struct {
		name string
		fn   func() error
		path string
	}{
		{"Unmount", func() error { return c.Drives.Unmount(ctx, DriveA) }, "/v1/drives/a:remove"},
		{"ResetDrive", func() error { return c.Drives.ResetDrive(ctx, DriveA) }, "/v1/drives/a:reset"},
		{"TurnOn", func() error { return c.Drives.TurnOn(ctx, DriveA) }, "/v1/drives/a:on"},
		{"TurnOff", func() error { return c.Drives.TurnOff(ctx, DriveA) }, "/v1/drives/a:off"},
		{"Unlink", func() error { return c.Drives.Unlink(ctx, DriveA) }, "/v1/drives/a:unlink"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.fn(); err != nil {
				t.Fatal(err)
			}
			if got := m.lastRequest(); got.Method != http.MethodPut || got.Path != tc.path {
				t.Fatalf("got %s %q, want PUT %q", got.Method, got.Path, tc.path)
			}
		})
	}

	if err := c.Drives.SetMode(ctx, DriveA, DriveMode1581); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m.lastRequest().FullPath, "mode=1581") {
		t.Fatalf("set_mode = %q", m.lastRequest().FullPath)
	}

	if err := c.Drives.LoadROM(ctx, DriveA, "/roms/1541.rom"); err != nil {
		t.Fatal(err)
	}
	if err := c.Drives.LoadROMBytes(ctx, DriveA, []byte("ROM")); err != nil {
		t.Fatal(err)
	}
	if m.lastRequest().Method != http.MethodPost {
		t.Fatal("expected POST for LoadROMBytes")
	}
}

func TestConfigs(t *testing.T) {
	m := newMockServer(t)
	c := newTestClient(t, m)
	ctx := context.Background()

	cats, err := c.Configs.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(cats) != 2 {
		t.Fatalf("categories = %v", cats)
	}

	cfg, err := c.Configs.Get(ctx, "Drive A Settings")
	if err != nil {
		t.Fatal(err)
	}
	items := cfg["Drive A Settings"]
	if items["Drive"] != "Enabled" {
		t.Fatalf("Drive = %v", items["Drive"])
	}

	// Test safe lookup methods
	val, ok := cfg.Get("Drive A Settings", "Drive")
	if !ok || val != "Enabled" {
		t.Fatalf("cfg.Get failed: val=%v, ok=%v", val, ok)
	}
	if _, ok := cfg.Get("Drive A Settings", "Nonexistent"); ok {
		t.Fatal("cfg.Get succeeded for nonexistent item")
	}
	if _, ok := cfg.Get("Nonexistent", "Drive"); ok {
		t.Fatal("cfg.Get succeeded for nonexistent category")
	}

	detail, err := c.Configs.GetItem(ctx, "Drive A Settings", "Drive Bus ID")
	if err != nil {
		t.Fatal(err)
	}
	item := detail["Drive A Settings"]["Drive Bus ID"]
	if item.Current != float64(8) || item.Min != 8 || item.Max != 11 {
		t.Fatalf("item = %+v", item)
	}

	det, ok := detail.Get("Drive A Settings", "Drive Bus ID")
	if !ok || det.Current != float64(8) {
		t.Fatalf("detail.Get failed: det=%+v, ok=%v", det, ok)
	}
	if _, ok := detail.Get("Drive A Settings", "Nonexistent"); ok {
		t.Fatal("detail.Get succeeded for nonexistent item")
	}

	if err := c.Configs.Set(ctx, "Drive A Settings", "Drive Bus ID", "9"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m.lastRequest().FullPath, "value=9") {
		t.Fatalf("set path = %q", m.lastRequest().FullPath)
	}

	if err := c.Configs.Apply(ctx, ConfigMap{"Drive A Settings": {"Drive": "Disabled"}}); err != nil {
		t.Fatal(err)
	}
	r := m.lastRequest()
	if r.Method != http.MethodPost || !r.HasBody {
		t.Fatalf("expected POST with body, got %+v", r)
	}

	if err := c.Configs.SaveToFlash(ctx, ConfigOptions{}); err != nil {
		t.Fatal(err)
	}
	if m.lastRequest().FullPath != "/v1/configs:save_to_flash" {
		t.Fatalf("save path = %q", m.lastRequest().FullPath)
	}
	if err := c.Configs.SaveToFlash(ctx, ConfigOptions{Category: "Drive A*"}); err != nil {
		t.Fatal(err)
	}
	if m.lastRequest().FullPath != "/v1/configs/Drive%20A*:save_to_flash" {
		t.Fatalf("save path = %q", m.lastRequest().FullPath)
	}
	if err := c.Configs.LoadFromFlash(ctx, ConfigOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := c.Configs.ResetToDefault(ctx, ConfigOptions{}); err != nil {
		t.Fatal(err)
	}
}

func TestStreams(t *testing.T) {
	m := newMockServer(t)
	c := newTestClient(t, m)
	ctx := context.Background()
	if err := c.Streams.Start(ctx, StreamVideo, "192.168.1.10"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m.lastRequest().FullPath, "/v1/streams/video:start") || !strings.Contains(m.lastRequest().FullPath, "ip=192.168.1.10") {
		t.Fatalf("start path = %q", m.lastRequest().FullPath)
	}
	if err := c.Streams.Stop(ctx, StreamAudio); err != nil {
		t.Fatal(err)
	}
	if m.lastRequest().FullPath != "/v1/streams/audio:stop" {
		t.Fatalf("stop path = %q", m.lastRequest().FullPath)
	}
}

func TestFiles(t *testing.T) {
	m := newMockServer(t)
	c := newTestClient(t, m)
	ctx := context.Background()
	info, err := c.Files.Info(ctx, "/games/game.prg")
	if err != nil {
		t.Fatal(err)
	}
	if info.Size != 12345 || info.Extension != "prg" {
		t.Fatalf("info = %+v", info)
	}
	if err := c.Files.CreateD64(ctx, "/disks/new.d64", CreateOptions{Tracks: 40, DiskName: "MYDISK"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m.lastRequest().FullPath, "tracks=40") || !strings.Contains(m.lastRequest().FullPath, "diskname=MYDISK") {
		t.Fatalf("create_d64 path = %q", m.lastRequest().FullPath)
	}
	if err := c.Files.CreateD71(ctx, "/disks/new.d71", CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := c.Files.CreateD71(ctx, "/disks/new.d71", CreateOptions{Tracks: 40}); err == nil {
		t.Fatal("expected error: CreateD71 does not support tracks")
	}
	if err := c.Files.CreateD81(ctx, "/disks/new.d81", CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := c.Files.CreateD81(ctx, "/disks/new.d81", CreateOptions{Tracks: 40}); err == nil {
		t.Fatal("expected error: CreateD81 does not support tracks")
	}
	if err := c.Files.CreateDNP(ctx, "/disks/new.dnp", CreateOptions{Tracks: 100}); err != nil {
		t.Fatal(err)
	}
	if err := c.Files.CreateDNP(ctx, "/disks/new.dnp", CreateOptions{}); err == nil {
		t.Fatal("expected error: DNP requires tracks")
	}
}

func TestErrorsAreTyped(t *testing.T) {
	m := newMockServer(t)
	c := newTestClient(t, m)
	// Hit an unknown endpoint by using a path that the mock returns 404 for.
	// Reuse Files.Info on a path that the mock always answers 200 for; instead
	// test 404 mapping via a dedicated mock with 404 bodies by overriding drivesJSON.
	m.drivesJSON = `{"drives":[],"errors":["boom"]}`
	_, err := c.Drives.List(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if !contains(apiErr.Errors, "boom") {
		t.Fatalf("apiErr.Errors = %v", apiErr.Errors)
	}
}

func contains(s []string, v string) bool {
	return slices.Contains(s, v)
}

// roundTripperFunc adapts a function into an http.RoundTripper.
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Ensure ConfigMap marshals as expected JSON (sanity check).
func TestConfigMapMarshal(t *testing.T) {
	m := ConfigMap{"A": {"x": 1, "y": "z"}}
	b, _ := json.Marshal(m)
	if !bytes.Contains(b, []byte(`"x":1`)) || !bytes.Contains(b, []byte(`"y":"z"`)) {
		t.Fatalf("marshal = %s", b)
	}
}

func TestStatus203Success(t *testing.T) {
	hc := &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 203, Body: http.NoBody, Header: make(http.Header)}, nil
	})}
	c, err := New("device", WithHTTPClient(hc))
	if err != nil {
		t.Fatal(err)
	}
	err = c.Machine.Reset(context.Background())
	if err != nil {
		t.Fatalf("expected Reset to succeed on HTTP 203, got %v", err)
	}
}

func TestDebugBitmap(t *testing.T) {
	m := newMockServer(t)
	c := newTestClient(t, m)

	// Configure machine state in mock memory:
	// Set BMM bit (bit 5) in $D011: 0x20
	m.setMem(0xD011, []byte{0x20})
	// Set MCM bit (bit 4) in $D016: 0x10 (multicolor bitmap)
	m.setMem(0xD016, []byte{0x10})
	// VIC-II memory setup $D018: bitmap at offset 0x2000 (bit 3 set), screen at offset 0x0400 (bits 4-7 = 1) -> 0x18
	m.setMem(0xD018, []byte{0x18})
	// CIA2 $DD00: bank 0 (value 3)
	m.setMem(0xDD00, []byte{0x03})
	// Background color $D021: 9
	m.setMem(0xD021, []byte{0x09})

	// Set some mock data in screenColors and bitmap
	// Screen offset is 0x0400, length is 1000. Let's set screen color byte at 0x0400 to 0x25
	m.setMem(0x0400, []byte{0x25})
	// Bitmap offset is 0x2000. Let's set bitmap byte at 0x2000 to 0x0F
	m.setMem(0x2000, []byte{0x0F})

	img, err := c.Debug.Bitmap(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if img.Bounds() != image.Rect(0, 0, 320, 200) {
		t.Fatalf("unexpected bounds: %v", img.Bounds())
	}

	// Test when not in bitmap mode:
	m.setMem(0xD011, []byte{0x00})
	_, err = c.Debug.Bitmap(context.Background())
	if err == nil {
		t.Fatal("expected error when not in bitmap mode")
	}
}

func TestDebugSprites(t *testing.T) {
	m := newMockServer(t)
	c := newTestClient(t, m)

	m.setMem(0xDD00, []byte{0x03})
	ptrs := make([]byte, 8)
	ptrs[3] = 0x10
	m.setMem(0x07F8, ptrs)

	vic := make([]byte, 47)
	vic[3*2] = 100
	vic[3*2+1] = 50
	vic[0x15] = 0x08
	vic[0x27+3] = 2
	m.setMem(0xD000, vic)

	raw := make([]byte, 64)
	raw[0] = 0b10000000
	m.setMem(0x0400, raw)

	sprites, err := c.Debug.Sprites(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(sprites) != 8 {
		t.Fatalf("expected 8 sprites, got %d", len(sprites))
	}

	s3 := sprites[3]
	if !s3.Enabled || s3.Color != 2 {
		t.Fatalf("unexpected sprite properties: %+v", s3)
	}

	img, err := s3.Image()
	if err != nil {
		t.Fatalf("Image(): %v", err)
	}
	if got := img.ColorIndexAt(0, 0); got != 1 {
		t.Errorf("got %d, want 1", got)
	}
}

func TestPeekPoke(t *testing.T) {
	m := newMockServer(t)
	c := newTestClient(t, m)

	m.setMem(0xD020, []byte{0x05})

	val, err := c.Machine.Peek(context.Background(), 0xD020)
	if err != nil {
		t.Fatal(err)
	}
	if val != 0x05 {
		t.Fatalf("peek = %02X, want 05", val)
	}

	if err := c.Machine.Poke(context.Background(), 0xD020, 0x07); err != nil {
		t.Fatal(err)
	}

	val, _ = c.Machine.Peek(context.Background(), 0xD020)
	if val != 0x07 {
		t.Fatalf("peek after poke = %02X, want 07", val)
	}
}

func TestKeyboardType(t *testing.T) {
	m := newMockServer(t)
	c := newTestClient(t, m)

	m.setMem(0x00C6, []byte{0})
	m.setMem(0xDC00, []byte{0xFF, 0xFF, 0xFF, 0x00})

	if err := c.Keyboard.Type(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	wrote := m.mem[0x0277 : 0x0277+5]
	m.mu.Unlock()
	expected := []byte{0x48, 0x45, 0x4C, 0x4C, 0x4F}
	if !bytes.Equal(wrote, expected) {
		t.Fatalf("Type wrote = %v, want %v", wrote, expected)
	}

	if err := c.Keyboard.Press(context.Background(), c64.KeySpace); err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	restored := m.mem[0xDC00]
	m.mu.Unlock()
	if restored != 0xFF {
		t.Fatalf("CIA port A = $%02X, want $FF restored after Press", restored)
	}
}

func TestKeyboardPressSelectiveDDR(t *testing.T) {
	m := newMockServer(t)
	c := newTestClient(t, m)

	// Pre-fill CIA registers: $DC00=0xFF, $DC01=0xFF, $DC02=0xFF, $DC03=0x00
	m.setMem(0xDC00, []byte{0xFF, 0xFF, 0xFF, 0x00})

	// KeySpace: Column = 0x7F, Row = 0xEF (Row 4 active bit is 0x10)
	if err := c.Keyboard.Press(context.Background(), c64.KeySpace); err != nil {
		t.Fatal(err)
	}

	m.mu.Lock()
	restoredPortA := m.mem[0xDC00]
	restoredDDRB := m.mem[0xDC03]
	m.mu.Unlock()

	if restoredPortA != 0xFF {
		t.Fatalf("CIA port A = $%02X, want $FF restored after Press", restoredPortA)
	}
	if restoredDDRB != 0x00 {
		t.Fatalf("CIA DDRB = $%02X, want $00 restored after Press", restoredDDRB)
	}
}

func TestRawClient(t *testing.T) {
	// Spin up a test TCP server on localhost
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()

	addr := l.Addr().String()

	// Create client pointing at this TCP listener for Raw connection
	client, err := New(addr, WithPassword("secret"))
	if err != nil {
		t.Fatal(err)
	}
	// override base URL so HTTP calls don't fire to it (since it's a TCP raw server)
	client.baseURL = "http://127.0.0.1:0"

	go func() {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		// 1. Read authentication cmd frame
		// header: 2 bytes cmd, 2 bytes len
		header := make([]byte, 4)
		if _, err := conn.Read(header); err != nil {
			return
		}
		cmd := binary.LittleEndian.Uint16(header[0:2])
		payloadLen := binary.LittleEndian.Uint16(header[2:4])

		if cmd != 0xFF1F || payloadLen != 6 {
			return
		}

		payload := make([]byte, payloadLen)
		if _, err := conn.Read(payload); err != nil {
			return
		}
		if string(payload) != "secret" {
			_, _ = conn.Write([]byte{0}) // fail
			return
		}
		_, _ = conn.Write([]byte{1}) // success

		// 2. Read DMA write cmd frame
		if _, err := conn.Read(header); err != nil {
			return
		}
		cmd = binary.LittleEndian.Uint16(header[0:2])
		payloadLen = binary.LittleEndian.Uint16(header[2:4])
		if cmd != 0xFF06 || payloadLen != 5 { // 2 bytes address + 3 bytes data
			return
		}
		payload = make([]byte, payloadLen)
		if _, err := conn.Read(payload); err != nil {
			return
		}
	}()

	conn, err := client.Raw.Dial(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	// Write memory using Raw client
	err = conn.WriteMemory(context.Background(), 0x1000, []byte{1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
}

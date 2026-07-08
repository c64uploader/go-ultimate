package ultimate

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
)

// mockServer is a minimal stub of the Ultimate REST API for testing. It
// records requests and returns canned responses for the endpoints the
// client uses.
type mockServer struct {
	t           testingT
	mu          sync.Mutex
	password    string
	requests    []mockRequest
	mem         []byte // 64K C64 memory
	drivesJSON  string
	versionJSON string
	infoJSON    string
	debugReg    byte
}

// testingT is the subset of *testing.T used, so examples can also drive it.
type testingT interface {
	Helper()
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
	Logf(format string, args ...any)
}

type mockRequest struct {
	Method     string
	Path       string // path only, without query
	FullPath   string // path + query
	Body       []byte
	HasBody    bool
	Password   string
	StatusCode int
}

func newMockServer(t testingT) *mockServer {
	return &mockServer{
		t:           t,
		mem:         make([]byte, 0x10000),
		versionJSON: `{"version":"0.1","errors":[]}`,
		infoJSON:    `{"product":"Ultimate 64","firmware_version":"3.12","fpga_version":"11F","core_version":"143","hostname":"test","unique_id":"8D927F","errors":[]}`,
		drivesJSON:  `{"drives":[{"a":{"enabled":true,"bus_id":8,"type":"1581","rom":"1581.rom","image_file":"","image_path":""}}],"errors":[]}`,
	}
}

// withPassword enables password checking on the mock.
func (m *mockServer) withPassword(pw string) *mockServer { m.password = pw; return m }

func (m *mockServer) server() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(m.handle))
}

// lastRequest returns the most recent request, or fails the test if none.
func (m *mockServer) lastRequest() mockRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.requests) == 0 {
		m.t.Fatalf("no requests recorded")
	}
	return m.requests[len(m.requests)-1]
}

func (m *mockServer) record(r mockRequest) {
	m.mu.Lock()
	m.requests = append(m.requests, r)
	m.mu.Unlock()
}

func (m *mockServer) handle(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	rec := mockRequest{Method: r.Method, Path: r.URL.Path, FullPath: r.URL.RequestURI(), Body: body, HasBody: len(body) > 0, Password: r.Header.Get("X-Password")}

	if m.password != "" && r.Header.Get("X-Password") != m.password {
		rec.StatusCode = http.StatusForbidden
		m.record(rec)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = fmt.Fprint(w, `{"errors":["Forbidden."]}`)
		return
	}

	path := r.URL.Path
	switch {
	case path == "/v1/version" && r.Method == http.MethodGet:
		rec.StatusCode = 200
		m.record(rec)
		writeJSON(w, 200, m.versionJSON)
	case path == "/v1/help" && r.Method == http.MethodGet:
		rec.StatusCode = 200
		m.record(rec)
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(200)
		_, _ = w.Write([]byte("Help text for " + r.URL.Query().Get("command")))
	case path == "/v1/info" && r.Method == http.MethodGet:
		rec.StatusCode = 200
		m.record(rec)
		writeJSON(w, 200, m.infoJSON)
	case path == "/v1/machine:reset" && r.Method == http.MethodPut:
		m.action(w, rec)
	case path == "/v1/machine:reboot" && r.Method == http.MethodPut:
		m.action(w, rec)
	case path == "/v1/machine:pause" && r.Method == http.MethodPut:
		m.action(w, rec)
	case path == "/v1/machine:resume" && r.Method == http.MethodPut:
		m.action(w, rec)
	case path == "/v1/machine:poweroff" && r.Method == http.MethodPut:
		m.action(w, rec)
	case path == "/v1/machine:menu_button" && r.Method == http.MethodPut:
		m.action(w, rec)
	case strings.HasPrefix(path, "/v1/machine:readmem") && r.Method == http.MethodGet:
		m.readmem(w, r, rec)
	case strings.HasPrefix(path, "/v1/machine:writemem") && r.Method == http.MethodPut:
		m.writememPut(w, r, rec)
	case strings.HasPrefix(path, "/v1/machine:writemem") && r.Method == http.MethodPost:
		m.writememPost(w, r, rec)
	case strings.HasPrefix(path, "/v1/machine:debugreg") && r.Method == http.MethodGet:
		rec.StatusCode = 200
		m.record(rec)
		writeJSON(w, 200, fmt.Sprintf(`{"value":"%02X","errors":[]}`, m.debugReg))
	case strings.HasPrefix(path, "/v1/machine:debugreg") && r.Method == http.MethodPut:
		m.writeDebugReg(w, r, rec)
	case strings.HasPrefix(path, "/v1/machine:measure") && r.Method == http.MethodGet:
		rec.StatusCode = 200
		m.record(rec)
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(200)
		_, _ = w.Write([]byte("$timescale 20ns\n"))
	case strings.HasPrefix(path, "/v1/runners:") && r.Method == http.MethodPut:
		m.action(w, rec)
	case strings.HasPrefix(path, "/v1/runners:") && r.Method == http.MethodPost:
		m.action(w, rec)
	case path == "/v1/drives" && r.Method == http.MethodGet:
		rec.StatusCode = 200
		m.record(rec)
		writeJSON(w, 200, m.drivesJSON)
	case strings.HasPrefix(path, "/v1/drives/") && r.Method == http.MethodPut:
		m.action(w, rec)
	case strings.HasPrefix(path, "/v1/drives/") && r.Method == http.MethodPost:
		m.action(w, rec)
	case path == "/v1/configs" && r.Method == http.MethodGet:
		rec.StatusCode = 200
		m.record(rec)
		writeJSON(w, 200, `{"categories":["Drive A Settings","Network settings"],"errors":[]}`)
	case strings.HasPrefix(path, "/v1/configs/") && r.Method == http.MethodGet:
		m.configsGet(w, r, rec)
	case strings.HasPrefix(path, "/v1/configs/") && r.Method == http.MethodPut:
		m.action(w, rec)
	case path == "/v1/configs" && r.Method == http.MethodPost:
		m.action(w, rec)
	case strings.HasPrefix(path, "/v1/configs:") && r.Method == http.MethodPut:
		m.action(w, rec)
	case strings.HasPrefix(path, "/v1/streams/") && r.Method == http.MethodPut:
		m.action(w, rec)
	case strings.HasPrefix(path, "/v1/files/") && r.Method == http.MethodGet:
		m.filesInfo(w, r, rec)
	case strings.HasPrefix(path, "/v1/files/") && r.Method == http.MethodPut:
		m.action(w, rec)
	default:
		rec.StatusCode = 404
		m.record(rec)
		writeJSON(w, 404, `{"errors":["Not found"]}`)
	}
}

func (m *mockServer) action(w http.ResponseWriter, rec mockRequest) {
	rec.StatusCode = 200
	m.record(rec)
	writeJSON(w, 200, `{"errors":[]}`)
}

func (m *mockServer) readmem(w http.ResponseWriter, r *http.Request, rec mockRequest) {
	addr := parseHexQuery(r, "address")
	length := 256
	if l := r.URL.Query().Get("length"); l != "" {
		_, _ = fmt.Sscanf(l, "%d", &length)
	}
	rec.StatusCode = 200
	m.record(rec)
	m.mu.Lock()
	data := make([]byte, length)
	copy(data, m.mem[addr:int(addr)+length])
	m.mu.Unlock()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(200)
	_, _ = w.Write(data)
}

func (m *mockServer) writememPut(w http.ResponseWriter, r *http.Request, rec mockRequest) {
	addr := parseHexQuery(r, "address")
	data := decodeHex(r.URL.Query().Get("data"))
	m.mu.Lock()
	copy(m.mem[addr:], data)
	m.mu.Unlock()
	rec.StatusCode = 200
	m.record(rec)
	writeJSON(w, 200, fmt.Sprintf(`{"address":"%04X-%04X","errors":[]}`, addr, addr+uint16(len(data))-1))
}

func (m *mockServer) writememPost(w http.ResponseWriter, r *http.Request, rec mockRequest) {
	addr := parseHexQuery(r, "address")
	m.mu.Lock()
	copy(m.mem[addr:], rec.Body)
	m.mu.Unlock()
	rec.StatusCode = 200
	m.record(rec)
	writeJSON(w, 200, fmt.Sprintf(`{"address":"%04X-%04X","errors":[]}`, addr, addr+uint16(len(rec.Body))-1))
}

func (m *mockServer) writeDebugReg(w http.ResponseWriter, r *http.Request, rec mockRequest) {
	v := parseHexQuery(r, "value")
	m.mu.Lock()
	m.debugReg = byte(v)
	m.mu.Unlock()
	rec.StatusCode = 200
	m.record(rec)
	writeJSON(w, 200, fmt.Sprintf(`{"value":"%02X","errors":[]}`, m.debugReg))
}

func (m *mockServer) configsGet(w http.ResponseWriter, r *http.Request, rec mockRequest) {
	// path after /v1/configs/
	rest := strings.TrimPrefix(r.URL.Path, "/v1/configs/")
	rest, _ = url.PathUnescape(rest)
	segments := strings.Split(strings.TrimPrefix(rest, "/"), "/")
	switch len(segments) {
	case 1: // category -> items with values
		rec.StatusCode = 200
		m.record(rec)
		writeJSON(w, 200, `{"Drive A Settings":{"Drive":"Enabled","Drive Type":"1541","Drive Bus ID":8},"errors":[]}`)
	case 2: // category/item -> detailed item
		rec.StatusCode = 200
		m.record(rec)
		writeJSON(w, 200, `{"Drive A Settings":{"Drive Bus ID":{"current":8,"min":8,"max":11,"format":"%d","default":8}},"errors":[]}`)
	default:
		rec.StatusCode = 404
		m.record(rec)
		writeJSON(w, 404, `{"errors":["not found"]}`)
	}
}

func (m *mockServer) filesInfo(w http.ResponseWriter, r *http.Request, rec mockRequest) {
	rec.StatusCode = 200
	m.record(rec)
	writeJSON(w, 200, `{"files":{"path":"/games/game.prg","filename":"game.prg","size":12345,"extension":"prg"},"errors":[]}`)
}

func writeJSON(w http.ResponseWriter, code int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = fmt.Fprint(w, body)
}

func parseHexQuery(r *http.Request, key string) uint16 {
	s := r.URL.Query().Get(key)
	var v uint16
	_, _ = fmt.Sscanf(s, "%X", &v)
	return v
}

func decodeHex(s string) []byte {
	out := make([]byte, 0, len(s)/2)
	for i := 0; i+1 < len(s); i += 2 {
		var b byte
		_, _ = fmt.Sscanf(s[i:i+2], "%x", &b)
		out = append(out, b)
	}
	return out
}

// setMem writes b into the mock memory at addr, for test setup.
func (m *mockServer) setMem(addr uint16, b []byte) {
	m.mu.Lock()
	copy(m.mem[addr:], b)
	m.mu.Unlock()
}

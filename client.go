// HTTP client, authentication, and request helpers.

package ultimate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is an HTTP connection to a C64 Ultimate device.
// Create one with New; each exported field is a service group for one area of the API.
type Client struct {
	baseURL    string
	host       string
	password   string
	timeout    time.Duration
	httpClient *http.Client

	Machine  *MachineService  // C64 power state and DMA memory access
	Runners  *RunnersService  // load/run programs and play music
	Drives   *DrivesService   // floppy drives and disk mounting
	Configs  *ConfigsService  // device settings
	Streams  *StreamsService  // UDP video/audio/debug streams (Ultimate 64)
	Files    *FilesService    // device filesystem and blank disk images
	Debug    *DebugService    // decode live C64 hardware state from RAM
	Raw      *RawService      // TCP command socket on port 64
	Keyboard *KeyboardService // inject keystrokes
}

// MemoryReader reads bytes from C64 memory.
type MemoryReader interface {
	ReadMemory(ctx context.Context, address uint16, length int) ([]byte, error)
}

// MemoryWriter writes bytes to C64 memory.
type MemoryWriter interface {
	WriteMemory(ctx context.Context, address uint16, data []byte) error
}

// MemoryReaderWriter combines MemoryReader and MemoryWriter.
type MemoryReaderWriter interface {
	MemoryReader
	MemoryWriter
}

// Option configures a Client.
type Option func(*Client)

// WithPassword sets the password used to authenticate with the device.
func WithPassword(password string) Option {
	return func(c *Client) { c.password = password }
}

// WithHTTPClient sets a custom http.Client. Overrides WithTimeout.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// WithTimeout sets the per-request timeout. Ignored when WithHTTPClient is set.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.timeout = d }
}

// WithBaseURL sets the full device base URL. Used mainly in tests.
func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(u, "/") }
}

// New creates a Client for host (hostname, IP, or URL). http:// is added if missing.
func New(host string, opts ...Option) (*Client, error) {
	c := &Client{timeout: 30 * time.Second}
	for _, opt := range opts {
		opt(c)
	}
	if c.httpClient == nil {
		c.httpClient = &http.Client{Timeout: c.timeout}
	}
	if c.baseURL == "" {
		if host == "" {
			return nil, fmt.Errorf("ultimate: host is required (or use WithBaseURL)")
		}
		if !strings.Contains(host, "://") {
			host = "http://" + host
		}
		c.baseURL = strings.TrimRight(host, "/")
	}

	u, err := url.Parse(c.baseURL)
	if err == nil {
		c.host = u.Host
	}

	c.Machine = &MachineService{client: c}
	c.Runners = &RunnersService{client: c}
	c.Drives = &DrivesService{client: c}
	c.Configs = &ConfigsService{client: c}
	c.Streams = &StreamsService{client: c}
	c.Files = &FilesService{client: c}
	c.Debug = &DebugService{
		Mem: c.Machine,
	}
	c.Raw = &RawService{client: c}
	c.Keyboard = &KeyboardService{Mem: c.Machine}
	return c, nil
}

// apiResponse mirrors the "errors" field present in every device JSON response.
type apiResponse struct {
	Errors []string `json:"errors"`
}

// do executes an HTTP request and returns the raw response body and status code.
// The caller is responsible for interpreting the body.
func (c *Client) do(ctx context.Context, method, path string, body io.Reader, contentType string) (data []byte, status int, err error) {
	u := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, 0, fmt.Errorf("ultimate: build request: %w", err)
	}
	if c.password != "" {
		req.Header.Set("X-Password", c.password)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("ultimate: request: %w", err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			err = errors.Join(err, fmt.Errorf("ultimate: close response body: %w", cerr))
		}
	}()

	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("ultimate: read response: %w", err)
	}
	return data, resp.StatusCode, nil
}

// okStatus reports whether a status code is successful.
// The device returns HTTP 203 for successful calls that have no response body.
func okStatus(status int) bool {
	return status == http.StatusOK || status == http.StatusNonAuthoritativeInfo || status == http.StatusNoContent
}

// doChecked runs a request and maps non-success status codes to errors.
func (c *Client) doChecked(ctx context.Context, method, path string, body io.Reader, contentType string) ([]byte, int, error) {
	data, status, err := c.do(ctx, method, path, body, contentType)
	if err != nil {
		return nil, 0, err
	}
	if okStatus(status) {
		return data, status, nil
	}
	return nil, status, mapError(status, data)
}

// getJSON runs a request, checks the HTTP status, unmarshals the JSON body into result,
// and then checks the firmware-level "errors" array — the device may return HTTP 200
// while still reporting a failure. Pass nil for result to skip unmarshaling.
func (c *Client) getJSON(ctx context.Context, method, path string, body io.Reader, contentType string, result any) error {
	data, status, err := c.doChecked(ctx, method, path, body, contentType)
	if err != nil {
		return err
	}
	// Check firmware-level errors first (the device may return HTTP 200/203 but still report errors).
	var ar apiResponse
	if json.Unmarshal(data, &ar) == nil && len(ar.Errors) > 0 {
		return &APIError{StatusCode: status, Errors: ar.Errors}
	}
	if result != nil {
		if err := json.Unmarshal(data, result); err != nil {
			return fmt.Errorf("ultimate: parse response: %w", err)
		}
	}
	return nil
}

// getRaw runs a request and returns the raw response bytes.
// Used for endpoints that return binary data (memory reads, bus measurements).
func (c *Client) getRaw(ctx context.Context, method, path string, body io.Reader, contentType string) ([]byte, error) {
	data, _, err := c.doChecked(ctx, method, path, body, contentType)
	return data, err
}

// mapError builds an *APIError from a non-success HTTP status, parsing any
// error messages from the response body.
func mapError(status int, data []byte) error {
	e := &APIError{StatusCode: status}
	var ar apiResponse
	if json.Unmarshal(data, &ar) == nil {
		e.Errors = ar.Errors
	}
	return e
}

// encodePath percent-encodes a string for use as a single URL path segment.
// Any '/' inside the string is encoded as %2F so it stays within one segment.
// The firmware decodes each segment separately, so the exact path is preserved.
// The wildcard '*' is left unencoded to match the firmware's wildcard syntax.
func encodePath(path string) string {
	return strings.ReplaceAll(url.PathEscape(path), "%2A", "*")
}

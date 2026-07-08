// API error types.

package ultimate

import (
	"net/http"
	"strconv"
	"strings"
)

// APIError is returned on HTTP or firmware-level failures from the device.
type APIError struct {
	StatusCode int      // HTTP status code (or 0 for success status errors)
	Errors     []string // Messages returned by the device firmware
}

func (e *APIError) Error() string {
	var parts []string
	if e.StatusCode > 0 {
		statusStr := "HTTP " + strconv.Itoa(e.StatusCode)
		if text := http.StatusText(e.StatusCode); text != "" {
			statusStr += " (" + text + ")"
		}
		parts = append(parts, statusStr)
	}
	if len(e.Errors) > 0 {
		parts = append(parts, strings.Join(e.Errors, "; "))
	}
	if len(parts) == 0 {
		return "ultimate: unknown error"
	}
	return "ultimate: " + strings.Join(parts, ": ")
}

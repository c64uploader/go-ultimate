// Device identity, API version, and built-in help text.

package ultimate

import (
	"context"
	"net/http"
	"net/url"
)

// VersionInfo is the REST API version string.
type VersionInfo struct {
	Version string   `json:"version"`
	Errors  []string `json:"errors"`
}

// DeviceInfo is hardware model, firmware versions, and network identity.
type DeviceInfo struct {
	Product         string   `json:"product"`          // e.g. "Ultimate 64"
	FirmwareVersion string   `json:"firmware_version"` // e.g. "3.12"
	FPGAVersion     string   `json:"fpga_version"`     // e.g. "11F"
	CoreVersion     string   `json:"core_version,omitempty"`
	Hostname        string   `json:"hostname"`
	UniqueID        string   `json:"unique_id,omitempty"`
	Errors          []string `json:"errors"`
}

// Version returns the REST API version. Can be used as a connectivity probe.
func (c *Client) Version(ctx context.Context) (*VersionInfo, error) {
	var v VersionInfo
	if err := c.getJSON(ctx, http.MethodGet, "/v1/version", nil, "", &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// Info returns the device model, firmware version, and network details.
func (c *Client) Info(ctx context.Context) (*DeviceInfo, error) {
	var info DeviceInfo
	if err := c.getJSON(ctx, http.MethodGet, "/v1/info", nil, "", &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// Help returns the built-in help text for a device console command.
func (c *Client) Help(ctx context.Context, command string) (string, error) {
	q := url.Values{}
	q.Set("command", command)
	path := "/v1/help?" + q.Encode()
	data, err := c.getRaw(ctx, http.MethodGet, path, nil, "")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

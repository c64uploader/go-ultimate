// Emulated floppy drives: status, mounting, power, and custom ROMs.

package ultimate

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
)

// DrivesService controls emulated 1541/1571/1581 drives and disk mounting.
type DrivesService struct {
	client *Client
}

// Drive is the status of one emulated floppy drive.
type Drive struct {
	Enabled   bool   `json:"enabled"`    // Whether the drive is turned on
	BusID     int    `json:"bus_id"`     // IEC bus ID (typically 8 or 9)
	Type      string `json:"type"`       // Drive model (e.g., "1541", "1571", "1581")
	ROM       string `json:"rom"`        // Custom ROM file name (if loaded)
	ImageFile string `json:"image_file"` // Name of the mounted disk image file
	ImagePath string `json:"image_path"` // Directory path of the mounted image
}

// SoftIECPartition is one directory partition on the software IEC device.
type SoftIECPartition struct {
	ID   int    `json:"id"`
	Path string `json:"path"`
}

// SoftIEC is the software-based IEC storage device (filesystem browser).
type SoftIEC struct {
	Enabled    bool               `json:"enabled"`
	BusID      int                `json:"bus_id"`
	Type       string             `json:"type"`
	LastError  string             `json:"last_error"`
	Partitions []SoftIECPartition `json:"partitions"`
}

// Drives holds the current status of drive slots A, B, and the SoftIEC device.
type Drives struct {
	A       *Drive   // primary drive (typically IEC device 8)
	B       *Drive   // secondary drive (typically IEC device 9)
	SoftIEC *SoftIEC // software IEC filesystem device
}

type driveEntry struct {
	A       *Drive   `json:"a,omitempty"`
	B       *Drive   `json:"b,omitempty"`
	SoftIEC *SoftIEC `json:"softiec,omitempty"`
}

// UnmarshalJSON decodes the device JSON array format into a Drives struct.
func (d *Drives) UnmarshalJSON(data []byte) error {
	var raw struct {
		Drives []driveEntry `json:"drives"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	for _, entry := range raw.Drives {
		if entry.A != nil {
			d.A = entry.A
		}
		if entry.B != nil {
			d.B = entry.B
		}
		if entry.SoftIEC != nil {
			d.SoftIEC = entry.SoftIEC
		}
	}
	return nil
}

// List returns the current status of all floppy drives.
func (s *DrivesService) List(ctx context.Context) (*Drives, error) {
	var resp Drives
	if err := s.client.getJSON(ctx, http.MethodGet, "/v1/drives", nil, "", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// MountOptions sets image format and write mode for a mount operation.
type MountOptions struct {
	ImageType ImageType // Image type (d64, d71, d81, etc.)
	Mode      MountMode // Mount mode (readonly, readwrite, etc.)
}

// Mount mounts an image file from the device filesystem onto drive.
func (s *DrivesService) Mount(ctx context.Context, drive DriveID, image string, opts MountOptions) error {
	return s.mount(ctx, drive, image, opts)
}

// MountBytes uploads image bytes and mounts them on drive.
func (s *DrivesService) MountBytes(ctx context.Context, drive DriveID, data []byte, opts MountOptions) error {
	path := "/v1/drives/" + string(drive) + ":mount" + mountQuery(opts, "")
	return s.client.getJSON(ctx, http.MethodPost, path, bytes.NewReader(data), "application/octet-stream", nil)
}

func (s *DrivesService) mount(ctx context.Context, drive DriveID, image string, opts MountOptions) error {
	path := "/v1/drives/" + string(drive) + ":mount" + mountQuery(opts, image)
	return s.client.getJSON(ctx, http.MethodPut, path, nil, "", nil)
}

// mountQuery builds a URL query string for mount operations.
func mountQuery(opts MountOptions, imageParam string) string {
	q := url.Values{}
	if imageParam != "" {
		q.Set("image", imageParam)
	}
	if opts.ImageType != "" {
		q.Set("type", string(opts.ImageType))
	}
	if opts.Mode != "" {
		q.Set("mode", string(opts.Mode))
	}
	enc := q.Encode()
	if enc == "" {
		return ""
	}
	return "?" + enc
}

// Unmount removes the mounted image from drive.
func (s *DrivesService) Unmount(ctx context.Context, drive DriveID) error {
	return s.client.getJSON(ctx, http.MethodPut, "/v1/drives/"+string(drive)+":remove", nil, "", nil)
}

// ResetDrive resets drive emulation.
func (s *DrivesService) ResetDrive(ctx context.Context, drive DriveID) error {
	return s.client.getJSON(ctx, http.MethodPut, "/v1/drives/"+string(drive)+":reset", nil, "", nil)
}

// TurnOn powers on drive. If already on, it is reset.
func (s *DrivesService) TurnOn(ctx context.Context, drive DriveID) error {
	return s.client.getJSON(ctx, http.MethodPut, "/v1/drives/"+string(drive)+":on", nil, "", nil)
}

// TurnOff powers off drive and removes it from the IEC bus.
func (s *DrivesService) TurnOff(ctx context.Context, drive DriveID) error {
	return s.client.getJSON(ctx, http.MethodPut, "/v1/drives/"+string(drive)+":off", nil, "", nil)
}

// Unlink detaches the image file. Writes are allowed but not saved on unmount.
func (s *DrivesService) Unlink(ctx context.Context, drive DriveID) error {
	return s.client.getJSON(ctx, http.MethodPut, "/v1/drives/"+string(drive)+":unlink", nil, "", nil)
}

// SetMode switches drive emulation model and restores the default ROM.
func (s *DrivesService) SetMode(ctx context.Context, drive DriveID, mode DriveMode) error {
	path := "/v1/drives/" + string(drive) + ":set_mode?mode=" + string(mode)
	return s.client.getJSON(ctx, http.MethodPut, path, nil, "", nil)
}

// LoadROM installs a custom drive ROM from the device filesystem.
// The default ROM returns after a mode change or reboot.
func (s *DrivesService) LoadROM(ctx context.Context, drive DriveID, file string) error {
	path := "/v1/drives/" + string(drive) + ":load_rom?file=" + url.QueryEscape(file)
	return s.client.getJSON(ctx, http.MethodPut, path, nil, "", nil)
}

// LoadROMBytes uploads and installs a custom drive ROM.
// The default ROM returns after a mode change or reboot.
func (s *DrivesService) LoadROMBytes(ctx context.Context, drive DriveID, data []byte) error {
	path := "/v1/drives/" + string(drive) + ":load_rom"
	return s.client.getJSON(ctx, http.MethodPost, path, bytes.NewReader(data), "application/octet-stream", nil)
}

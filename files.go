// Device filesystem metadata and blank disk image creation.

package ultimate

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// FilesService reads file metadata and creates blank disk images on the device.
type FilesService struct {
	client *Client
}

// FileInfo is metadata for one file on the device.
type FileInfo struct {
	Path      string `json:"path"`
	Filename  string `json:"filename"`
	Size      int    `json:"size"`
	Extension string `json:"extension"`
}

// Info returns metadata (size, extension) for a file on the device.
func (s *FilesService) Info(ctx context.Context, path string) (*FileInfo, error) {
	var resp struct {
		Files  FileInfo `json:"files"`
		Errors []string `json:"errors"`
	}
	reqPath := "/v1/files/" + encodePath(path) + ":info"
	if err := s.client.getJSON(ctx, http.MethodGet, reqPath, nil, "", &resp); err != nil {
		return nil, err
	}
	return &resp.Files, nil
}

// CreateOptions sets optional parameters when creating a disk image.
type CreateOptions struct {
	Tracks   int    // Track count (for D64 or DNP custom size formats)
	DiskName string // Custom disk name header/label (up to 16 characters)
}

// CreateD64 creates a blank .d64 disk image (standard 35 tracks).
func (s *FilesService) CreateD64(ctx context.Context, path string, opts CreateOptions) error {
	return s.create(ctx, path, "create_d64", opts)
}

// CreateD71 creates a blank .d71 disk image (70 tracks).
func (s *FilesService) CreateD71(ctx context.Context, path string, opts CreateOptions) error {
	return s.create(ctx, path, "create_d71", opts)
}

// CreateD81 creates a blank .d81 3.5-inch disk image (80 tracks).
func (s *FilesService) CreateD81(ctx context.Context, path string, opts CreateOptions) error {
	return s.create(ctx, path, "create_d81", opts)
}

// CreateDNP creates a blank .dnp partition-based image. Tracks is required.
func (s *FilesService) CreateDNP(ctx context.Context, path string, opts CreateOptions) error {
	return s.create(ctx, path, "create_dnp", opts)
}

func (s *FilesService) create(ctx context.Context, path, command string, opts CreateOptions) error {
	if command == "create_dnp" && opts.Tracks == 0 {
		return fmt.Errorf("ultimate: CreateDNP requires Tracks")
	}
	if opts.Tracks != 0 && (command == "create_d71" || command == "create_d81") {
		return fmt.Errorf("ultimate: %s does not support custom track count", command)
	}
	q := url.Values{}
	if opts.Tracks != 0 {
		q.Set("tracks", strconv.Itoa(opts.Tracks))
	}
	if opts.DiskName != "" {
		q.Set("diskname", opts.DiskName)
	}
	enc := q.Encode()
	reqPath := "/v1/files/" + encodePath(path) + ":" + command
	if enc != "" {
		reqPath += "?" + enc
	}
	return s.client.getJSON(ctx, http.MethodPut, reqPath, nil, "", nil)
}
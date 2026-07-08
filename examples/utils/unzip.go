package utils

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"log/slog"
)

// Unzip extracts the named file from a ZIP archive byte slice.
func Unzip(zipBytes []byte, filename string) ([]byte, error) {
	slog.Info("Extracting file from zip", "filename", filename)
	reader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		slog.Error("failed to read zip archive", "error", err)
		return nil, err
	}
	for _, file := range reader.File {
		if file.Name == filename {
			rc, err := file.Open()
			if err != nil {
				slog.Error("failed to open file in zip", "error", err)
				return nil, err
			}
			defer func() { _ = rc.Close() }()
			data, err := io.ReadAll(rc)
			if err != nil {
				slog.Error("failed to read file in zip", "error", err)
				return nil, err
			}
			return data, nil
		}
	}
	slog.Error("file not found in zip", "filename", filename)
	return nil, fmt.Errorf("file %s not found in zip", filename)
}

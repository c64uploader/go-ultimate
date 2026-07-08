// Package utils implements common utilities for the examples.
package utils

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
)

// HTTPGet fetches url and returns its bytes.
// Results are cached in <TMPDIR>/go-ultimate-testdata/<hash> where <hash> is the first 16 hex digits of the SHA256 of the url.
func HTTPGet(url string) ([]byte, error) {
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(url)))[:16]
	cachePath := filepath.Join(os.TempDir(), "go-ultimate-testdata", hash)
	if data, err := os.ReadFile(cachePath); err == nil {
		slog.Info("using cached file", "url", url, "cachePath", cachePath)
		return data, nil
	}
	slog.Info("downloading file", "url", url, "cachePath", cachePath)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		slog.Error("failed to download file", "url", url, "status", resp.Status)
		return nil, fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}

	if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
		slog.Error("failed to create cache directory", "error", err)
		return nil, err
	}

	f, err := os.Create(cachePath)
	if err != nil {
		slog.Error("failed to create cache file", "error", err)
		return nil, err
	}
	defer func() { _ = f.Close() }()

	if _, err := io.Copy(f, resp.Body); err != nil {
		slog.Error("failed to write cache file", "error", err)
		return nil, err
	}

	data, err := os.ReadFile(cachePath)
	if err != nil {
		slog.Error("failed to read cache file", "error", err)
		return nil, err
	}
	return data, nil
}

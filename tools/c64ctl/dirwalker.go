package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// WalkOptions configures the parallel directory walk.
type WalkOptions struct {
	Roots       []string
	Extensions  map[string]bool
	Workers     int
	IncludeHidden bool
	Callback    func(path string) bool
}

// WalkFiles walks directories in parallel using a fixed worker pool.
// Workers read directory paths from a channel, process entries, and push
// subdirectories back to the channel for other workers to pick up.
// This avoids per-subdirectory goroutine overhead.
func WalkFiles(opts WalkOptions) ([]string, error) {
	if len(opts.Roots) == 0 {
		return nil, nil
	}
	if opts.Workers <= 0 {
		opts.Workers = 8
	}

	// Normalize extensions
	exts := opts.Extensions
	if exts != nil {
		normalized := make(map[string]bool, len(exts))
		for ext := range exts {
			normalized[strings.ToLower(strings.TrimPrefix(ext, "."))] = true
		}
		exts = normalized
	}

	// Validate and dedup roots
	roots := make([]string, 0, len(opts.Roots))
	seen := make(map[string]bool)
	for _, r := range opts.Roots {
		abs, err := filepath.Abs(r)
		if err != nil {
			continue
		}
		if info, err := os.Stat(abs); err != nil || !info.IsDir() {
			continue
		}
		if !seen[abs] {
			seen[abs] = true
			roots = append(roots, abs)
		}
	}
	if len(roots) == 0 {
		return nil, nil
	}

	var (
		mu      sync.Mutex
		results []string
		wg      sync.WaitGroup
		quit    bool
		quitMu  sync.Mutex
		dirs    = make(chan string, 4096)
	)

	shouldQuit := func() bool {
		quitMu.Lock()
		q := quit
		quitMu.Unlock()
		return q
	}

	// Worker: reads directories from channel, processes entries, pushes subdirs
	worker := func() {
		for dir := range dirs {
			if shouldQuit() {
				wg.Done()
				continue
			}

			entries, err := os.ReadDir(dir)
			if err != nil {
				wg.Done()
				continue
			}

			for _, entry := range entries {
				if shouldQuit() {
					break
				}

				name := entry.Name()
				if !opts.IncludeHidden && strings.HasPrefix(name, ".") {
					continue
				}

				fullPath := filepath.Join(dir, name)

				if entry.IsDir() {
					wg.Add(1)
					select {
					case dirs <- fullPath:
					default:
						// Channel full, process inline to avoid deadlock
						wg.Done()
						walkDir(fullPath, exts, !opts.IncludeHidden, &results, &mu, opts.Callback, shouldQuit, &quit, &quitMu, &wg, dirs)
					}
					continue
				}

				// File: check extension
				if exts != nil {
					ext := strings.ToLower(filepath.Ext(name))
					ext = strings.TrimPrefix(ext, ".")
					if !exts[ext] {
						continue
					}
				}

				mu.Lock()
				if opts.Callback != nil && !opts.Callback(fullPath) {
					quitMu.Lock()
					quit = true
					quitMu.Unlock()
					mu.Unlock()
					wg.Done()
					return
				}
				results = append(results, fullPath)
				mu.Unlock()
			}

			wg.Done()
		}
	}

	// Start workers
	for i := 0; i < opts.Workers; i++ {
		go worker()
	}

	// Seed root directories
	for _, root := range roots {
		wg.Add(1)
		dirs <- root
	}

	wg.Wait()
	close(dirs)

	return results, nil
}

// walkDir recursively walks a directory tree sequentially (no channel dispatch).
// Used as fallback when the parallel channel is full.
func walkDir(
	dir string,
	exts map[string]bool,
	skipHidden bool,
	results *[]string,
	mu *sync.Mutex,
	callback func(string) bool,
	shouldQuit func() bool,
	quit *bool,
	quitMu *sync.Mutex,
	wg *sync.WaitGroup,
	dirs chan string,
) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if shouldQuit() {
			return
		}

		name := entry.Name()
		if skipHidden && strings.HasPrefix(name, ".") {
			continue
		}

		fullPath := filepath.Join(dir, name)

		if entry.IsDir() {
			// Try to dispatch back to the parallel pool, but fall back to
			// sequential if the channel is still full.
			wg.Add(1)
			select {
			case dirs <- fullPath:
			default:
				wg.Done()
				walkDir(fullPath, exts, skipHidden, results, mu, callback, shouldQuit, quit, quitMu, wg, dirs)
			}
			continue
		}

		if exts != nil {
			ext := strings.ToLower(filepath.Ext(name))
			ext = strings.TrimPrefix(ext, ".")
			if !exts[ext] {
				continue
			}
		}

		mu.Lock()
		if callback != nil && !callback(fullPath) {
			quitMu.Lock()
			*quit = true
			quitMu.Unlock()
			mu.Unlock()
			return
		}
		*results = append(*results, fullPath)
		mu.Unlock()
	}
}

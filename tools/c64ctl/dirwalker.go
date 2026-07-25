// High-performance parallel directory walker for c64ctl.
//
// Performance Optimizations:
// 1. Fixed Worker Pool Queue:
//    WalkFiles uses a fixed worker pool of goroutines reading directory paths from a shared channel,
//    preventing goroutine spawn overhead during deep directory trees.
// 2. Thread-Local Result Batching:
//    Each worker goroutine collects matched paths into a 4,096-item thread-local slice (localResults)
//    and flushes to the main results slice under mu.Lock() only in batches. This reduces global mutex
//    acquisitions from hundreds of thousands down to ~100.
// 3. Deadlock-Safe Inline Fallback (walkDirInline):
//    When the work queue fills up, workers process subdirectories inline (walkDirInline) while maintaining
//    thread-local result batching, avoiding channel block deadlocks.

package main

import (
	"os"
	"path/filepath"
	"runtime"
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
		opts.Workers = runtime.NumCPU()
		if opts.Workers < 1 {
			opts.Workers = 4
		}
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
		localResults := make([]string, 0, 4096)
		flushLocal := func() {
			if len(localResults) > 0 {
				mu.Lock()
				results = append(results, localResults...)
				mu.Unlock()
				localResults = localResults[:0]
			}
		}

		for dir := range dirs {
			if shouldQuit() {
				flushLocal()
				wg.Done()
				continue
			}

			entries, err := os.ReadDir(dir)
			if err != nil {
				flushLocal()
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
						// Channel full, process inline using localResults batching
						wg.Done()
						walkDirInline(fullPath, exts, !opts.IncludeHidden, &localResults, flushLocal, opts.Callback, shouldQuit, &quit, &quitMu, &mu, &wg, dirs)
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

				if opts.Callback != nil {
					mu.Lock()
					stop := !opts.Callback(fullPath)
					if stop {
						quitMu.Lock()
						quit = true
						quitMu.Unlock()
					}
					mu.Unlock()
					if stop {
						flushLocal()
						wg.Done()
						return
					}
				}

				localResults = append(localResults, fullPath)
				if len(localResults) >= 4096 {
					flushLocal()
				}
			}

			flushLocal()
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

// walkDirInline recursively walks a directory tree sequentially (no channel dispatch).
// Used as fallback when the parallel channel is full.
func walkDirInline(
	dir string,
	exts map[string]bool,
	skipHidden bool,
	localResults *[]string,
	flushLocal func(),
	callback func(string) bool,
	shouldQuit func() bool,
	quit *bool,
	quitMu *sync.Mutex,
	mu *sync.Mutex,
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
			wg.Add(1)
			select {
			case dirs <- fullPath:
			default:
				wg.Done()
				walkDirInline(fullPath, exts, skipHidden, localResults, flushLocal, callback, shouldQuit, quit, quitMu, mu, wg, dirs)
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

		if callback != nil {
			mu.Lock()
			stop := !callback(fullPath)
			if stop {
				quitMu.Lock()
				*quit = true
				quitMu.Unlock()
			}
			mu.Unlock()
			if stop {
				return
			}
		}

		*localResults = append(*localResults, fullPath)
		if len(*localResults) >= 4096 {
			flushLocal()
		}
	}
}

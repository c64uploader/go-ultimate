// Binary search index library for c64ctl.
//
// Performance Optimizations:
// 1. Binary Cache Layout (C64F v1):
//    Stores packed 1-byte extension IDs (ExtToID) and folder IDs (FolderToID) in contiguous tables
//    to allow O(1) candidate rejection during type (-t) or folder (-f) filtering without path parsing.
// 2. Zero-Allocation String Pool:
//    File paths are stored relative to the collection root in a contiguous byte pool with 32-bit uint
//    offsets. Candidate evaluation reads slice views directly from data without heap string allocations.
// 3. In-Place ASCII Case-Insensitive Matching:
//    ContainsIgnoreCaseBytes performs sub-slice matching directly over raw byte arrays in ASCII,
//    eliminating string lowercasing (strings.ToLower) and heap memory churn.
// 4. Literal Prefix Pre-Filtering:
//    For regex queries, re.LiteralPrefix extracts any required plain prefix substring (if available).
//    Candidate paths must pass ContainsIgnoreCaseBytes for the literal prefix before invoking
//    the regex NFA matcher, eliminating unnecessary re.Match calls.
// 5. Parallel Goroutine Worker Scanning:
//    Index.Find partitions the binary index into chunks across runtime.NumCPU() worker goroutines to
//    execute searches concurrently in parallel.
// 6. Fast Single-Pass Binary Encoder (BuildIndexBytes):
//    Uses fast prefix stripping (strings.TrimPrefix) instead of filepath.Rel (avoiding millions of path
//    allocations) and pre-allocates the exact destination byte array with direct binary.LittleEndian writes.
//
// The binary index file (cache.bin) is created by cmdBuildCache and stored in os.UserCacheDir()/c64ctl/cache.bin.

package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
)

const (
	// IndexMagic is the 4-byte header identifying a c64ctl binary cache.
	IndexMagic = "C64F"
	// IndexVersion is the current binary format version.
	IndexVersion = uint32(1)
)

// SearchDirs lists top-level collection folders scanned by default.
var SearchDirs = []string{"Games", "Demos", "Music", "Discmags", "Tools", "Graphics"}

// CacheExtensions lists file extensions included in the search index.
var CacheExtensions = []string{"prg", "d64", "d71", "d81", "g64", "crt", "tap", "t64", "sid", "mod"}

var cacheExtSet = func() map[string]bool {
	s := make(map[string]bool, len(CacheExtensions))
	for _, e := range CacheExtensions {
		s[e] = true
	}
	return s
}()

// ExtToID maps file extension strings to binary table IDs.
var ExtToID = map[string]uint8{
	"prg": 1,
	"crt": 2,
	"d64": 3,
	"d71": 4,
	"d81": 5,
	"g64": 6,
	"tap": 7,
	"t64": 8,
	"sid": 9,
	"mod": 10,
}

// FolderToID maps top-level collection folder names to binary table IDs.
var FolderToID = map[string]uint8{
	"games":    1,
	"demos":    2,
	"music":    3,
	"discmags": 4,
	"tools":    5,
	"graphics": 6,
}

// WalkCollection scans the collection root directory for supported files using WalkFiles.
func WalkCollection(root string) ([]string, error) {
	roots := make([]string, 0, len(SearchDirs))
	matchedDirs := make([]string, 0, len(SearchDirs))
	for _, dir := range SearchDirs {
		fullPath := filepath.Join(root, dir)
		if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
			roots = append(roots, fullPath)
			matchedDirs = append(matchedDirs, dir)
		}
	}
	if len(roots) == 0 {
		if info, err := os.Stat(root); err == nil && info.IsDir() {
			roots = append(roots, root)
			matchedDirs = append(matchedDirs, filepath.Base(root))
		}
	}

	workers := runtime.NumCPU()
	if workers < 1 {
		workers = 4
	}

	if len(matchedDirs) > 0 {
		fmt.Printf("Scanning %s (%s) using %d worker threads...\n", root, strings.Join(matchedDirs, ", "), workers)
	} else {
		fmt.Printf("Scanning %s using %d worker threads...\n", root, workers)
	}

	return WalkFiles(WalkOptions{
		Roots:      roots,
		Extensions: cacheExtSet,
		Workers:    workers,
	})
}

// BuildIndexFile scans root directory and writes the binary index file to targetPath.
func BuildIndexFile(root string, targetPath string) error {
	allFiles, err := WalkCollection(root)
	if err != nil {
		return err
	}

	binData, err := BuildIndexBytes(allFiles, root)
	if err != nil {
		return fmt.Errorf("encoding binary cache: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(targetPath, binData, 0644); err != nil {
		return err
	}

	fmt.Printf("\nCached %d files to %s (%d KB)\n", len(allFiles), targetPath, len(binData)/1024)
	return nil
}

// FindOptions configures search filters and limit for Index.Find.
type FindOptions struct {
	Query        string
	FilterType   string
	FilterFolder string
	Limit        int
}

// FindResult contains search results and match counts.
type FindResult struct {
	Matches      []string
	TotalMatches int
	Displayed    int
}

// Index represents a zero-allocation binary search index for a file collection.
type Index struct {
	root           string
	data           []byte
	entryCount     int
	extTable       []byte
	folderTable    []byte
	offsetTablePos int
	stringPoolPos  int
	stringPool     []byte
}

// OpenIndex loads an Index from binary cache bytes.
func OpenIndex(data []byte, root string) (*Index, error) {
	if len(data) < 16 {
		return nil, fmt.Errorf("index data too short")
	}
	if string(data[0:4]) != IndexMagic {
		return nil, fmt.Errorf("invalid index magic: %q", string(data[0:4]))
	}
	ver := binary.LittleEndian.Uint32(data[4:8])
	if ver != IndexVersion {
		return nil, fmt.Errorf("unsupported index version %d (expected %d)", ver, IndexVersion)
	}

	entryCount := int(binary.LittleEndian.Uint32(data[8:12]))
	if entryCount < 0 {
		return nil, fmt.Errorf("invalid index entry count %d", entryCount)
	}

	offsetTablePos := 16 + 2*entryCount
	stringPoolPos := offsetTablePos + (entryCount+1)*4

	if len(data) < stringPoolPos {
		return nil, fmt.Errorf("corrupted index data")
	}

	extTable := data[16 : 16+entryCount]
	folderTable := data[16+entryCount : 16+2*entryCount]

	return &Index{
		root:           root,
		data:           data,
		entryCount:     entryCount,
		extTable:       extTable,
		folderTable:    folderTable,
		offsetTablePos: offsetTablePos,
		stringPoolPos:  stringPoolPos,
		stringPool:     data[stringPoolPos:],
	}, nil
}

// LoadIndex reads a binary index file from disk and initializes an Index.
func LoadIndex(path string, root string) (*Index, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return OpenIndex(data, root)
}

// BuildIndexBytes encodes file paths into a binary index byte stream.
func BuildIndexBytes(allFiles []string, root string) ([]byte, error) {
	entryCount := len(allFiles)
	rootPrefix := root
	if !strings.HasSuffix(rootPrefix, string(filepath.Separator)) {
		rootPrefix += string(filepath.Separator)
	}

	// Pass 1: Calculate total string pool size
	totalPoolLen := 0
	for _, file := range allFiles {
		rel := strings.TrimPrefix(file, rootPrefix)
		totalPoolLen += len(rel)
	}

	// Pass 2: Allocate exact contiguous byte array
	headerLen := 16
	extTablePos := headerLen
	folderTablePos := extTablePos + entryCount
	offsetTablePos := folderTablePos + entryCount
	stringPoolPos := offsetTablePos + (entryCount+1)*4
	totalSize := stringPoolPos + totalPoolLen

	buf := make([]byte, totalSize)

	// Write Header
	copy(buf[0:4], IndexMagic)
	binary.LittleEndian.PutUint32(buf[4:8], IndexVersion)
	binary.LittleEndian.PutUint32(buf[8:12], uint32(entryCount))
	binary.LittleEndian.PutUint32(buf[12:16], 0) // reserved

	poolOffset := uint32(0)
	for i, file := range allFiles {
		rel := strings.TrimPrefix(file, rootPrefix)

		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(file)), ".")
		buf[extTablePos+i] = ExtToID[ext]

		topFolder := strings.SplitN(rel, string(filepath.Separator), 2)[0]
		buf[folderTablePos+i] = FolderToID[strings.ToLower(topFolder)]

		binary.LittleEndian.PutUint32(buf[offsetTablePos+i*4:], poolOffset)
		copy(buf[stringPoolPos+int(poolOffset):], rel)
		poolOffset += uint32(len(rel))
	}
	binary.LittleEndian.PutUint32(buf[offsetTablePos+entryCount*4:], poolOffset)

	return buf, nil
}

// Find searches the index using the provided options.
func (idx *Index) Find(opts FindOptions) (FindResult, error) {
	var targetExtID uint8
	if opts.FilterType != "" {
		extKey := strings.TrimPrefix(strings.ToLower(opts.FilterType), ".")
		id, ok := ExtToID[extKey]
		if !ok {
			return FindResult{}, nil
		}
		targetExtID = id
	}

	var targetFolderID uint8
	if opts.FilterFolder != "" {
		folderKey := strings.ToLower(opts.FilterFolder)
		id, ok := FolderToID[folderKey]
		if !ok {
			return FindResult{}, nil
		}
		targetFolderID = id
	}

	var re *regexp.Regexp
	var literalPrefix string
	if opts.Query != "" && strings.ContainsAny(opts.Query, ".*+?^$()[]{}|\\") {
		var err error
		re, err = regexp.Compile("(?i)" + opts.Query)
		if err != nil {
			return FindResult{}, fmt.Errorf("invalid regex query %q: %w", opts.Query, err)
		}
		literalPrefix, _ = re.LiteralPrefix()
	}

	numWorkers := runtime.NumCPU()
	if numWorkers < 1 {
		numWorkers = 1
	}
	if idx.entryCount < 5000 {
		numWorkers = 1
	}

	chunkSize := (idx.entryCount + numWorkers - 1) / numWorkers
	matchedIndices := make([][]int, numWorkers)

	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			startIdx := workerID * chunkSize
			endIdx := startIdx + chunkSize
			if endIdx > idx.entryCount {
				endIdx = idx.entryCount
			}

			var local []int
			for i := startIdx; i < endIdx; i++ {
				if targetExtID != 0 && idx.extTable[i] != targetExtID {
					continue
				}

				if targetFolderID != 0 && idx.folderTable[i] != targetFolderID {
					continue
				}

				relStart := binary.LittleEndian.Uint32(idx.data[idx.offsetTablePos+i*4 : idx.offsetTablePos+(i+1)*4])
				relEnd := binary.LittleEndian.Uint32(idx.data[idx.offsetTablePos+(i+1)*4 : idx.offsetTablePos+(i+2)*4])
				relBytes := idx.stringPool[relStart:relEnd]

				if opts.Query != "" {
					if re != nil {
						if literalPrefix != "" && !ContainsIgnoreCaseBytes(relBytes, literalPrefix) {
							continue
						}
						if !re.Match(relBytes) {
							continue
						}
					} else if !ContainsIgnoreCaseBytes(relBytes, opts.Query) {
						continue
					}
				}

				local = append(local, i)
			}
			matchedIndices[workerID] = local
		}(w)
	}
	wg.Wait()

	var matches []string
	for _, local := range matchedIndices {
		for _, i := range local {
			relStart := binary.LittleEndian.Uint32(idx.data[idx.offsetTablePos+i*4 : idx.offsetTablePos+(i+1)*4])
			relEnd := binary.LittleEndian.Uint32(idx.data[idx.offsetTablePos+(i+1)*4 : idx.offsetTablePos+(i+2)*4])
			relPath := string(idx.stringPool[relStart:relEnd])
			matches = append(matches, filepath.Join(idx.root, relPath))
		}
	}

	total := len(matches)
	displayed := total
	if opts.Limit > 0 && total > opts.Limit {
		displayed = opts.Limit
		matches = matches[:opts.Limit]
	}

	return FindResult{
		Matches:      matches,
		TotalMatches: total,
		Displayed:    displayed,
	}, nil
}

// ContainsIgnoreCaseBytes checks if s contains substr case-insensitively without string allocations.
func ContainsIgnoreCaseBytes(s []byte, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(substr) > len(s) {
		return false
	}

	b0 := substr[0]
	var l0, u0 byte
	if b0 >= 'A' && b0 <= 'Z' {
		u0 = b0
		l0 = b0 + ('a' - 'A')
	} else if b0 >= 'a' && b0 <= 'z' {
		l0 = b0
		u0 = b0 - ('a' - 'A')
	} else {
		l0 = b0
		u0 = b0
	}

	maxIdx := len(s) - len(substr)
	for i := 0; i <= maxIdx; i++ {
		c := s[i]
		if c != l0 && c != u0 {
			continue
		}
		match := true
		for j := 1; j < len(substr); j++ {
			sc := s[i+j]
			subc := substr[j]
			if sc == subc {
				continue
			}
			if sc >= 'A' && sc <= 'Z' {
				sc += 'a' - 'A'
			}
			if subc >= 'A' && subc <= 'Z' {
				subc += 'a' - 'A'
			}
			if sc != subc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func cacheFile() string {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	return filepath.Join(cacheDir, "c64ctl", "cache.txt")
}

var findType string
var findFolder string
var findLimit int

func newFindCmd() *cobra.Command {
	findCmd := &cobra.Command{
		Use:   "find [<query>]",
		Short: "Search local assembly64 collection",
		Long: `Search for files by name across Games, Demos, Music, etc. Uses cache for instant results.

Query is optional — omit it to list all files (use with -t/-f to narrow down).
Use --limit 0 to show all matches (pipe to grep/rg for further filtering).`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			q := ""
			if len(args) > 0 {
				q = args[0]
			}
			return cmdFind(q, findType, findFolder, findLimit)
		},
	}
	findCmd.Flags().StringVarP(&findType, "type", "t", "", "Filter by type: prg, crt, d64, d71, d81, g64, tap, t64, sid, mod")
	findCmd.Flags().StringVarP(&findFolder, "folder", "f", "", "Filter by folder: Games, Demos, Music, Discmags, Tools, Graphics")
	findCmd.Flags().IntVarP(&findLimit, "limit", "l", 30, "Max results to show (0 = no limit)")
	return findCmd
}

var buildCachePath string

func newBuildCacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build-cache",
		Short: "Build/rebuild the file cache for instant search",
		Long:  "Scan Games, Demos, Music, Discmags, Tools, Graphics and build a cache file.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdBuildCache(buildCachePath)
		},
	}
	cmd.Flags().StringVarP(&buildCachePath, "path", "p", envOrDefault("ASSEMBLY64_PATH", assembly64Root()), "Path to assembly64 collection")
	return cmd
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show connection status and current screen",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Connected to C64 Ultimate")
			screen, err := client.Debug.Screen(context.Background())
			if err != nil {
				return err
			}
			fmt.Println()
			for _, row := range screen.Rows {
				fmt.Println(row)
			}
			return nil
		},
	}
}

// cacheExtensions lists all file extensions included in the cache.
var cacheExtensions = []string{"prg", "d64", "d71", "d81", "g64", "crt", "tap", "t64", "sid", "mod"}

// cacheExtSet is cacheExtensions as a set for the Go walker.
var cacheExtSet = func() map[string]bool {
	s := make(map[string]bool, len(cacheExtensions))
	for _, e := range cacheExtensions {
		s[e] = true
	}
	return s
}()

var searchDirs = []string{"Games", "Demos", "Music", "Discmags", "Tools", "Graphics"}

func assembly64Root() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Downloads", "assembly64")
}

func walkCollection(root string) ([]string, error) {
	roots := make([]string, 0, len(searchDirs))
	for _, dir := range searchDirs {
		fullPath := filepath.Join(root, dir)
		if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
			roots = append(roots, fullPath)
		}
	}
	fmt.Printf("Scanning %d directories with Go walker...\n", len(roots))
	return WalkFiles(WalkOptions{
		Roots:      roots,
		Extensions: cacheExtSet,
		Workers:    8,
	})
}

func cmdBuildCache(root string) error {
	allFiles, err := walkCollection(root)
	if err != nil {
		return err
	}

	out := []byte(strings.Join(allFiles, "\n"))
	if err := os.MkdirAll(filepath.Dir(cacheFile()), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(cacheFile(), out, 0644); err != nil {
		return err
	}

	fmt.Printf("\nCached %d files to %s\n", len(allFiles), cacheFile())
	return nil
}

func cmdFind(query string, filterType string, filterFolder string, limit int) error {
	data, err := os.ReadFile(cacheFile())
	if err != nil {
		allFiles, err := walkCollection(assembly64Root())
		if err != nil {
			return err
		}
		data = []byte(strings.Join(allFiles, "\n"))
		// Build cache for next time
		if err := os.MkdirAll(filepath.Dir(cacheFile()), 0755); err == nil {
			os.WriteFile(cacheFile(), data, 0644)
		}
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	query = strings.ToLower(query)
	root := assembly64Root()

	var matches []string
	for _, line := range lines {
		if query != "" && !strings.Contains(strings.ToLower(line), query) {
			continue
		}
		if filterType != "" {
			ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(line)), ".")
			if ext != filterType {
				continue
			}
		}
		if filterFolder != "" {
			rel, err := filepath.Rel(root, line)
			if err == nil {
				topFolder := strings.SplitN(rel, string(filepath.Separator), 2)[0]
				if !strings.EqualFold(topFolder, filterFolder) {
					continue
				}
			}
		}
		matches = append(matches, line)
	}

	if len(matches) == 0 {
		return nil
	}

	if limit > 0 && len(matches) > limit {
		matches = matches[:limit]
	}

	for _, line := range matches {
		fmt.Println(line)
	}
	return nil
}

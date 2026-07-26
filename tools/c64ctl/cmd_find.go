// CLI subcommands for searching local file collections (c64ctl find / build-cache).

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func cacheBinFile() string {
	dir := c64CacheDir
	if dir == "" {
		dir = defaultCacheDir()
	}
	return filepath.Join(dir, "cache.bin")
}

var findType string
var findFolder string
var findLimit int

func newFindCmd() *cobra.Command {
	findCmd := &cobra.Command{
		Use:   "find [<query>]",
		Short: "Search local assembly64 collection",
		Long: `Search for files by name or regex pattern across Games, Demos, Music, etc. Uses cache for instant results.

Query supports case-insensitive egrep/ripgrep-style regex (e.g., "mayhem.*stix" or "stix|karate").
Query is optional - omit it to list all files (use with -t/-f to narrow down).
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
	findCmd.Flags().StringVar(&c64CacheDir, "cache-dir", "", "Directory to store index cache files")
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
			p := buildCachePath
			if p == "" || !cmd.Flags().Changed("path") {
				if c64Assembly64Path != "" {
					p = c64Assembly64Path
				} else {
					p = envOrDefault("C64U_ASSEMBLY64_PATH", assembly64Root())
				}
			}
			return cmdBuildCache(p)
		},
	}
	cmd.Flags().StringVarP(&buildCachePath, "path", "p", "", "Path to assembly64 collection")
	cmd.Flags().StringVar(&c64CacheDir, "cache-dir", "", "Directory to store index cache files")
	return cmd
}



func assembly64Root() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Downloads", "assembly64")
}

func cmdBuildCache(root string) error {
	return BuildIndexFile(root, cacheBinFile())
}

func cmdFind(query string, filterType string, filterFolder string, limit int) error {
	root := c64Assembly64Path
	if root == "" {
		root = envOrDefault("C64U_ASSEMBLY64_PATH", assembly64Root())
	}
	idx, err := LoadIndex(cacheBinFile(), root)

	if err != nil {
		if err := cmdBuildCache(root); err != nil {
			return err
		}
		idx, err = LoadIndex(cacheBinFile(), root)
		if err != nil {
			return fmt.Errorf("reading index file: %w", err)
		}
	}

	res, err := idx.Find(FindOptions{
		Query:        query,
		FilterType:   filterType,
		FilterFolder: filterFolder,
		Limit:        limit,
	})
	if err != nil {
		return err
	}

	if res.TotalMatches == 0 {
		fmt.Fprintln(os.Stderr, "No matches found.")
		return nil
	}

	for _, line := range res.Matches {
		fmt.Println(line)
	}

	if res.TotalMatches > res.Displayed {
		var tips []string
		if filterType == "" {
			tips = append(tips, "-t crt")
		}
		if filterFolder == "" {
			tips = append(tips, "-f Games")
		}
		if len(tips) > 0 {
			fmt.Fprintf(os.Stderr, "\nShowing %d of %d matches (use -l 0 to show all, or filter with e.g. %s)\n", res.Displayed, res.TotalMatches, strings.Join(tips, ", "))
		} else {
			fmt.Fprintf(os.Stderr, "\nShowing %d of %d matches (use -l 0 to show all)\n", res.Displayed, res.TotalMatches)
		}
	}
	return nil
}

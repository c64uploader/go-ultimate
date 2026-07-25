// CLI subcommands for searching local file collections (c64ctl find / build-cache).

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/c64uploader/go-ultimate"
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
	return cmd
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show effective configuration, cache status, and C64 firmware info",
		Long:  "Displays effective c64ctl settings, local directories, search binary cache size, C64 Ultimate connectivity status, and firmware details.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			fmt.Println("c64ctl Status & Configuration")
			fmt.Println("==============================")
			fmt.Println()

			// 1. Effective Configuration
			fmt.Println("Effective Configuration:")
			_, loadedPath, _ := loadConfigFile(configFile)
			if configFile != "" {
				fmt.Printf("  Config File:      %s (specified via --config)\n", configFile)
			} else if loadedPath != "" {
				fmt.Printf("  Config File:      %s (loaded)\n", loadedPath)
			} else {
				fmt.Printf("  Config File:      (none found; using defaults / env / flags)\n")
			}
			fmt.Printf("  Target Host:      %s\n", c64Host)
			fmt.Printf("  User:             %s\n", c64User)
			if c64Password != "" {
				fmt.Printf("  Password:         (set)\n")
			} else {
				fmt.Printf("  Password:         (none)\n")
			}

			// 2. Directories & Cache
			fmt.Println()
			fmt.Println("Directories & Search Cache:")
			fmt.Printf("  Config Directory: %s\n", defaultConfigDir())
			fmt.Printf("  Assembly64 Path:  %s\n", c64Assembly64Path)
			fmt.Printf("  Cache Directory:  %s\n", c64CacheDir)

			cacheFile := cacheBinFile()
			if fi, err := os.Stat(cacheFile); err == nil {
				fmt.Printf("  Binary Cache:     %s (%s)\n", cacheFile, formatBytes(fi.Size()))
			} else {
				fmt.Printf("  Binary Cache:     %s (not built — run 'c64ctl build-cache')\n", cacheFile)
			}

			// 3. Connectivity & Firmware Info
			fmt.Println()
			fmt.Println("C64 Ultimate Connection:")

			var opts []ultimate.Option
			if c64Password != "" {
				opts = append(opts, ultimate.WithPassword(c64Password))
			}
			c, err := ultimate.New(c64Host, opts...)
			if err != nil {
				fmt.Printf("  Status:           Disconnected\n")
				fmt.Printf("  Error:            %v\n", err)
				return nil
			}

			info, err := c.Info(ctx)
			if err != nil {
				fmt.Printf("  Status:           Disconnected (http://%s)\n", c64Host)
				fmt.Printf("  Error:            %v\n", err)
				return nil
			}

			fmt.Printf("  Status:           Connected (http://%s)\n", c64Host)
			if info.Product != "" {
				fmt.Printf("  Product:          %s\n", info.Product)
			}
			if info.FirmwareVersion != "" {
				fmt.Printf("  Firmware Version: %s\n", info.FirmwareVersion)
			}
			if info.FPGAVersion != "" {
				fmt.Printf("  FPGA Version:     %s\n", info.FPGAVersion)
			}
			if info.CoreVersion != "" {
				fmt.Printf("  Core Version:     %s\n", info.CoreVersion)
			}
			if info.Hostname != "" {
				fmt.Printf("  Device Hostname:  %s\n", info.Hostname)
			}
			if info.UniqueID != "" {
				fmt.Printf("  Unique ID:        %s\n", info.UniqueID)
			}

			if ver, err := c.Version(ctx); err == nil && ver.Version != "" {
				fmt.Printf("  REST API Version: %s\n", ver.Version)
			}

			if screen, err := c.Debug.Screen(ctx); err == nil && len(screen.Rows) > 0 {
				fmt.Println()
				fmt.Println("Current Screen:")
				for _, row := range screen.Rows {
					fmt.Printf("  %s\n", row)
				}
			}

			return nil
		},
	}
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB (%d bytes)", float64(bytes)/float64(div), "KMGTPE"[exp], bytes)
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

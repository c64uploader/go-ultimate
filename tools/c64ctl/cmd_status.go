// CLI subcommand for c64ctl status.

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/c64uploader/go-ultimate"
	"github.com/spf13/cobra"
)

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
			cfgFile := configFile
			if cfgFile == "" {
				cfgFile = filepath.Join(defaultConfigDir(), "config.json")
			}
			if _, err := os.Stat(cfgFile); err == nil {
				if configFile != "" {
					fmt.Printf("  Config File:      %s (specified via --config)\n", cfgFile)
				} else {
					fmt.Printf("  Config File:      %s (loaded)\n", cfgFile)
				}
			} else {
				if configFile != "" {
					fmt.Printf("  Config File:      %s (specified via --config, not present)\n", cfgFile)
				} else {
					fmt.Printf("  Config File:      %s (not present)\n", cfgFile)
				}
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
				fmt.Printf("  Binary Cache:     %s (not present)\n", cacheFile)
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

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/c64uploader/go-ultimate/c64"
	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	var entryNum int
	var songNum int

	cmd := &cobra.Command{
		Use:   "run <file> [--entry N] [--song N]",
		Short: "Upload and run a PRG, T64, or SID file",
		Long: `Upload a file to C64 memory and start execution.
Supports .PRG, .T64, and .SID file formats.

Resets the C64, uploads the binary, and jumps to the load address.

For T64 tape archives, the first entry is run by default.
Use --entry to select a different entry.

For SID music files, use --song to select a sub-tune (default: 0).

Note: .TAP files are raw tape waveform images and cannot be run directly.
Use a .T64 file instead (a tape archive that c64ctl run can handle).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]
			ext := strings.ToLower(filepath.Ext(path))

			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			// T64 tape archive: extract entry and run as PRG
			if ext == ".t64" {
				entries, err := c64.ParseT64(data)
				if err != nil {
					return fmt.Errorf("parsing T64: %w", err)
				}
				if entryNum < 0 || entryNum >= len(entries) {
					return fmt.Errorf("entry %d out of range (0-%d)", entryNum, len(entries)-1)
				}
				entry := entries[entryNum]
				prg := entry.Program()
				fmt.Printf("T64: %s — %s (%d bytes)\n",
					filepath.Base(path), entry.Name, prg.Size())
				fmt.Printf("Running entry %d at $%04X...\n", entryNum, prg.LoadAddress())
				return client.Runners.Run(context.Background(), prg)
			}

			// TAP tape image can't be run as PRG
			if ext == ".tap" {
				return fmt.Errorf(".TAP is a raw tape waveform image, not a program. Did you mean a .T64 file?")
			}

			// SID music file
			if ext == ".sid" {
				fmt.Printf("Playing %s (song %d)...\n", filepath.Base(path), songNum)
				return client.Runners.PlaySIDBytes(context.Background(), data, songNum)
			}

			// Plain PRG
			fmt.Printf("Uploading %s (%d bytes)...\n", filepath.Base(path), len(data))
			if err := client.Runners.RunPRGBytes(context.Background(), data); err != nil {
				return err
			}
			fmt.Println("Running!")
			return nil
		},
	}

	cmd.Flags().IntVarP(&entryNum, "entry", "e", 0, "T64 entry index to run")
	cmd.Flags().IntVarP(&songNum, "song", "s", 0, "SID sub-tune index")
	return cmd
}

func newCrtCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "crt <file.crt>",
		Short: "Run a CRT cartridge file",
		Long: `Upload and run a .CRT cartridge image.
Cartridges load instantly and bypass disk loading entirely.
Supports standard, ocean, easyflash, and other CRT formats.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			fmt.Printf("Loading CRT %s (%d bytes)...\n", filepath.Base(args[0]), len(data))
			return client.Runners.RunCRTBytes(context.Background(), data)
		},
	}
}

func newLoadCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "load <file.prg>",
		Short: "Upload PRG without running (for multi-part loaders)",
		Long: `Upload a .PRG file into C64 memory without starting it.
Use this for multi-file games where you load part 1, then part 2, then RUN.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			fmt.Printf("Loading %s (%d bytes)...\n", filepath.Base(args[0]), len(data))
			if err := client.Runners.LoadPRGBytes(context.Background(), data); err != nil {
				return err
			}
			fmt.Println("Loaded. Use 'c64ctl type RUN' to start.")
			return nil
		},
	}
}

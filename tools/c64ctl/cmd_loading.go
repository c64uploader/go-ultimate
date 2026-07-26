// Loading and execution subcommands for c64ctl.
//
// Convenience Features:
// 1. Unified Multi-Format Media Execution (c64ctl run):
//    Always resets the C64 first, then auto-detects file format by extension
//    (.prg, .crt, .d64, .d71, .d81, .g64, .t64, .sid, .mod)
//    and routes execution directly to the appropriate device runner or disk mounter.
// 2. Automated Disk Mount & Boot:
//    For disk images (.d64, .d71, .d81, .g64), automatically mounts the disk image into Drive A
//    and types LOAD "*",8,1 followed immediately by RUN into the KERNAL keyboard buffer.

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
		Short: "Upload and run a PRG, CRT, D64, T64, SID, or MOD file",
		Long: `Upload a file to C64 memory and start execution.
Supports .PRG, .CRT, .D64/.D71/.D81/.G64, .T64, .SID, and .MOD file formats.

Always resets the C64 first, then uploads the binary and jumps to the load address
(or loads the CRT cartridge).
For disk images (.D64, .D71, .D81, .G64), mounts the image and automatically types LOAD "*",8,1 and RUN.

For T64 tape archives, the first entry is run by default.
Use --entry to select a different entry.

For SID music files, use --song to select a sub-tune (default: 0).

Note: .TAP files are raw tape waveform images and cannot be run directly.
Use a .T64 file instead (a tape archive that c64ctl run can handle).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]
			ext := strings.ToLower(filepath.Ext(path))

			fmt.Println("Resetting C64...")
			if err := client.Machine.Reset(cmd.Context()); err != nil {
				return fmt.Errorf("reset: %w", err)
			}

			if ext == ".d64" || ext == ".d71" || ext == ".d81" || ext == ".g64" {
				return cmdPlay(cmd.Context(), path)
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			if ext == ".crt" {
				fmt.Printf("Loading CRT %s (%d bytes)...\n", filepath.Base(path), len(data))
				return client.Runners.RunCRTBytes(context.Background(), data)
			}

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
				fmt.Printf("T64: %s - %s (%d bytes)\n",
					filepath.Base(path), entry.Name, prg.Size())
				fmt.Printf("Running entry %d at $%04X...\n", entryNum, prg.LoadAddress())
				return client.Runners.Run(context.Background(), prg)
			}

			if ext == ".tap" {
				return fmt.Errorf(".TAP is a raw tape waveform image, not a program. Did you mean a .T64 file?")
			}

			if ext == ".sid" {
				fmt.Printf("Playing %s (song %d)...\n", filepath.Base(path), songNum)
				return client.Runners.PlaySIDBytes(context.Background(), data, songNum)
			}

			if ext == ".mod" {
				fmt.Printf("Playing %s...\n", filepath.Base(path))
				return client.Runners.PlayMODBytes(context.Background(), data)
			}

			fmt.Printf("Uploading %s (%d bytes)...\n", filepath.Base(path), len(data))
			if err := client.Runners.RunPRGBytes(context.Background(), data); err != nil {
				return err
			}
			fmt.Println("Running!")
			return nil
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return []string{"prg", "crt", "d64", "g64", "d71", "d81", "t64", "sid", "mod"}, cobra.ShellCompDirectiveFilterFileExt
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
	}

	cmd.Flags().IntVarP(&entryNum, "entry", "e", 0, "T64 entry index to run")
	cmd.Flags().IntVarP(&songNum, "song", "s", 0, "SID sub-tune index")
	return cmd
}


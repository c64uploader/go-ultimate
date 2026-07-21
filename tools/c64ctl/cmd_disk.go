package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/c64uploader/go-ultimate"
	"github.com/spf13/cobra"
)

var playWait int
var mountDriveID string
var unmountDriveID string

func newPlayCmd() *cobra.Command {
	playCmd := &cobra.Command{
		Use:   "play <file.d64>",
		Short: "Mount disk, load, and run automatically",
		Long: `Mount a D64/D71/D81/TAP and automatically type LOAD/RUN.
Waits for loading to complete, then starts the game.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdPlay(args[0], playWait)
		},
	}
	playCmd.Flags().IntVarP(&playWait, "wait", "w", 180, "Seconds to wait for loading")
	return playCmd
}

func newMountCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mount <file.d64>",
		Short: "Mount a disk image to a drive",
		Long: `Mount a D64, D71, D81, or G64 disk image to a drive.
Image type is auto-detected from extension.
After mounting, use 'c64ctl type LOAD "*",8,1' to load from the disk.

Use --drive b to mount to Drive B (device 9).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			drive := ultimate.DriveA
			if mountDriveID == "b" {
				drive = ultimate.DriveB
			}
			return mountDrive(args[0], drive)
		},
	}
	cmd.Flags().StringVarP(&mountDriveID, "drive", "d", "a", "Drive to mount: a or b")
	return cmd
}

func newUnmountCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unmount",
		Short: "Unmount a drive",
		Long: `Unmount a disk drive. Use --drive b to unmount Drive B (device 9).`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			drive := ultimate.DriveA
			if unmountDriveID == "b" {
				drive = ultimate.DriveB
			}
			return client.Drives.Unmount(context.Background(), drive)
		},
	}
	cmd.Flags().StringVarP(&unmountDriveID, "drive", "d", "a", "Drive to unmount: a or b")
	return cmd
}

func newDrivesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "drives",
		Short: "Show status of all emulated drives",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			drives, err := client.Drives.List(context.Background())
			if err != nil {
				return err
			}
			if drives.A != nil {
				img := drives.A.ImageFile
				if img == "" {
					img = "(empty)"
				}
				fmt.Printf("Drive A: %s  BusID=%d  Image=%s\n", drives.A.Type, drives.A.BusID, img)
			}
			if drives.B != nil {
				img := drives.B.ImageFile
				if img == "" {
					img = "(empty)"
				}
				fmt.Printf("Drive B: %s  BusID=%d  Image=%s\n", drives.B.Type, drives.B.BusID, img)
			}
			if drives.SoftIEC != nil {
				fmt.Printf("SoftIEC: BusID=%d  Partitions=%d\n", drives.SoftIEC.BusID, len(drives.SoftIEC.Partitions))
			}
			return nil
		},
	}
}

func newDriveResetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "drive-reset",
		Short: "Reset drive emulation",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return client.Drives.ResetDrive(context.Background(), ultimate.DriveA)
		},
	}
}

func cmdPlay(path string, waitSeconds int) error {
	ctx := context.Background()

	// Mount the disk
	if err := mountDrive(path, ultimate.DriveA); err != nil {
		return err
	}

	// Type LOAD command
	fmt.Println("Loading...")
	if err := client.Keyboard.Type(ctx, "LOAD \"*\",8,1\n"); err != nil {
		return err
	}

	// Poll screen for READY. prompt
	fmt.Printf("Waiting for load (max %d seconds)...\n", waitSeconds)
	deadline := time.Now().Add(time.Duration(waitSeconds) * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(2 * time.Second)
		screen, err := client.Debug.Screen(ctx)
		if err != nil {
			continue
		}
		for _, row := range screen.Rows {
			if strings.Contains(row, "READY.") {
				fmt.Println("Load complete!")
				fmt.Println("Starting game...")
				return client.Keyboard.Type(ctx, "RUN\n")
			}
		}
	}

	// Timeout - try RUN anyway
	fmt.Println("Timeout reached, trying RUN...")
	return client.Keyboard.Type(ctx, "RUN\n")
}

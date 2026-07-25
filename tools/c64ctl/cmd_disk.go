package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/c64uploader/go-ultimate"
	"github.com/spf13/cobra"
)

var mountDriveID string
var unmountDriveID string

func parseDriveID(s string) (ultimate.DriveID, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "a", "8", "drive a", "drivea", "0":
		return ultimate.DriveA, nil
	case "b", "9", "drive b", "driveb", "1":
		return ultimate.DriveB, nil
	default:
		return ultimate.DriveA, fmt.Errorf("invalid drive %q (use 'a' or 'b')", s)
	}
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
			drive, err := parseDriveID(mountDriveID)
			if err != nil {
				return err
			}
			return mountDrive(args[0], drive)
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return []string{"d64", "g64", "d71", "d81"}, cobra.ShellCompDirectiveFilterFileExt
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
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
			drive, err := parseDriveID(unmountDriveID)
			if err != nil {
				return err
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

func cmdPlay(ctx context.Context, path string) error {
	if ctx == nil {
		ctx = context.Background()
	}

	if err := mountDrive(path, ultimate.DriveA); err != nil {
		return err
	}

	// Type LOAD and RUN together. BASIC processes the keyboard buffer after
	// LOAD finishes, so RUN executes automatically once the disk is done.
	fmt.Println("Loading...")
	return client.Keyboard.Type(ctx, "LOAD \"*\",8,1\nRUN\n")
}

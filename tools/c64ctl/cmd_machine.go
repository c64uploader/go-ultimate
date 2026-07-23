package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newRebootCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reboot",
		Short: "Full reboot (Ultimate firmware + C64)",
		Long:  "Reboots the Ultimate firmware and then resets the C64. Cartridge settings are kept.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return client.Machine.Reboot(context.Background())
		},
	}
}

func newResetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reset",
		Short: "Hardware reset (keeps cartridge)",
		Long:  "Performs a hardware C64 reset. Cartridge settings are preserved.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return client.Machine.Reset(context.Background())
		},
	}
}

func newPauseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pause",
		Short: "Freeze the C64 CPU",
		Long:  "Pauses the C64. CPU stops but RAM is preserved. Use 'resume' to continue.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return client.Machine.Pause(context.Background())
		},
	}
}

func newResumeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resume",
		Short: "Unfreeze after pause",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return client.Machine.Resume(context.Background())
		},
	}
}

func newPeekCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "peek <address>",
		Short: "Read one byte from C64 RAM",
		Long:  "Read a single byte from C64 RAM. Address in hex (e.g., D020 for border color).",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			addr := parseHex16(args[0])
			val, err := client.Machine.Peek(context.Background(), addr)
			if err != nil {
				return err
			}
			fmt.Printf("$%04X = $%02X (%d)\n", addr, val, val)
			return nil
		},
	}
}

func newPokeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "poke <address> <value>",
		Short: "Write one byte to C64 RAM",
		Long:  "Write a single byte to C64 RAM. Both address and value in hex.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			addr := parseHex16(args[0])
			val := parseHex8(args[1])
			return client.Machine.Poke(context.Background(), addr, val)
		},
	}
}

func newOffCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "off",
		Short: "Power off the device",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return client.Machine.PowerOff(context.Background())
		},
	}
}



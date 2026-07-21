package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/c64uploader/go-ultimate"
	"github.com/c64uploader/go-ultimate/c64"
	"github.com/spf13/cobra"
)

var client *ultimate.Client
var c64Host string

// rootCmd is the root cobra command.
var rootCmd = &cobra.Command{
	Use:   "c64ctl",
	Short: "Control C64 Ultimate hardware (1541 Ultimate II+ / Ultimate 64)",
	Long: `c64ctl - C64 Ultimate Control Tool

Control a C64 Ultimate device over the network.
Load games, mount disks, play music, read screen, and more.

Examples:
  c64ctl run game.prg              Upload and run a PRG instantly
  c64ctl crt game.crt              Run a cartridge
  c64ctl mount disk.d64            Mount disk image
  c64ctl type LOAD "*",8,1         Type a BASIC command
  c64ctl screen                    Read the screen
  c64ctl find karate               Search local game collection

Cache:
  c64ctl build-cache  Indexes the local assembly64 collection
  c64ctl find ...     Instant searches via cached index

Environment:
  C64U_ADDRESS      C64 Ultimate hostname (default: c64u)
  C64U_PASSWORD     Password for the device
  ASSEMBLY64_PATH   Path to assembly64 collection (default: ~/Downloads/assembly64)`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Commands that don't need C64 connection
		if cmd.Name() == "find" || cmd.Name() == "help" || cmd.Name() == "build-cache" {
			return nil
		}
		var opts []ultimate.Option
		if pwd := os.Getenv("C64U_PASSWORD"); pwd != "" {
			opts = append(opts, ultimate.WithPassword(pwd))
		}
		var err error
		client, err = ultimate.New(c64Host, opts...)
		return err
	},
}

func registerCommands(root *cobra.Command) {
	// cmd_loading.go
	root.AddCommand(newRunCmd())
	root.AddCommand(newCrtCmd())
	root.AddCommand(newLoadCmd())

	// cmd_disk.go
	root.AddCommand(newPlayCmd())
	root.AddCommand(newMountCmd())
	root.AddCommand(newUnmountCmd())
	root.AddCommand(newDrivesCmd())
	root.AddCommand(newDriveResetCmd())

	// cmd_keyboard.go
	root.AddCommand(newTypeCmd())
	root.AddCommand(newPressCmd())

	// cmd_screen.go
	root.AddCommand(newScreenCmd())
	root.AddCommand(newScreenModeCmd())
	root.AddCommand(newBasicCmd())
	root.AddCommand(newSpritesCmd())

	// cmd_joy.go
	root.AddCommand(newJoyCmd())

	// cmd_machine.go
	root.AddCommand(newRebootCmd())
	root.AddCommand(newResetCmd())
	root.AddCommand(newPauseCmd())
	root.AddCommand(newResumeCmd())
	root.AddCommand(newPeekCmd())
	root.AddCommand(newPokeCmd())
	root.AddCommand(newOffCmd())
	root.AddCommand(newReadCmd())
	root.AddCommand(newFillCmd())
	root.AddCommand(newDisasmCmd())

	// cmd_music.go
	root.AddCommand(newModCmd())

	// cmd_record.go
	root.AddCommand(newRecordCmd())

	// cmd_find.go
	root.AddCommand(newFindCmd())
	root.AddCommand(newBuildCacheCmd())
	root.AddCommand(newStatusCmd())

	// cmd_asm.go
	root.AddCommand(newAsmCmd())
}

func main() {
	registerCommands(rootCmd)
	rootCmd.PersistentFlags().StringVarP(&c64Host, "host", "H", envOrDefault("C64U_ADDRESS", "c64u"), "C64 Ultimate hostname")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func printNotConnected() {
	fmt.Println("Not connected to C64 Ultimate")
}

// ---------------------------------------------------------------------------
//  read command — hex dump from C64 RAM
// ---------------------------------------------------------------------------

func newReadCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "read <address> <count>",
		Short: "Read hex dump from C64 RAM",
		Long: `Read bytes from C64 RAM and display as a hex dump with ASCII sidebar.
Address is hex (e.g., $0400). Count is decimal.

Examples:
  c64ctl read $0400 1000       # dump screen memory
  c64ctl read $d000 256        # dump VIC-II registers`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			addr := parseHex16(args[0])
			count, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("invalid count %q", args[1])
			}

			data, err := client.Machine.ReadMemory(context.Background(), addr, count)
			if err != nil {
				return fmt.Errorf("read memory: %w", err)
			}

			for i := 0; i < len(data); i += 16 {
				lineAddr := addr + uint16(i)
				fmt.Printf("  $%04X  ", lineAddr)

				hexPart := ""
				ascPart := ""
				for j := 0; j < 16; j++ {
					if i+j < len(data) {
						hexPart += fmt.Sprintf("%02X ", data[i+j])
						b := data[i+j]
						if b >= 32 && b < 127 {
							ascPart += string(b)
						} else {
							ascPart += "."
						}
					} else {
						hexPart += "   "
					}
					if j == 7 {
						hexPart += " "
					}
				}
				fmt.Printf("%s |%s|\n", hexPart, ascPart)
			}
			fmt.Printf("  (%d bytes from $%04X)\n", len(data), addr)
			return nil
		},
	}
}

// ---------------------------------------------------------------------------
//  fill command — fill a range of C64 RAM with a byte value
// ---------------------------------------------------------------------------

func newFillCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "fill <address> <count> <value>",
		Short: "Fill a range of C64 RAM with a byte value",
		Long: `Write a repeating byte value into C64 RAM.
Address and value are hex. Count is decimal.

Examples:
  c64ctl fill $2000 4096 0      # zero out 4K at $2000
  c64ctl fill $1000 256 $ea     # fill with NOPs`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			addr := parseHex16(args[0])
			count, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("invalid count %q", args[1])
			}
			val := parseHex8(args[2])

			data := make([]byte, count)
			for i := range data {
				data[i] = val
			}

			if err := client.Machine.WriteMemory(context.Background(), addr, data); err != nil {
				return fmt.Errorf("fill failed: %w", err)
			}
			fmt.Printf("✓ Filled %d bytes at $%04X with $%02X\n", count, addr, val)
			return nil
		},
	}
}

// ---------------------------------------------------------------------------
//  disasm command — read RAM and disassemble 6502
// ---------------------------------------------------------------------------

func newDisasmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disasm <address> [<count>]",
		Short: "Disassemble 6502 code from C64 RAM",
		Long: `Read bytes from C64 RAM and disassemble them as 6502 instructions.
Address is hex (e.g., $1000 or 1000). Count is decimal (default: 64).

Examples:
  c64ctl disasm $c000          # disassemble 64 bytes from $C000
  c64ctl disasm $1000 128      # disassemble 128 bytes from $1000`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			addr := parseHex16(args[0])
			count := 64
			if len(args) > 1 {
				var err error
				count, err = strconv.Atoi(args[1])
				if err != nil {
					return fmt.Errorf("invalid count %q", args[1])
				}
			}

			data, err := client.Machine.ReadMemory(context.Background(), addr, count)
			if err != nil {
				return fmt.Errorf("read memory: %w", err)
			}

			for _, inst := range c64.Disassemble(data, addr) {
				hexStr := ""
				for _, b := range inst.Bytes {
					hexStr += fmt.Sprintf("%02X ", b)
				}
				fmt.Printf("  $%04X  %-10s %s\n", inst.Address, hexStr, inst.Code)
			}
			return nil
		},
	}
}



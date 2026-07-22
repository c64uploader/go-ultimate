package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newJoyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "joy [normal|swap|wasd1|wasd2]",
		Short: "Control joystick port swapping and WASD emulation",
		Long: `Swap joystick ports 1 and 2, or enable keyboard-based joystick emulation.

  normal    — Normal joystick assignment (default)
  swap      — Swap joystick ports 1 ↔ 2
  wasd1     — WASD keys emulate joystick on port 1 (Up/Down/Left/Right + Space=Fire)
  wasd2     — WASD keys emulate joystick on port 2

Without arguments, shows the current joystick mode.

Use c64ctl press to send keys remotely (best-effort; requires KERNAL IRQ or standard CIA reading):
  c64ctl press W A S D Space   — simulate joystick movement + fire`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := client.Configs
			cat := "U64 Specific Settings"
			item := "Joystick Swapper"

			if len(args) == 0 {
				// Show current setting
				items, err := cfg.GetItem(context.Background(), cat, item)
				if err != nil {
					return fmt.Errorf("reading joystick config: %w", err)
				}
				ci, ok := items.Get(cat, item)
				if !ok {
					return fmt.Errorf("joystick setting not found")
				}
				fmt.Printf("Joystick mode: %v\n", ci.Current)
				if len(ci.Values) > 0 {
					fmt.Println("Available modes:")
					for _, v := range ci.Values {
						fmt.Printf("  %s\n", v)
					}
				}
				return nil
			}

			// Map short names to full mode names
			modeMap := map[string]string{
				"normal": "Normal",
				"swap":   "Swapped",
				"wasd1":  "WASD Port 1",
				"wasd2":  "WASD Port 2",
			}

			mode, ok := modeMap[args[0]]
			if !ok {
				return fmt.Errorf("unknown mode %q — use: normal, swap, wasd1, wasd2", args[0])
			}

			if err := cfg.Set(context.Background(), cat, item, mode); err != nil {
				return fmt.Errorf("setting joystick mode: %w", err)
			}
			fmt.Printf("Joystick mode set to %q\n", mode)
			return nil
		},
	}
}

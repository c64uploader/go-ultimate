package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/c64uploader/go-ultimate/c64"
	"github.com/spf13/cobra"
)

func newTypeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "type <text>",
		Short: "Type text on the C64 keyboard",
		Long: `Type ASCII text into the C64 KERNAL keyboard buffer.
A newline is automatically appended (acts as pressing RETURN).
Use this for BASIC commands like LOAD, RUN, SYS, POKE, etc.

Note: Does not work in software that reads hardware registers directly.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			text := strings.Join(args, " ")
			fmt.Printf("Typing: %s\n", text)
			return client.Keyboard.Type(context.Background(), text+"\n")
		},
	}
}

func newPressCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "press <key> [key...]",
		Short: "Simulate pressing key(s) via KERNAL hooks and CIA matrix",
		Long: `Simulate pressing one or more keys simultaneously using KERNAL hooks and CIA register overrides.

WARNING: Press is a best-effort feature and WILL FAIL in many games, demos, or
programs that use custom IRQ handlers, disable interrupts, or scan CIA hardware directly.

Available keys: SPACE, RETURN, RUN/STOP, F1, F3, F5, F7,
  LEFT, DOWN, UP, DELETE, HOME, SHIFT, COMMODORE, CTRL,
  A-Z (letters), 0-9 (digits)`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var keys []c64.Key
			for _, name := range args {
				k, ok := parseKey(name)
				if !ok {
					return fmt.Errorf("unknown key: %s", name)
				}
				keys = append(keys, k)
			}
			return client.Keyboard.Press(context.Background(), keys...)
		},
	}
}

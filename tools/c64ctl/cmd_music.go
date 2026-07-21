package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newModCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mod <file.mod>",
		Short: "Play a MOD music file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			fmt.Printf("Playing %s...\n", filepath.Base(args[0]))
			return client.Runners.PlayMODBytes(context.Background(), data)
		},
	}
}

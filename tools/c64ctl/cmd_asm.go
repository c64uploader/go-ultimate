package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/c64uploader/go-ultimate/c64"
	"github.com/spf13/cobra"
)

func newAsmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "asm [<file>]",
		Short: "Assemble 6502 source and inject into C64 RAM",
		Long: `Read 6502 assembly source from a file or stdin, assemble it,
and inject the resulting machine code into C64 RAM via DMA.

Prints the SYS command to run the code from BASIC.

With a file argument, reads from that file. Without one, reads from stdin.

Examples:
  c64ctl asm mycode.asm
  echo 'lda #$05 sta $d020 rts' | c64ctl asm

The assembler supports labels, .byte/.word/.text, BASICHeader,
and standard 6502 addressing modes. Use * = $addr to set origin
(defaults to $0801 — BASIC area).`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var source []byte
			var err error

			if len(args) > 0 {
				source, err = os.ReadFile(args[0])
				if err != nil {
					return fmt.Errorf("reading file: %w", err)
				}
			} else {
				source, err = io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("reading stdin: %w", err)
				}
			}

			prog, err := c64.Assemble(string(source))
			if err != nil {
				return fmt.Errorf("assembly error: %w", err)
			}

			if err := client.Machine.Inject(context.Background(), prog); err != nil {
				return fmt.Errorf("inject failed: %w", err)
			}

			addr := prog.LoadAddress()
			size := prog.Size()

			fmt.Printf("✓ Assembled and injected %d bytes at $%04X\n\n", size, addr)

			for _, inst := range prog.Disassemble() {
				hexStr := ""
				for _, b := range inst.Bytes {
					hexStr += fmt.Sprintf("%02X ", b)
				}
				fmt.Printf("  $%04X  %-10s %s\n", inst.Address, hexStr, inst.Code)
			}
			fmt.Printf("\n  SYS %d\n", addr)
			return nil
		},
	}
}

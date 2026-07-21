package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/c64uploader/go-ultimate/c64/codec"
	"github.com/spf13/cobra"
)

func newScreenCmd() *cobra.Command {
	var petscii bool

	cmd := &cobra.Command{
		Use:   "screen",
		Short: "Read the current 25x40 screen text",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			screen, err := client.Debug.Screen(ctx)
			if err != nil {
				return err
			}

			if petscii {
				// Show each row as a PETSCII hex dump
				for r := range 25 {
					offset := r * 40
					raw := screen.RawScreen[offset : offset+40]

					// Convert screen codes -> PETSCII
					petsciiBytes := make([]byte, 40)
					for i, sc := range raw {
						petsciiBytes[i] = codec.ScreenPET.DecodeByte(sc)
					}

					// Hex part
					var hexParts []string
					for i := 0; i < 40; i++ {
						hexParts = append(hexParts, fmt.Sprintf("%02X", petsciiBytes[i]))
						if i == 19 {
							hexParts = append(hexParts, " ")
						}
					}

					// ASCII sidebar via DecodePETSCII
					var asc strings.Builder
					for _, pb := range petsciiBytes {
						ch := codec.PETSCII.DecodeByte(pb)
						if ch >= 32 && ch < 127 {
							asc.WriteByte(ch)
						} else {
							asc.WriteByte('.')
						}
					}

					fmt.Printf("  row%2d  %s |%s|\n", r, strings.Join(hexParts, " "), asc.String())
				}
				return nil
			}

			// Default: plain text rows
			for _, row := range screen.Rows {
				fmt.Println(row)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&petscii, "petscii", false, "Show screen as PETSCII hex dump")
	return cmd
}

func newBasicCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "basic",
		Short: "Read tokenized BASIC program from RAM",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			lines, err := client.Debug.BASIC(context.Background())
			if err != nil {
				return err
			}
			for _, line := range lines {
				fmt.Printf("%d %s\n", line.Number, line.Content)
			}
			return nil
		},
	}
}

func newScreenModeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "screenmode",
		Short: "Show VIC-II display mode and character set",
		Long: `Read VIC-II registers and report the current display mode
and active character set.

Displays the mode name, the VIC register values that determine
it, and whether the lowercase/uppercase or uppercase/graphics
character set is active.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			mode, err := client.Debug.ScreenMode(ctx)
			if err != nil {
				return err
			}

			// Read VIC-II registers $D011–$D018 for display
			regs, err := client.Machine.ReadMemory(ctx, 0xD011, 8)
			if err != nil {
				return err
			}
			d011 := regs[0]
			d016 := regs[5]
			d018 := regs[7]

			// $D018 bit 1 (value $02) selects the character generator:
			//   0 = uppercase/graphics set  (character base $1000)
			//   1 = lowercase/uppercase set (character base $1800)
			cbBit := (d018 >> 1) & 1
			charsetName := "uppercase/graphics"
			if cbBit == 1 {
				charsetName = "lowercase/uppercase"
			}

			fmt.Printf("Mode:         %s\n", mode)
			fmt.Printf("Char set:     %s ($D018 bit1=%d)\n", charsetName, cbBit)
			fmt.Printf("$D011:        $%02X  (bit5=BMM=%d  bit6=ECM=%d)\n", d011, (d011>>5)&1, (d011>>6)&1)
			fmt.Printf("$D016:        $%02X  (bit4=MCM=%d)\n", d016, (d016>>4)&1)
			fmt.Printf("$D018:        $%02X  (bit1=CB=%d  nibble4-7=screen=$%X000)\n", d018, cbBit, d018>>4)
			return nil
		},
	}
}

func newSpritesCmd() *cobra.Command {

	return &cobra.Command{
		Use:   "sprites",
		Short: "Show all 8 hardware sprites",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			sprites, err := client.Debug.Sprites(context.Background())
			if err != nil {
				return err
			}
			for _, s := range sprites {
				if s.Enabled {
					fmt.Printf("Sprite %d: X=%3d Y=%3d Color=%d MC=%v XExp=%v YExp=%v\n",
						s.Number, s.X, s.Y, s.Color, s.Multicolor, s.XExpand, s.YExpand)
				}
			}
			return nil
		},
	}
}

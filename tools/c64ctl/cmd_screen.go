package main

import (
	"context"
	"fmt"
	"image/png"
	"os"
	"path/filepath"

	"github.com/c64uploader/go-ultimate/c64"
	"github.com/spf13/cobra"
)

func newScreenCmd() *cobra.Command {
	var hex bool

	cmd := &cobra.Command{
		Use:   "screen",
		Short: "Read the current 25×40 screen text",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			screen, err := client.Debug.Screen(ctx)
			if err != nil {
				return err
			}

			if hex {
				fmt.Print(screen.HexDump(c64.HexScreen | c64.HexPETSCII | c64.HexColor))
				return nil
			}

			fmt.Print(screen.Dump())
			return nil
		},
	}

	cmd.Flags().BoolVar(&hex, "hex", false, "Show screen-codes: and petscii: hex dumps for non-empty rows")
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
	var outDir string

	cmd := &cobra.Command{
		Use:   "sprites",
		Short: "Show all 8 hardware sprites or dump active sprites to PNG",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			sprites, err := client.Debug.Sprites(context.Background())
			if err != nil {
				return err
			}

			if outDir != "" {
				if err := os.MkdirAll(outDir, 0755); err != nil {
					return fmt.Errorf("create directory %s: %w", outDir, err)
				}
				saved := 0
				for _, s := range sprites {
					if s.Enabled {
						img, err := s.Image()
						if err != nil {
							return fmt.Errorf("render sprite %d: %w", s.Number, err)
						}
						filename := filepath.Join(outDir, fmt.Sprintf("sprite_%d.png", s.Number))
						f, err := os.Create(filename)
						if err != nil {
							return fmt.Errorf("create %s: %w", filename, err)
						}
						if err := png.Encode(f, img); err != nil {
							_ = f.Close()
							return fmt.Errorf("encode %s: %w", filename, err)
						}
						_ = f.Close()
						fmt.Printf("✓ Saved sprite %d to %s\n", s.Number, filename)
						saved++
					}
				}
				if saved == 0 {
					fmt.Println("No active sprites to dump.")
				}
				return nil
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

	cmd.Flags().StringVarP(&outDir, "out", "o", "", "Directory to save active sprite PNG images")
	return cmd
}

func newScreenshotCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "screenshot <file.png>",
		Short: "Grab a bitmap-mode screenshot and save as PNG",
		Long: `Capture the current VIC-II bitmap display (standard or multicolor) and save
it as a 320×200 PNG image. Requires the C64 to be in bitmap mode.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			outFile := args[0]

			img, err := client.Debug.Bitmap(ctx)
			if err != nil {
				return fmt.Errorf("screenshot: %w", err)
			}

			f, err := os.Create(outFile)
			if err != nil {
				return fmt.Errorf("create %s: %w", outFile, err)
			}
			defer func() { _ = f.Close() }()

			if err := png.Encode(f, img); err != nil {
				return fmt.Errorf("encode PNG: %w", err)
			}

			fmt.Printf("✓ Saved screenshot to %s\n", outFile)
			return nil
		},
	}

	return cmd
}

//go:build ignore

// Run: go run examples/debug.go
package main

import (
	"context"
	"fmt"
	"image/png"
	"os"
	"time"

	"github.com/c64uploader/go-ultimate"
	"github.com/c64uploader/go-ultimate/c64"
)

func main() {
	client, _ := ultimate.New("c64u")
	ctx := context.Background()

	_ = client.Machine.Reboot(ctx)
	time.Sleep(3 * time.Second)

	// Read the 25*40 screen from the C64 and print each row.
	screen, _ := client.Debug.Screen(ctx)
	for i, row := range screen.Rows {
		fmt.Printf("%2d: %s\n", i, row)
	}

	// Screen mode
	mode, _ := client.Debug.ScreenMode(ctx)
	fmt.Println("screen mode:", mode)

	// Sprites
	// Read all eight VIC-II hardware sprites and print active ones.
	sprites, _ := client.Debug.Sprites(ctx)
	for _, s := range sprites {
		if s.Enabled {
			fmt.Printf("sprite %d @ (%d,%d) color=%s\n",
				s.Number, s.X, s.Y, c64.ColorName(s.Color))
		}
	}

	// Render a sprite to a PNG image:
	// img, _ := sprites[0].Image()

	// Screen code encoding / decoding

	fmt.Println(c64.EncodeScreen("HELLO"))
	fmt.Println(c64.DecodeScreen([]byte{8, 5, 12, 12, 15})) // H E L L O

	// Bitmap rendering
	// Write bitmap data to C64 RAM, switch to multicolor bitmap mode.

	bitmap, _ := os.ReadFile("tests/e2e/testdata/golang_bitmap.bin")
	screenColors, _ := os.ReadFile("tests/e2e/testdata/golang_screen.bin")

	_ = client.Machine.Reboot(ctx)

	_ = client.Machine.WriteMemory(ctx, 0x2000, bitmap)
	_ = client.Machine.WriteMemory(ctx, 0x0400, screenColors)

	// Configure VIC: bitmap at $2000, screen at $0400, hires bitmap mode.
	_ = client.Machine.Poke(ctx, 0xD018, 0x18)
	_ = client.Machine.Poke(ctx, 0xD011, 0x3B)

	// Freeze CPU so BASIC doesn't reclaim the screen, then read the bitmap and write it to a PNG file.
	_ = client.Machine.Pause(ctx)
	img, _ := client.Debug.Bitmap(ctx)
	fmt.Println("Writing bitmap.png...")
	f, _ := os.Create("bitmap.png")
	defer f.Close()
	png.Encode(f, img)

	// VIC-II colors
	fmt.Println(c64.ColorName(5))              // "GREEN"
	fmt.Println(c64.Palette()[c64.ColorGreen]) // RGBA value

	// BASIC listing
	// Inject a BASIC program into RAM and read it back detokenized.
	prog := c64.NewProgram([]byte{
		0x0b, 0x08, // next line pointer
		0x0a, 0x00, // line number 10
		0x99, 0x20, // PRINT token + space
		0x22, 0x48, 0x49, 0x22, // "HI"
		0x00,       // end of line
		0x00, 0x00, // end of program
	}, 0x0801)
	_ = client.Machine.Inject(ctx, prog)

	lines, _ := client.Debug.BASIC(ctx)
	for _, line := range lines {
		fmt.Println(line.Number, line.Content)
	}

	// Character set
	// Read and render the active character generator.
	charset, _ := client.Debug.Charset(ctx)
	charImage, _ := charset.CharacterImage('A', c64.ColorWhite, c64.ColorBlack)
	fmt.Printf("character A image: %dx%d\n", charImage.Bounds().Dx(), charImage.Bounds().Dy())
}

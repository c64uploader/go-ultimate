//go:build ignore

// Run: go run examples/configs.go
package main

import (
	"context"
	"fmt"

	"github.com/c64uploader/go-ultimate"
)

func main() {
	client, _ := ultimate.New("c64u")
	ctx := context.Background()

	// 1. Read REU settings
	reuCfg, _ := client.Configs.Get(ctx, "C64 and Cartridge Settings")
	reuEnabled, _ := reuCfg.Bool("C64 and Cartridge Settings", "RAM Expansion Unit")
	reuSize, _ := reuCfg.String("C64 and Cartridge Settings", "REU Size")
	fmt.Printf("REU Enabled: %v, Size: %s\n", reuEnabled, reuSize)

	// 2. Read Accelerator settings
	speedCfg, _ := client.Configs.Get(ctx, "U64 Specific Settings")
	cpuSpeed, _ := speedCfg.Int("U64 Specific Settings", "CPU Speed")
	turboMode, _ := speedCfg.String("U64 Specific Settings", "Turbo Control")
	fmt.Printf("CPU Speed: %dx, Turbo Control: %s\n", cpuSpeed, turboMode)

	// 3. Update REU and Accelerator settings
	_ = client.Configs.SetBool(ctx, "C64 and Cartridge Settings", "RAM Expansion Unit", true)
	_ = client.Configs.Set(ctx, "C64 and Cartridge Settings", "REU Size", "16 MB")
	_ = client.Configs.SetInt(ctx, "U64 Specific Settings", "CPU Speed", 4)

	// 4. Inspect REU Size metadata (allowed sizes)
	meta, _ := client.Configs.GetItem(ctx, "C64 and Cartridge Settings", "REU Size")
	info, _ := meta.Get("C64 and Cartridge Settings", "REU Size")
	fmt.Println("Allowed REU Sizes:", info.Values)

	// 5. Persist changes to flash storage
	// _ = client.Configs.SaveToFlash(ctx, ultimate.ConfigOptions{})
}

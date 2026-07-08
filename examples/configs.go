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

	// List category names.
	categories, _ := client.Configs.List(ctx)
	fmt.Println("categories:", categories)

	// Pick a category and read all its current values.
	category := categories[0]
	settings, _ := client.Configs.Get(ctx, category)
	for name, value := range settings[category] {
		fmt.Println(name, value)
	}

	// Get metadata for one item: current value, allowed values, min/max, etc.
	// Pick an item name from the output above.
	item := "Vol Sampler L"
	meta, _ := client.Configs.GetItem(ctx, category, item)
	info := meta[category][item]
	fmt.Println(item, "current:", info.Current, "allowed:", info.Values)

	// Change the setting and restore the original.
	original := fmt.Sprint(info.Current)
	_ = client.Configs.Set(ctx, category, item, info.Values[0])
	_ = client.Configs.Set(ctx, category, item, original)

	// Save to flash to persist after reboot:
	// _ = client.Configs.SaveToFlash(ctx, ultimate.ConfigOptions{})
}

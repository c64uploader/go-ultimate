package e2e

import (
	"context"
	"testing"

	"github.com/c64uploader/go-ultimate"
)

// Safe setting to mutate and restore in write tests.
const (
	e2eConfigCat  = "Drive A Settings"
	e2eConfigItem = "Drive Bus ID"
)

func TestE2E_ConfigsList(t *testing.T) {
	client, ctx := setupE2E(t)

	categories, err := client.Configs.List(ctx)
	if err != nil {
		t.Fatalf("Configs.List failed: %v", err)
	}
	t.Logf("Config categories: %v", categories)
	if len(categories) == 0 {
		t.Error("Expected at least one config category")
	}
}

func TestE2E_ConfigsGet(t *testing.T) {
	client, ctx := setupE2E(t)

	cfg, err := client.Configs.Get(ctx, e2eConfigCat)
	if err != nil {
		t.Fatalf("Configs.Get failed: %v", err)
	}

	items := cfg[e2eConfigCat]
	if len(items) == 0 {
		t.Fatal("Expected items in category")
	}

	// Verify typed accessors work on real data
	s, ok := cfg.String(e2eConfigCat, "Drive")
	if !ok || (s != "Enabled" && s != "Disabled") {
		t.Errorf("String(Drive) = %q, %v; want Enabled/Disabled", s, ok)
	}

	i, ok := cfg.Int(e2eConfigCat, e2eConfigItem)
	if !ok || i < 8 || i > 15 {
		t.Errorf("Int(Drive Bus ID) = %d, %v; want 8-15", i, ok)
	}

	b, ok := cfg.Bool(e2eConfigCat, "Drive")
	if !ok {
		t.Errorf("Bool(Drive) not found")
	} else {
		t.Logf("Drive = %v (Bool)", b)
	}
}

func TestE2E_ConfigsGetItem(t *testing.T) {
	client, ctx := setupE2E(t)

	meta, err := client.Configs.GetItem(ctx, e2eConfigCat, e2eConfigItem)
	if err != nil {
		t.Fatalf("Configs.GetItem failed: %v", err)
	}

	ci, ok := meta.Get(e2eConfigCat, e2eConfigItem)
	if !ok {
		t.Fatal("GetItem returned no item")
	}
	if ci.Current == nil {
		t.Error("Expected non-nil current value")
	}
	t.Logf("%s = %v (min=%d max=%d)", e2eConfigItem, ci.Current, ci.Min, ci.Max)
}

func TestE2E_ConfigsSet(t *testing.T) {
	client, ctx := setupE2E(t)

	// Read original value
	orig := readE2EInt(t, client, ctx, e2eConfigCat, e2eConfigItem)
	t.Logf("original %s = %d", e2eConfigItem, orig)

	// Change it
	newVal := orig + 1
	if newVal > 15 {
		newVal = 8
	}
	if err := client.Configs.SetInt(ctx, e2eConfigCat, e2eConfigItem, newVal); err != nil {
		t.Fatalf("SetInt failed: %v", err)
	}

	current := readE2EInt(t, client, ctx, e2eConfigCat, e2eConfigItem)
	if current != newVal {
		t.Errorf("after SetInt: got %d, want %d", current, newVal)
	}

	// Restore
	if err := client.Configs.SetInt(ctx, e2eConfigCat, e2eConfigItem, orig); err != nil {
		t.Fatalf("restore SetInt failed: %v", err)
	}
}

func TestE2E_ConfigsSetBool(t *testing.T) {
	client, ctx := setupE2E(t)

	cfg, _ := client.Configs.Get(ctx, e2eConfigCat)
	wasEnabled, _ := cfg.Bool(e2eConfigCat, "Drive")

	// Toggle
	if err := client.Configs.SetBool(ctx, e2eConfigCat, "Drive", !wasEnabled); err != nil {
		t.Fatalf("SetBool failed: %v", err)
	}

	current, _ := client.Configs.Bool(ctx, e2eConfigCat, "Drive")
	if current != !wasEnabled {
		t.Errorf("after SetBool: got %v, want %v", current, !wasEnabled)
	}

	// Restore
	if err := client.Configs.SetBool(ctx, e2eConfigCat, "Drive", wasEnabled); err != nil {
		t.Fatalf("restore SetBool failed: %v", err)
	}
}

func TestE2E_ConfigsApply(t *testing.T) {
	client, ctx := setupE2E(t)

	orig := readE2EInt(t, client, ctx, e2eConfigCat, e2eConfigItem)

	newVal := orig + 1
	if newVal > 15 {
		newVal = 8
	}

	err := client.Configs.Apply(ctx, ultimate.ConfigMap{
		e2eConfigCat: {e2eConfigItem: float64(newVal)},
	})
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	current := readE2EInt(t, client, ctx, e2eConfigCat, e2eConfigItem)
	if current != newVal {
		t.Errorf("after Apply: got %d, want %d", current, newVal)
	}

	// Restore
	_ = client.Configs.SetInt(ctx, e2eConfigCat, e2eConfigItem, orig)
}

func readE2EInt(t *testing.T, client *ultimate.Client, ctx context.Context, cat, item string) int {
	t.Helper()
	cfg, err := client.Configs.Get(ctx, cat)
	if err != nil {
		t.Fatalf("Get %q failed: %v", cat, err)
	}
	v, ok := cfg.Int(cat, item)
	if !ok {
		t.Fatalf("%q / %q not found or not int", cat, item)
	}
	return v
}

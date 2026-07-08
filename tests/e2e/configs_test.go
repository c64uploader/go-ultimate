package e2e

import (
	"testing"
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

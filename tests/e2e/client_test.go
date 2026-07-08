package e2e

import (
	"testing"
)

func TestE2E_VersionAndInfo(t *testing.T) {
	client, ctx := setupE2E(t)

	v, err := client.Version(ctx)
	if err != nil {
		t.Fatalf("Version failed: %v", err)
	}
	t.Logf("Firmware API version: %s", v.Version)

	info, err := client.Info(ctx)
	if err != nil {
		t.Fatalf("Info failed: %v", err)
	}
	t.Logf("Product: %s, Firmware: %s, FPGA: %s", info.Product, info.FirmwareVersion, info.FPGAVersion)
	if info.Product == "" {
		t.Error("Product name is empty")
	}
}

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigLoadingAndPrecedence(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Create a config file
	configContent := `{
		"host": "confighost",
		"user": "configuser",
		"password": "configpass",
		"assembly64_path": "/config/path",
		"cache_dir": "` + filepath.ToSlash(filepath.Join(tempDir, "config_cache")) + `"
	}`
	cfgPath := filepath.Join(tempDir, "config.json")
	if err := os.WriteFile(cfgPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config file: %v", err)
	}

	// 2. Load config file
	cfg, loadedPath, err := loadConfigFile(cfgPath)
	if err != nil {
		t.Fatalf("loadConfigFile failed: %v", err)
	}
	if loadedPath != cfgPath {
		t.Errorf("expected loadedPath %q, got %q", cfgPath, loadedPath)
	}
	if cfg.Host != "confighost" || cfg.User != "configuser" || cfg.Password != "configpass" {
		t.Errorf("unexpected config values: %+v", cfg)
	}

	// 3. Test precedence: Config value used when env and CLI flag not set
	resHost := resolveSetting(nil, "host", "C64U_ADDRESS_TEST_UNSET", cfg.Host, "defaulthost")
	if resHost != "confighost" {
		t.Errorf("expected %q, got %q", "confighost", resHost)
	}

	// 4. Test precedence: Environment variable overrides config value
	t.Setenv("C64U_ADDRESS_TEST", "envhost")
	resHostEnv := resolveSetting(nil, "host", "C64U_ADDRESS_TEST", cfg.Host, "defaulthost")
	if resHostEnv != "envhost" {
		t.Errorf("expected %q, got %q", "envhost", resHostEnv)
	}
}

func TestFormatBytes(t *testing.T) {
	if got := formatBytes(500); got != "500 B" {
		t.Errorf("formatBytes(500) = %q; want 500 B", got)
	}
	if got := formatBytes(2048); got != "2.0 KB (2048 bytes)" {
		t.Errorf("formatBytes(2048) = %q; want 2.0 KB (2048 bytes)", got)
	}
}

func TestStatusCmd(t *testing.T) {
	cmd := newStatusCmd()
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("status command failed: %v", err)
	}
}

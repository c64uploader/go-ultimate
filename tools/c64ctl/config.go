package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Config holds settings for c64ctl.
type Config struct {
	Host           string `json:"host,omitempty"`
	User           string `json:"user,omitempty"`
	Password       string `json:"password,omitempty"`
	Assembly64Path string `json:"assembly64_path,omitempty"`
	CacheDir       string `json:"cache_dir,omitempty"`
}

func defaultConfigDir() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = os.TempDir()
	}
	return filepath.Join(configDir, "c64ctl")
}

func defaultCacheDir() string {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	return filepath.Join(cacheDir, "c64ctl")
}

// loadConfigFile loads configuration from customPath or default search locations.
// Default location if customPath is empty:
// <UserConfigDir>/c64ctl/config.json
func loadConfigFile(customPath string) (*Config, string, error) {
	var targetPath string

	if customPath != "" {
		targetPath = customPath
	} else {
		targetPath = filepath.Join(defaultConfigDir(), "config.json")
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && customPath == "" {
			return nil, "", nil // No config file found in default locations
		}
		return nil, targetPath, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, targetPath, err
	}

	return &cfg, targetPath, nil
}

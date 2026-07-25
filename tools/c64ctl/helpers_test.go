package main

import (
	"testing"

	"github.com/c64uploader/go-ultimate"
)

func TestParseKey(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"a", true},
		{"A", true},
		{"z", true},
		{"Z", true},
		{"0", true},
		{"9", true},
		{"SPACE", true},
		{"space", true},
		{"RETURN", true},
		{"return", true},
		{"INVALID_KEY", false},
	}

	for _, tt := range tests {
		_, ok := parseKey(tt.input)
		if ok != tt.want {
			t.Errorf("parseKey(%q) = %v; want %v", tt.input, ok, tt.want)
		}
	}
}

func TestParseHex(t *testing.T) {
	// Test parseHex16
	v16, err := parseHex16("$0400")
	if err != nil || v16 != 0x0400 {
		t.Errorf("parseHex16($0400) = %X, %v; want 0400, nil", v16, err)
	}

	v16, err = parseHex16("0x0400")
	if err != nil || v16 != 0x0400 {
		t.Errorf("parseHex16(0x0400) = %X, %v; want 0400, nil", v16, err)
	}

	_, err = parseHex16("$G000")
	if err == nil {
		t.Errorf("parseHex16($G000) expected error, got nil")
	}

	// Test parseHex8
	v8, err := parseHex8("$EA")
	if err != nil || v8 != 0xEA {
		t.Errorf("parseHex8($EA) = %X, %v; want EA, nil", v8, err)
	}

	_, err = parseHex8("$100") // overflow byte
	if err == nil {
		t.Errorf("parseHex8($100) expected overflow error, got nil")
	}
}

func TestParseDriveID(t *testing.T) {
	d, err := parseDriveID("a")
	if err != nil || d != ultimate.DriveA {
		t.Errorf("parseDriveID('a') = %v, %v; want DriveA", d, err)
	}

	d, err = parseDriveID("B")
	if err != nil || d != ultimate.DriveB {
		t.Errorf("parseDriveID('B') = %v, %v; want DriveB", d, err)
	}

	d, err = parseDriveID("9")
	if err != nil || d != ultimate.DriveB {
		t.Errorf("parseDriveID('9') = %v, %v; want DriveB", d, err)
	}

	_, err = parseDriveID("invalid")
	if err == nil {
		t.Errorf("parseDriveID('invalid') expected error, got nil")
	}
}

func TestParseFTPEntry_DOS_Spaces(t *testing.T) {
	line := "01-01-26 12:00PM <DIR> My Cool Games"
	entry := parseFTPEntry(line)
	if !entry.IsDir {
		t.Errorf("expected IsDir to be true")
	}
	if entry.Name != "My Cool Games" {
		t.Errorf("expected Name to be 'My Cool Games', got %q", entry.Name)
	}
}

func TestGlobalFlagsAndEnvVars(t *testing.T) {
	t.Setenv("C64U_ASSEMBLY64_PATH", "/tmp/test_assembly64")
	resolvedPath := resolveSetting(nil, "path", "C64U_ASSEMBLY64_PATH", "", assembly64Root())
	if resolvedPath != "/tmp/test_assembly64" {
		t.Errorf("expected resolved path /tmp/test_assembly64 from C64U_ASSEMBLY64_PATH, got %q", resolvedPath)
	}

	userFlag := rootCmd.PersistentFlags().Lookup("user")
	if userFlag == nil {
		t.Fatalf("expected persistent --user flag")
	}
	if userFlag.Shorthand != "u" {
		t.Errorf("expected persistent --user shorthand 'u', got %q", userFlag.Shorthand)
	}

	passFlag := rootCmd.PersistentFlags().Lookup("password")
	if passFlag == nil {
		t.Fatalf("expected persistent --password flag")
	}
	if passFlag.Shorthand != "P" {
		t.Errorf("expected persistent --password shorthand 'P', got %q", passFlag.Shorthand)
	}

	configFlag := rootCmd.PersistentFlags().Lookup("config")
	if configFlag == nil {
		t.Fatalf("expected persistent --config flag")
	}
	if configFlag.Shorthand != "c" {
		t.Errorf("expected persistent --config shorthand 'c', got %q", configFlag.Shorthand)
	}

	cacheFlag := rootCmd.PersistentFlags().Lookup("cache-dir")
	if cacheFlag == nil {
		t.Fatalf("expected persistent --cache-dir flag")
	}
}

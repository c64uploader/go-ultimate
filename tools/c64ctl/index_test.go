package main

import (
	"path/filepath"
	"testing"
)

func TestIndex_BuildAndFind(t *testing.T) {
	root := "/test/root"
	files := []string{
		filepath.Join(root, "Games/Action/Stix.crt"),
		filepath.Join(root, "Games/Action/Karate.d64"),
		filepath.Join(root, "Music/Pop/Song.sid"),
		filepath.Join(root, "Demos/Scene/Demo.prg"),
	}

	data, err := BuildIndexBytes(files, root)
	if err != nil {
		t.Fatalf("BuildIndexBytes failed: %v", err)
	}

	idx, err := OpenIndex(data, root)
	if err != nil {
		t.Fatalf("OpenIndex failed: %v", err)
	}

	// Test 1: Plain query
	res, err := idx.Find(FindOptions{Query: "stix"})
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	if len(res.Matches) != 1 || res.Matches[0] != files[0] {
		t.Errorf("expected 1 match for 'stix', got %v", res.Matches)
	}

	// Test 2: Filter by type
	res, err = idx.Find(FindOptions{FilterType: "d64"})
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	if len(res.Matches) != 1 || res.Matches[0] != files[1] {
		t.Errorf("expected 1 match for d64, got %v", res.Matches)
	}

	// Test 3: Filter by folder
	res, err = idx.Find(FindOptions{FilterFolder: "Music"})
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	if len(res.Matches) != 1 || res.Matches[0] != files[2] {
		t.Errorf("expected 1 match for Music folder, got %v", res.Matches)
	}

	// Test 4: Regex query
	res, err = idx.Find(FindOptions{Query: "stix|karate"})
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	if len(res.Matches) != 2 {
		t.Errorf("expected 2 matches for regex OR, got %d", len(res.Matches))
	}

	// Test 5: Optional quantifier regex (e.g. stix?)
	res, err = idx.Find(FindOptions{Query: "stix?"})
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	if len(res.Matches) != 1 || res.Matches[0] != files[0] {
		t.Errorf("expected 1 match for 'stix?', got %v", res.Matches)
	}

	// Test 6: Invalid filter type should return 0 matches, not all
	res, err = idx.Find(FindOptions{FilterType: "nonexistent"})
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	if len(res.Matches) != 0 {
		t.Errorf("expected 0 matches for invalid filter type, got %d", len(res.Matches))
	}

	// Test 7: Invalid filter folder should return 0 matches, not all
	res, err = idx.Find(FindOptions{FilterFolder: "NonExistentFolder"})
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	if len(res.Matches) != 0 {
		t.Errorf("expected 0 matches for invalid filter folder, got %d", len(res.Matches))
	}
}

func TestOpenIndex_CorruptedData(t *testing.T) {
	// Test header too short
	_, err := OpenIndex([]byte("C64F"), "/root")
	if err == nil {
		t.Errorf("expected error for short header, got nil")
	}

	// Test corrupted entry count without panic
	corrupted := make([]byte, 20)
	copy(corrupted[0:4], IndexMagic)
	// uint32 version = 1
	corrupted[4] = 1
	// uint32 entryCount = 10000 (larger than byte slice)
	corrupted[8] = 0x10
	corrupted[9] = 0x27

	_, err = OpenIndex(corrupted, "/root")
	if err == nil {
		t.Errorf("expected error for corrupted slice bounds, got nil")
	}
}

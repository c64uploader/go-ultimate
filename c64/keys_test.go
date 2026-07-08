package c64

import "testing"

func TestEncodeKeysFoldCase(t *testing.T) {
	got := string(EncodeKeys("load RUN\n", FoldCase))
	want := "LOAD RUN\r"
	if got != want {
		t.Fatalf("EncodeKeys fold = %q, want %q", got, want)
	}
}

func TestEncodeKeysLiteralCase(t *testing.T) {
	got := string(EncodeKeys("Aa\n", LiteralCase))
	want := "Aa\r"
	if got != want {
		t.Fatalf("EncodeKeys literal = %q, want %q", got, want)
	}
}

func TestCombineKeys(t *testing.T) {
	col, row := CombineKeys(KeyLeftShift, KeyA)
	if col != 0xFD || row != 0x7B {
		t.Fatalf("shift+A = $%02X/$%02X, want $FD/$7B", col, row)
	}
	col, row = CombineKeys(KeySpace)
	if col != 0x7F || row != 0xEF {
		t.Fatalf("space = $%02X/$%02X, want $7F/$EF", col, row)
	}
}

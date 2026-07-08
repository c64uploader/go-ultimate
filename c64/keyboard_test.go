package c64

import "testing"

func TestKeySpaceMatrix(t *testing.T) {
	if KeySpace.Column != 0x7F || KeySpace.Row != 0xEF {
		t.Fatalf("KeySpace = $%02X/$%02X, want $7F/$EF", KeySpace.Column, KeySpace.Row)
	}
}
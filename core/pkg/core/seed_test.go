package core

import "testing"

func TestSeedHexStable(t *testing.T) {
	got := SeedHex("abc")
	want := "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got != want {
		t.Fatalf("SeedHex mismatch\nwant: %s\n got: %s", want, got)
	}
}

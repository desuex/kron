package core

import "testing"

func TestSplitMix64Deterministic(t *testing.T) {
	a := NewSplitMix64(42)
	b := NewSplitMix64(42)
	for i := 0; i < 64; i++ {
		av := a.Uint64()
		bv := b.Uint64()
		if av != bv {
			t.Fatalf("step %d mismatch: %d != %d", i, av, bv)
		}
	}
}

func TestSplitMix64DifferentSeedsDiffer(t *testing.T) {
	a := NewSplitMix64(1)
	b := NewSplitMix64(2)
	if a.Uint64() == b.Uint64() {
		t.Fatalf("first values unexpectedly equal")
	}
}

func TestSplitMix64Float64DeterministicAndBounded(t *testing.T) {
	a := NewSplitMix64(42)
	b := NewSplitMix64(42)
	for i := 0; i < 64; i++ {
		av := a.Float64()
		bv := b.Float64()
		if av != bv {
			t.Fatalf("step %d mismatch: %f != %f", i, av, bv)
		}
		if av < 0 || av >= 1 {
			t.Fatalf("step %d out of range [0,1): %f", i, av)
		}
	}
}

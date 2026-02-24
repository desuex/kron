package core

// SplitMix64 is a deterministic PRNG used as a stable sampler source.
type SplitMix64 struct {
	state uint64
}

func NewSplitMix64(seed uint64) *SplitMix64 {
	return &SplitMix64{state: seed}
}

func (s *SplitMix64) Uint64() uint64 {
	s.state += 0x9E3779B97F4A7C15
	z := s.state
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

// Float64 returns a value in [0,1) using the high 53 bits of Uint64 output.
func (s *SplitMix64) Float64() float64 {
	return float64(s.Uint64()>>11) / float64(uint64(1)<<53)
}

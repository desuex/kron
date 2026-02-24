package core

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
)

// SeedHash returns a stable SHA-256 hash for seed material.
func SeedHash(material string) [32]byte {
	return sha256.Sum256([]byte(material))
}

// SeedHex returns the lowercase hex encoding for seed material hash.
func SeedHex(material string) string {
	hash := SeedHash(material)
	return hex.EncodeToString(hash[:])
}

// SeedUint64 maps the first 8 hash bytes to a deterministic uint64 seed.
func SeedUint64(hash [32]byte) uint64 {
	return binary.BigEndian.Uint64(hash[0:8])
}

package crawler

import (
	"strings"

	"github.com/zeebo/xxh3"
)

type Shingle []string

func CreateSimhashFingerprint(text string) uint64 {
	//Google standard
	shingleSize := 8

	shingles := GenerateShingles(text, shingleSize)
	hashed := HashShingles(shingles)
	cShingles := CollapseShingles(hashed)
	simhash := ShrinkVector(cShingles)
	return simhash
}

// esssentially a sliding window
func GenerateShingles(text string, shingleSize int) []Shingle {
	out := make([]Shingle, 0)
	var tokens = strings.Split(text, " ")

	totalShingle := len(tokens) - shingleSize + 1
	if shingleSize <= 0 {
		return []Shingle{}
	}

	for i := range totalShingle {
		out = append(out, tokens[i:i+shingleSize])
	}
	return out
}

func HashShingles(shingles []Shingle) []uint64 {
	out := make([]uint64, len(shingles))

	for i, shingle := range shingles {
		normalized := strings.TrimSpace(strings.ToLower(strings.Join(shingle, " ")))
		out[i] = xxh3.HashString(normalized)
	}

	return out
}

func CollapseShingles(hashedShingles []uint64) [64]int8 {
	var out [64]int8

	for _, shingle := range hashedShingles {
		for j := range int8(64) {
			bit := (shingle >> j) & 1

			if bit == 1 {
				out[j]++
			} else {
				out[j]--
			}
		}
	}

	return out
}

func ShrinkVector(vector [64]int8) uint64 {
	var fingerprint uint64 = 0

	for _, bit := range vector {
		fingerprint <<= 1

		if bit > 0 {
			fingerprint |= 1
		}
	}

	return fingerprint
}

func ListBits64(hash uint64) []int {
	bits := make([]int, 64)
	for i := 0; i < 64; i++ {
		bits[63-i] = int((hash >> i) & 1)
	}
	return bits
}

// divide and conquer
func CompareSimHashes() {}

package crawler

import (
	"math/bits"
	"slices"
	"strings"
	"sync"

	"github.com/zeebo/xxh3"
)

/*

All of this could probably be rewritten with math/bits package

*/

type Shingle []string

type SimHashIndex struct {
	tables          [][]uint64
	hammingDistance int

	mu sync.Mutex
}

func NewSimHashIndex(hammingDistance int, seed bool) *SimHashIndex {
	var tables [][]uint64

	if seed {
		tables = [][]uint64{
			{0x1F3A9C2E7B105D48, 0xA47C2E9F31B8D065, 0x0C9E5A2F7D31B846, 0x8E2F1A9C4D703B5E, 0x3B9F0C2E7A415D68},
			{0x7A1C3E9F205B4D86, 0xE29F4C1A7B305D9E, 0x1D4A9E2F7C305B48, 0xF3B2C9E7A104D65C, 0x9C2E7F1A4B305D9E},
			{0x4E9C2A1F7B305D68, 0xB1C7E2A9F304D65C, 0x2F9E4C1A7D305B48, 0xD3A2C9E7F104B65E, 0x5C1A9E2F7B304D68, 0x1F3A9C2E7B105D48},
			{0x8B3F1C9E2A705D4E, 0x6E2C9A1F7B304D5E, 0xC1A9E2F73B405D68, 0x3E7A2C9F1B304D6E, 0xA9F1C2E73B405D8E},
		}

		for i := range tables {
			slices.Sort(tables[i])
		}

	} else {
		tables = [][]uint64{
			make([]uint64, 0),
			make([]uint64, 0),
			make([]uint64, 0),
			make([]uint64, 0),
		}
	}

	return &SimHashIndex{
		tables:          tables,
		hammingDistance: hammingDistance,
	}
}

func (shi *SimHashIndex) IsDuplicate(fingerprint uint64) (bool, uint64) {
	shi.mu.Lock()
	defer shi.mu.Unlock()
	return shi._isDuplicate(fingerprint)
}

func (shi *SimHashIndex) _isDuplicate(fingerprint uint64) (bool, uint64) {
	candidates := make([]uint64, 0)

	for i, fingerprints := range shi.tables {
		targetChunk := GetChunk(fingerprint, i)

		found, exists := shi.BinarySearchChunk(i, targetChunk, fingerprints)
		if exists {
			candidates = append(candidates, found)
		}
	}

	for _, candidate := range candidates {
		if AreFingerprintsSimilar(candidate, fingerprint, shi.hammingDistance) {
			return true, candidate
		}
	}

	return false, 0
}

func (shi *SimHashIndex) BinarySearchChunk(chunkNumber int, targetChunk uint64, fingerprints []uint64) (uint64, bool) {
	var fingerprint uint64

	if len(fingerprints) == 0 {
		return 0, false
	}
	_, found := slices.BinarySearchFunc(fingerprints, targetChunk, func(e uint64, t uint64) int {
		foundChunk := GetChunk(e, chunkNumber)

		if targetChunk < foundChunk {
			return -1
		} else if targetChunk > foundChunk {
			return 1
		}
		fingerprint = e

		return 0
	})

	if found {
		return fingerprint, true
	}

	return 0, false
}

func (shi *SimHashIndex) TryInsert(fingerprint uint64) (bool, uint64) {
	if yes, other := shi._isDuplicate(fingerprint); yes {
		return false, other
	}

	for i, fingerprints := range shi.tables {
		shi.tables[i] = append(fingerprints, fingerprint)

		slices.SortFunc(shi.tables[i], func(a, b uint64) int {
			return int(GetChunk(a, i) - GetChunk(b, i))
		})
	}

	return true, 0
}

func GetChunk(fingerprint uint64, chunkNumber int) uint64 {
	return (fingerprint >> uint64(chunkNumber)) & ((1 << 16) - 1)
}

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
func AreFingerprintsSimilar(f1, f2 uint64, threshold int) bool {
	out := f1 ^ f2
	hammingDistance := bits.OnesCount64(out)
	return hammingDistance <= threshold
}

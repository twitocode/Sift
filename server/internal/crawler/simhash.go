package crawler

import (
	"math/bits"
	"slices"
	"strings"
	"sync"

	"github.com/twitocode/sift/internal/common"
	"github.com/zeebo/xxh3"
)

/*

All of this could probably be rewritten with math/bits package

*/

type Shingle []string

type SimHashIndex struct {
	tables          [][]IndexedFingerprint
	hammingDistance int

	mu sync.Mutex
}

type IndexedFingerprint struct {
	fingerprint uint64
	pageIndex   int
}

type BinarySearchResult struct {
	fingerprint uint64
	index       int
}

func NewSimHashIndex(hammingDistance int) *SimHashIndex {
	var tables [][]IndexedFingerprint

		tables = [][]IndexedFingerprint{
			make([]IndexedFingerprint, 0),
			make([]IndexedFingerprint, 0),
			make([]IndexedFingerprint, 0),
			make([]IndexedFingerprint, 0),
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
	candidates := shi.FindSimilar(fingerprint)

	for _, candidate := range candidates {
		if AreFingerprintsSimilar(candidate.fingerprint, fingerprint, shi.hammingDistance) {
			return true, candidate.fingerprint
		}
	}

	return false, 0
}

func (shi *SimHashIndex) FindSimilar(fingerprint uint64) []BinarySearchResult {
	candidates := make([]BinarySearchResult, 0)

	for i, fingerprints := range shi.tables {
		targetChunk := GetChunk(fingerprint, i)

		found, exists := shi.BinarySearchChunk(i, targetChunk, fingerprints)
		if exists {
			candidates = slices.Concat(candidates, found)
		}
	}

	return candidates
}

func (shi *SimHashIndex) BinarySearchChunk(chunkNumber int, targetChunk uint64, fingerprints []IndexedFingerprint) ([]BinarySearchResult, bool) {

	//each chunk array is already sorted so I find an index that in a section of similar fingerprints and then i expand outwards until I find a dissimlar fingerprint

	if len(fingerprints) == 0 {
		return []BinarySearchResult{}, false
	}

	index, found := slices.BinarySearchFunc(fingerprints, targetChunk, func(e IndexedFingerprint, t uint64) int {
		foundChunk := GetChunk(e.fingerprint, chunkNumber)

		if targetChunk < foundChunk {
			return 1
		} else if targetChunk > foundChunk {
			return -1
		}

		return 0
	})

	if !found {
		return []BinarySearchResult{}, false
	}

	start := index
	end := index + 1

	for start > 0 && GetChunk(fingerprints[start-1].fingerprint, chunkNumber) == targetChunk {
		start--
	}

	for end < len(fingerprints) && GetChunk(fingerprints[end].fingerprint, chunkNumber) == targetChunk {
		end++
	}

	return common.Map(fingerprints[start:end], func(e IndexedFingerprint, i int) (BinarySearchResult, bool) {
		return BinarySearchResult{
			fingerprint: e.fingerprint,
			index:       e.pageIndex,
		}, true
	}), true
}

func (shi *SimHashIndex) TryInsert(fingerprint uint64, pageIndex int) (bool, uint64) {
	if yes, other := shi._isDuplicate(fingerprint); yes {
		return false, other
	}

	shi.DryInsert(fingerprint, pageIndex)

	return true, 0
}

func (shi *SimHashIndex) DryInsert(fingerprint uint64, pageIndex int) {
	for i, fingerprints := range shi.tables {
		shi.tables[i] = append(fingerprints, IndexedFingerprint{
      fingerprint: fingerprint,
      pageIndex: pageIndex,
    })

		slices.SortFunc(shi.tables[i], func(a, b IndexedFingerprint) int {
			return int(GetChunk(a.fingerprint, i) - GetChunk(b.fingerprint, i))
		})
	}
}

func GetChunk(fingerprint uint64, chunkNumber int) uint64 {
	return (fingerprint >> uint64(chunkNumber*16)) & ((1 << 16) - 1)
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

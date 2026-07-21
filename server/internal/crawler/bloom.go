package crawler

import (
	"math"

	"github.com/zeebo/xxh3"
)

type BloomFilter struct {
	numBits        uint64
	bits           []byte
	hashIterations uint64
}

func NewBloomFilter(expectedItems, falsePositiveRate float64) *BloomFilter {
	numBits := uint64(
		-(expectedItems * math.Log(falsePositiveRate)) /
			(math.Pow(math.Log(2), 2)),
	)

	hashIterations := uint64(math.Round((float64(numBits) / expectedItems) * math.Ln2))

	numBytes := (numBits + hashIterations) / 8

	return &BloomFilter{
		//8 bits in byte
		numBits:        numBits,
		bits:           make([]byte, numBytes),
		hashIterations: hashIterations,
	}
}

func (bf *BloomFilter) Insert(url URL) {
	hash128 := xxh3.HashString128(url.String())

	h1 := hash128.Lo
	h2 := hash128.Hi

	for i := uint64(0); i < bf.hashIterations; i++ {
		index := (h1 + uint64(i)*h2) % bf.numBits
		bitPosition := index % 8

		bf.bits[index/8] |= (1 << bitPosition)
	}
}

func (bf *BloomFilter) ProbablyContains(url URL) bool {
	hash128 := xxh3.HashString128(url.String())

	h1 := hash128.Lo
	h2 := hash128.Hi

	for i := uint64(0); i < bf.hashIterations; i++ {
		index := (h1 + uint64(i)*h2) % bf.numBits
		bitPosition := index % 8

		if bf.bits[index/8]&(1<<bitPosition) != (1 << bitPosition) {
			return false
		}
	}

	return true
}

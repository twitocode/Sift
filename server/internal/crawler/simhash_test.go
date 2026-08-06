package crawler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSimHashIndex(t *testing.T) {
	t.Run("seeded index", func(t *testing.T) {
		got := NewSimHashIndex(3, true)

		assert.Equal(t, 3, got.hammingDistance)
		require.Len(t, got.tables, 4)
		for i, table := range got.tables {
			assert.NotEmpty(t, table, "tables[%d] should not be empty", i)
		}
	})

	t.Run("empty index", func(t *testing.T) {
		got := NewSimHashIndex(5, false)

		assert.Equal(t, 5, got.hammingDistance)
		require.Len(t, got.tables, 4)
		for i, table := range got.tables {
			assert.Empty(t, table, "tables[%d] should be empty", i)
		}
	})
}

func TestSimHashIndex_IsDuplicate(t *testing.T) {
	shi := NewSimHashIndex(3, false)

	base := uint64(0x1F3A9C2E7B105D48)
	ok, _ := shi.TryInsert(base)
	require.True(t, ok)

	tests := []struct {
		name        string
		fingerprint uint64
		want        bool
	}{
		{
			name:        "exact match after insert",
			fingerprint: base,
			want:        true,
		},
		{
			name:        "near duplicate within threshold",
			fingerprint: 0x1F3A9C2E7B105D4F,
			want:        true,
		},
		{
			name:        "unrelated fingerprint",
			fingerprint: 0xFFFFFFFFFFFFFFFF,
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			duplicate, _ := shi.IsDuplicate(tt.fingerprint)
			assert.Equal(t, tt.want, duplicate)
		})
	}
}

func TestSimHashIndex_TryInsert(t *testing.T) {
	shi := NewSimHashIndex(3, false)

	ok, other := shi.TryInsert(0xABC)
	require.True(t, ok)
	assert.Equal(t, uint64(0), other)

	ok, other = shi.TryInsert(0xABC)
	assert.False(t, ok)
	assert.Equal(t, uint64(0xABC), other)
}

func TestSimHashIndex_BinarySearchChunk(t *testing.T) {
	shi := NewSimHashIndex(3, false)
	// Sorted by chunk 0: 0x10000 (chunk 0), then 0x1 and 0x10001 (chunk 1).
	fingerprints := []uint64{0x10000, 0x1, 0x10001}

	got, found := shi.BinarySearchChunk(0, 1, fingerprints)
	assert.True(t, found)
	assert.True(t, got == 0x1 || got == 0x10001)

	t.Run("empty fingerprints", func(t *testing.T) {
		got, found := shi.BinarySearchChunk(0, 1, nil)
		assert.False(t, found)
		assert.Equal(t, uint64(0), got)
	})

	t.Run("single element match", func(t *testing.T) {
		got, found := shi.BinarySearchChunk(0, 0x10, []uint64{0x10})
		assert.True(t, found)
		assert.Equal(t, uint64(0x10), got)
	})

	t.Run("single element no match", func(t *testing.T) {
		got, found := shi.BinarySearchChunk(0, 0x20, []uint64{0x10})
		assert.False(t, found)
		assert.Equal(t, uint64(0), got)
	})
}

func TestGetChunk(t *testing.T) {
	const fingerprint = uint64(0x123456789ABCDEF0)

	tests := []struct {
		name        string
		chunkNumber int
		want        uint64
	}{
		{name: "chunk 0", chunkNumber: 0, want: 0xDEF0},
		{name: "chunk 1", chunkNumber: 1, want: 0x6F78},
		{name: "chunk 2", chunkNumber: 2, want: 0x37BC},
		{name: "chunk 3", chunkNumber: 3, want: 0x9BDE},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, GetChunk(fingerprint, tt.chunkNumber))
		})
	}
}

func TestCreateSimhashFingerprint(t *testing.T) {
	text := "the quick brown fox jumps over the lazy dog"
	want := uint64(0x10A4080925016022)

	got := CreateSimhashFingerprint(text)
	assert.Equal(t, want, got)
	assert.Equal(t, got, CreateSimhashFingerprint(text))
}

func TestGenerateShingles(t *testing.T) {
	tests := []struct {
		name        string
		text        string
		shingleSize int
		want        []Shingle
	}{
		{
			name:        "two word shingles",
			text:        "one two three four",
			shingleSize: 2,
			want: []Shingle{
				{"one", "two"},
				{"two", "three"},
				{"three", "four"},
			},
		},
		{
			name:        "text shorter than shingle size",
			text:        "only",
			shingleSize: 3,
			want:        []Shingle{},
		},
		{
			name:        "non-positive shingle size",
			text:        "one two three",
			shingleSize: 0,
			want:        []Shingle{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, GenerateShingles(tt.text, tt.shingleSize))
		})
	}
}

func TestHashShingles(t *testing.T) {
	shingles := []Shingle{
		{"one", "two"},
		{"two", "three"},
		{"three", "four"},
	}

	want := []uint64{
		16650328638832365648,
		9445753358084541375,
		11825644699749150687,
	}

	assert.Equal(t, want, HashShingles(shingles))
}

func TestCollapseShingles(t *testing.T) {
	t.Run("single hash sets first bit positive", func(t *testing.T) {
		got := CollapseShingles([]uint64{1})

		assert.Equal(t, int8(1), got[0])
		for i := 1; i < 64; i++ {
			assert.Equal(t, int8(-1), got[i], "got[%d]", i)
		}
	})

	t.Run("empty input leaves zero vector", func(t *testing.T) {
		got := CollapseShingles(nil)
		assert.Equal(t, [64]int8{}, got)
	})
}

func TestShrinkVector(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*[64]int8)
		want   uint64
	}{
		{
			name:   "all zeros",
			mutate: func(v *[64]int8) {},
			want:   0,
		},
		{
			name: "most significant bit set",
			mutate: func(v *[64]int8) {
				v[63] = 1
			},
			want: 0x1,
		},
		{
			name: "least significant bit set",
			mutate: func(v *[64]int8) {
				v[0] = 1
			},
			want: 0x8000000000000000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var vector [64]int8
			tt.mutate(&vector)

			assert.Equal(t, tt.want, ShrinkVector(vector))
		})
	}
}

func TestListBits64(t *testing.T) {
	tests := []struct {
		name string
		hash uint64
		want []int
	}{
		{
			name: "zero",
			hash: 0,
			want: make([]int, 64),
		},
		{
			name: "value five",
			hash: 5,
			want: func() []int {
				bits := make([]int, 64)
				bits[61] = 1
				bits[63] = 1
				return bits
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ListBits64(tt.hash))
		})
	}
}

func TestAreFingerprintsSimilar(t *testing.T) {
	tests := []struct {
		name      string
		f1        uint64
		f2        uint64
		threshold int
		want      bool
	}{
		{
			name:      "identical fingerprints",
			f1:        0xABC,
			f2:        0xABC,
			threshold: 0,
			want:      true,
		},
		{
			name:      "one bit apart within threshold",
			f1:        5,
			f2:        4,
			threshold: 1,
			want:      true,
		},
		{
			name:      "one bit apart above threshold",
			f1:        5,
			f2:        4,
			threshold: 0,
			want:      false,
		},
		{
			name:      "maximally different",
			f1:        0,
			f2:        ^uint64(0),
			threshold: 64,
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, AreFingerprintsSimilar(tt.f1, tt.f2, tt.threshold))
		})
	}
}

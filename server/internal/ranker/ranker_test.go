package ranker

import (
	"testing"

	"github.com/twitocode/sift/internal/common"
)

func TestSortPagesByScoreDescending(t *testing.T) {
	pages := []*common.Page{
		{ID: 1},
		{ID: 2},
		{ID: 3},
	}
	scores := map[uint32]float64{
		1: 1.25,
		2: 3.75,
		3: 2.5,
	}

	sortPagesByScore(pages, scores)

	got := []int64{pages[0].ID, pages[1].ID, pages[2].ID}
	want := []int64{2, 3, 1}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sorted page IDs = %v, want %v", got, want)
		}
	}
}

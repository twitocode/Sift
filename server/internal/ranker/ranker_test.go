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

func TestCollectDuplicateURLsDoesNotDependOnResultOrder(t *testing.T) {
	pages := []*common.Page{
		{
			ID:          45483,
			DuplicateOf: -1,
		},
		{
			ID:           260562,
			DuplicateOf:  45483,
			RequestedURL: common.URL("https://raw.githubusercontent.com"),
		},
		{
			ID:           71400,
			DuplicateOf:  260562,
			RequestedURL: common.URL("https://gh.io"),
		},
	}

	duplicates := collectDuplicateURLs(pages)
	got := duplicates[45483]

	if len(got) != 2 ||
		got[0] != "https://raw.githubusercontent.com" ||
		got[1] != "https://gh.io" {
		t.Fatalf(
			"duplicates[45483] = %v, want [https://raw.githubusercontent.com https://gh.io]",
			got,
		)
	}
}

func TestAveragePostingScanDuration(t *testing.T) {
	got := averagePostingScanDuration(1800, 3)
	want := float64(600)

	if got != want {
		t.Fatalf("averagePostingScanDuration() = %v, want %v", got, want)
	}
}

func TestDomainMatchRanksFarAboveURLPathMatch(t *testing.T) {
	domainBoost := domainMatchBoost(1)
	urlBoost := urlMatchBoost(1)

	if domainBoost < urlBoost*5 {
		t.Fatalf("domain boost %v should be at least 5x URL path boost %v", domainBoost, urlBoost)
	}
}

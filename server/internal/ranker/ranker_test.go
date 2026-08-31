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
	idf := 4.0
	domainBoost := domainMatchBoost(1, idf)
	urlBoost := urlMatchBoost(1, idf)

	if domainBoost < urlBoost*5 {
		t.Fatalf("domain boost %v should be at least 5x URL path boost %v", domainBoost, urlBoost)
	}
}

func TestCommonDomainTokenBoostIsMuchSmallerThanRareOne(t *testing.T) {
	stats := &common.IndexStats{DocumentCount: 480700}
	commonBoost := domainMatchBoost(1, ComputeIDF(stats, 55550))
	rareBoost := domainMatchBoost(1, ComputeIDF(stats, 823))

	if commonBoost >= rareBoost {
		t.Fatalf("common domain boost %v should be less than rare boost %v", commonBoost, rareBoost)
	}
	if commonBoost >= 20 {
		t.Fatalf("common domain boost %v is still large enough to drown BM25", commonBoost)
	}
}

func TestCoverageMultiplierPenalizesPartialMatches(t *testing.T) {
	partial := coverageMultiplier(1, 3)
	full := coverageMultiplier(3, 3)

	if full != 1 {
		t.Fatalf("coverage 3/3 = %v, want 1", full)
	}
	if partial >= full {
		t.Fatalf("coverage 1/3 (%v) should be less than 3/3 (%v)", partial, full)
	}
	if partial >= 0.2 {
		t.Fatalf("coverage 1/3 multiplier %v should heavily penalize single-term matches", partial)
	}
}

func TestConcatenatedTermsJoinsAdjacentQueryWords(t *testing.T) {
	got := concatenatedTerms([]string{"anne", "hathaway", "archive"})
	for _, want := range []string{
		"annehathaway",
		"anne-hathaway",
		"annehathawayarchive",
		"anne-hathaway-archive",
		"hathawayarchive",
		"hathaway-archive",
	} {
		if !contains(got, want) {
			t.Fatalf("concatenatedTerms() = %v, want to contain %q", got, want)
		}
	}
}

func TestQueryLookupTokensKeepOriginalsAndConcatenations(t *testing.T) {
	got := queryLookupTokens("anne hathaway archive")
	for _, want := range []string{
		"anne",
		"hathaway",
		"archive",
		"archiv",
		"annehathaway",
		"anne-hathaway",
		"annehathawayarchive",
	} {
		if !contains(got, want) {
			t.Fatalf("queryLookupTokens() = %v, want to contain %q", got, want)
		}
	}
	if contains(got, "ann") {
		t.Fatalf("queryLookupTokens() = %v, should not stem anne to ann", got)
	}
}

func TestOriginalsCoveredByStemAndConcatenation(t *testing.T) {
	originals := []string{"anne", "hathaway", "archive"}

	got := originalsCoveredByToken("archiv", originals)
	if !contains(got, "archive") || len(got) != 1 {
		t.Fatalf("archiv should cover [archive], got %v", got)
	}

	got = originalsCoveredByToken("annehathaway", originals)
	if !contains(got, "anne") || !contains(got, "hathaway") {
		t.Fatalf("annehathaway should cover anne and hathaway, got %v", got)
	}

	got = originalsCoveredByToken("ann", originals)
	if len(got) != 0 {
		t.Fatalf("ann should not cover anne, got %v", got)
	}
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

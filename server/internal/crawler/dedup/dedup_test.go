package dedup

import (
	"testing"

	"github.com/twitocode/sift/internal/common"
)

func TestAddCanonicalCandidateKeepsLowestIDAndRetainsOthers(t *testing.T) {
	canonicals := make(map[common.URL]*CanonicalInfo)
	key := common.URL("https://github.com")

	addCanonicalCandidate(canonicals, &common.Page{
		ID:             260562,
		FoundCanonical: key,
	})
	addCanonicalCandidate(canonicals, &common.Page{
		ID:             45483,
		FoundCanonical: key,
	})
	addCanonicalCandidate(canonicals, &common.Page{
		ID:             93727,
		FoundCanonical: key,
	})

	info := canonicals[key]
	if info == nil {
		t.Fatal("canonical group was not created")
	}
	if info.page.ID != 45483 {
		t.Fatalf("canonical ID = %d, want 45483", info.page.ID)
	}

	got := map[int64]bool{}
	for _, page := range info.similar {
		got[page.ID] = true
	}
	for _, id := range []int64{260562, 93727} {
		if !got[id] {
			t.Errorf("candidate %d was not retained as a duplicate", id)
		}
	}
}

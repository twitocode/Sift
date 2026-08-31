package indexer

import (
	"context"
	"testing"

	"github.com/twitocode/sift/internal/common"
	"go.uber.org/zap"
)

func TestIndexSeparatesDomainTokensFromURLPathTokens(t *testing.T) {
	in := NewIndexer(zap.NewNop(), nil, nil, nil)
	page := &common.Page{
		ID:       42,
		FinalURL: common.URL("https://github.com/openai/github-helps"),
	}

	in.Index(context.Background(), page)

	for _, token := range []string{"github.com", "github"} {
		postings, ok := in.index.Get(token)
		if !ok || len(postings) != 1 {
			t.Fatalf("postings for %q = %v, want one posting", token, postings)
		}
		if postings[0].DomainFrequency == 0 {
			t.Fatalf("DomainFrequency for %q = 0, want domain match", token)
		}
		if token == "github.com" && postings[0].URLFrequency != 0 {
			t.Fatalf("URLFrequency for domain token %q = %d, want 0", token, postings[0].URLFrequency)
		}
	}

	for _, token := range []string{"openai", "github-helps", "help"} {
		postings, ok := in.index.Get(token)
		if !ok || len(postings) != 1 {
			t.Fatalf("postings for %q = %v, want one posting", token, postings)
		}
		if postings[0].URLFrequency == 0 {
			t.Fatalf("URLFrequency for %q = 0, want path match", token)
		}
		if postings[0].DomainFrequency != 0 {
			t.Fatalf("DomainFrequency for path token %q = %d, want 0", token, postings[0].DomainFrequency)
		}
	}
}

func TestIndexSplitsHyphenatedDomainLabels(t *testing.T) {
	in := NewIndexer(zap.NewNop(), nil, nil, nil)
	page := &common.Page{
		ID:       7,
		FinalURL: common.URL("https://anne-hathaway.org/gallery"),
	}

	in.Index(context.Background(), page)

	for _, token := range []string{"anne", "hathaway", "anne-hathaway"} {
		postings, ok := in.index.Get(token)
		if !ok || len(postings) != 1 {
			t.Fatalf("postings for %q = %v, want one posting", token, postings)
		}
		if postings[0].DomainFrequency == 0 {
			t.Fatalf("DomainFrequency for %q = 0, want domain match", token)
		}
	}

	if _, ok := in.index.Get("ann"); ok {
		t.Fatalf("indexed stemmed domain token %q", "ann")
	}
}

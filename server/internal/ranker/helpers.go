package ranker

import (
	"slices"
	"strings"

	"github.com/twitocode/sift/internal/common"
	"github.com/twitocode/sift/internal/indexer"
)

func sortPagesByScore(pages []*common.Page, scores map[uint32]float64) {
	slices.SortFunc(pages, func(a *common.Page, b *common.Page) int {
		switch {
		case scores[uint32(a.ID)] > scores[uint32(b.ID)]:
			return -1
		case scores[uint32(a.ID)] < scores[uint32(b.ID)]:
			return 1
		default:
			return 0
		}
	})
}

func collectDuplicateURLs(pages []*common.Page) map[int64][]string {
	parents := make(map[int64]int64, len(pages))
	for _, page := range pages {
		if page.DuplicateOf >= 0 {
			parents[page.ID] = page.DuplicateOf
		}
	}

	duplicates := make(map[int64][]string)
	for _, page := range pages {
		if page.DuplicateOf < 0 {
			continue
		}

		canonicalID := page.DuplicateOf
		for range len(parents) {
			parentID, exists := parents[canonicalID]
			if !exists {
				break
			}
			canonicalID = parentID
		}

		duplicates[canonicalID] = append(
			duplicates[canonicalID],
			page.RequestedURL.String(),
		)
	}
	return duplicates
}

func uniqueTokens(tokens []string) []string {
	seen := make(map[string]struct{}, len(tokens))
	out := make([]string, 0, len(tokens))

	for _, token := range tokens {
		if token == "" {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}

		seen[token] = struct{}{}
		out = append(out, token)
	}
	return out
}

func concatenatedTerms(parts []string) []string {
	var out []string

	for i := range parts {
		var joined strings.Builder
		var hyphen strings.Builder

		joined.WriteString(parts[i])
		hyphen.WriteString(parts[i])

		for j := i + 1; j < len(parts); j++ {
			joined.WriteString(parts[j])
			hyphen.WriteString("-")
			hyphen.WriteString(parts[j])

			out = append(out, joined.String(), hyphen.String())
		}
	}
	return out
}

func queryLookupTokens(query string) []string {
	originals := strings.Fields(query)
	tokens := indexer.TokenizeQuery(query)
	tokens = append(tokens, concatenatedTerms(originals)...)

	return uniqueTokens(tokens)
}

func originalsCoveredByToken(token string, originals []string) []string {
	var covered []string
	seen := make(map[string]struct{})

	add := func(original string) {
		if _, ok := seen[original]; ok {
			return
		}
		seen[original] = struct{}{}
		covered = append(covered, original)
	}

	for _, original := range originals {
		if token == original || slices.Contains(indexer.Tokenize(original), token) {
			add(original)
		}
	}

	for i := range originals {
		var joined strings.Builder
		var hyphen strings.Builder
    
		joined.WriteString(originals[i])
		hyphen.WriteString(originals[i])
		used := []string{originals[i]}

		for j := i + 1; j < len(originals); j++ {
			used = append(used, originals[j])

			joined.WriteString(originals[j])
			hyphen.WriteString("-")
			hyphen.WriteString(originals[j])

			if token != joined.String() && token != hyphen.String() {
				continue
			}

			for _, original := range used {
				add(original)
			}
		}
	}

	return covered
}

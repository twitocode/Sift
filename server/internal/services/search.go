package services

import (
	"context"

	"github.com/twitocode/sift/internal/ranker"
)

type SearchService struct {
	ranker *ranker.Ranker
}

func NewSearchService(ranker *ranker.Ranker) *SearchService {
	return &SearchService{
		ranker: ranker,
	}
}

func (ss *SearchService) Search(ctx context.Context, q string) ranker.QueryResult {
	return ss.ranker.Query(ctx, q)
}

package ranker

import (
	"github.com/twitocode/sift/internal/common"
	"github.com/twitocode/sift/internal/metrics"
)

type TokenStats struct {
	PostingsCount int64 `json:"postings_count"`
	ScanTime      int64 `json:"scan_time"`
}

type QueryResult struct {
	Results []common.SearchResult `json:"results"`
	Count   int                   `json:"count"`

	//milliseconds
	TimeElapsed int64 `json:"time_elapsed"`

	//microseconds
	AveragePostingsScanDuration float64               `json:"average_postings_scan_duration"`
	TokenStats                  map[string]TokenStats `json:"token_stats"`
	PossibleResultsQueried      int                   `json:"possible_results"`

	IndexerMetrics SimpleIndexerMetrics `json:"index_metrics"`
}

type SimpleIndexerMetrics struct {
	DocumentsRead    int64 `json:"docs_read"`
	DocumentsIndexed int64 `json:"docs_indexed"`

	BodyTokens  int64 `json:"body_tokens"`
	TitleTokens int64 `json:"title_tokens"`

	UniqueTerms int64 `json:"unique_terms"`

	TotalPostings int64 `json:"total_postings"`
	TitlePostings int64 `json:"title_postings"`

	TimeElapsed int64 `json:"time_elapsed"`
}

func ToSimpleIndexerMetrics(metrics *metrics.IndexerMetrics) SimpleIndexerMetrics {
	return SimpleIndexerMetrics{
		DocumentsRead:    metrics.DocumentsRead.Load(),
		DocumentsIndexed: metrics.DocumentsIndexed.Load(),

		BodyTokens:  metrics.BodyTokens.Load(),
		TitleTokens: metrics.TitleTokens.Load(),

		UniqueTerms: metrics.UniqueTerms.Load(),

		TotalPostings: metrics.TotalPostings.Load(),
		TitlePostings: metrics.TitlePostings.Load(),

		TimeElapsed: metrics.TimeElapsed.Load(),
	}
}

package common

import (
	"sync"
	"time"
)

type Page struct {
	ID                int64
	FinalURL          URL
	RequestedURL      URL
	Host              URL
	Title             string
	OGTitle           string
	Favicon           URL
	Description       string
	Text              string
	Links             []URL
	StatusCode        int
	CrawledAt         time.Time
	ContentHash       uint64
	DuplicateOf       int64
	FoundCanonical    URL
	InEnglish         bool
	HasBeenCrawled    bool
	ResolvedCanonical bool
}

type Posting struct {
	PageID    uint32
	Frequency uint32

	//ranks higher
	MatchesTitle bool
	MatchesDomain bool
}

type DocumentStats struct {
	PageID     int64
	ID         int64
	TokenCount uint32
}

type IndexStats struct {
	DocumentCount    uint64
	TotalTokenCount  uint64
	AverageDocLength float64

  sync.Mutex
}

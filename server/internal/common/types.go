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
	PageID        uint32
	BodyFrequency uint32

	//ranks higher
	TitleFrequency  uint32
	DomainFrequency uint32
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

	DocumentsRead    int64
	DocumentsIndexed int64
	BodyTokens       int64
	TitleTokens      int64
	UniqueTerms      int64
	TotalPostings    int64
	TitlePostings    int64
	TimeElapsed      int64

	sync.Mutex
}

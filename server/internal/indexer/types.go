package indexer

type Posting struct {
	DocID     int64
	Frequency uint64

	//ranks higher
	MatchesTitle bool
}

type DocumentStats struct {
	TokenCount uint32

	PageID int64
	ID     int64
}

type IndexStats struct {
	DocumentCount    uint64
	TotalTokenCount  uint64
	AverageDocLength float64
}

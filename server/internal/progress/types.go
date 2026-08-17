package progress

type Snapshot struct {
	Crawl CrawlSnapshot
	Index IndexSnapshot
}

type CrawlSnapshot struct {
	Limit          int
	PagesCrawled   int
	PagesFetched   int
	PagesStored    int64
	URLsDiscovered int64
	URLsFetched    int64
	FetchFailures  int64
	InFlight       int64
	PendingURLs    int
	UniqueHosts    int
	AvailableHosts int
	LockedHosts    int
	CooldownHosts  int
}

type IndexSnapshot struct {
	DocumentsTotal   int64
	DocumentsRead    int64
	DocumentsIndexed int64
	DocumentsStored  int64
	BatchesRead      int64
	CurrentBatch     int64
	BatchSize        int64
	TotalTokens      int64
	UniqueTerms      int64
	Flushes          int64
}

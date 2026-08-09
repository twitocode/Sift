package crawler

import "time"

type Page struct {
	ID             int64
	FinalURL       URL
	RequestedURL   URL
	Host           URL
	Title          string
	Description    string
	Text           string
	Links          []URL
	StatusCode     int // some pages return 429 stuff like that so i can filter out later if needed
	CrawledAt      time.Time
	InEnglish      bool
	ContentHash    uint64 //TODO: duplication detection, hash text form page (different urls same text)
	HasBeenCrawled bool
	DuplicateOf    int64
	FoundCanonical URL
  ResolvedCanonical bool
}

type Payload struct {
	url           URL
	host          URL
	dialerContext DialerContext
}

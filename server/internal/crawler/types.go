package crawler

import "time"

type Page struct {
	URL         URL
	Host        URL
	Title       string
	Description string
	Text        string
	Links       []URL
	StatusCode  int // some pages return 429 stuff like that so i can filter out later if needed
	CrawledAt   time.Time
  InEnglish bool
	ContentHash string //TODO: duplication detection, hash text form page (different urls same text)
}

type Payload struct {
	url           URL
	host          URL
	dialerContext DialerContext
}

package crawler

import (
	"fmt"
	"runtime"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

type CrawlMetrics struct {
	URLsDiscovered     atomic.Int64
	URLsRejected       atomic.Int64
	URLsFetched        atomic.Int64
	URLsSkippedAtLimit atomic.Int64
  URLDuplicates atomic.Int64

	FetchFailures       atomic.Int64
	PagesParsed         atomic.Int64
	PagesStored         atomic.Int64
	PagesDuplicates      atomic.Int64
	BlacklistedWebsites atomic.Int64

	BytesDownloaded atomic.Int64
	InFlight        atomic.Int64

	DNSLookupFailures atomic.Int64
	ParsingFailures   atomic.Int64
	HTTP400Errors     atomic.Int64
	HTTP500Errors     atomic.Int64

	TimeElapsed  atomic.Int64
	RequestCount atomic.Int64

	log *zap.Logger
}

func NewCrawlMetrics(log *zap.Logger) *CrawlMetrics {
	return &CrawlMetrics{
		log: log,
	}
}

func (cm *CrawlMetrics) PrintSummary(duration time.Duration) {
	cm.log.Info(formatCrawlingSummary(cm, duration))

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	cm.log.Info("memory",
		zap.Uint64("heap_alloc_mb", mem.HeapAlloc/1024/1024),
		zap.Uint64("heap_inuse_mb", mem.HeapInuse/1024/1024),
		zap.Uint64("heap_objects", mem.HeapObjects),
		zap.Uint32("gc_cycles", mem.NumGC),
	)
}

func formatCrawlingSummary(cm *CrawlMetrics, duration time.Duration) string {
	return fmt.Sprintf(
		"Crawling Summary\n"+
			"  URLs Discovered:   %d\n"+
			"  URLs Fetched:      %d\n"+
			"  URLs Rejected:     %d\n"+
			"  URLs Skipped at Limit:     %d\n"+
			"  URL Duplicates:     %d\n"+
			"  Blacklisted Websites:  %d\n"+
			"  Fetch Failures:    %d\n"+
			"  Total Requests:    %d\n"+
			"  Pages Parsed:      %d\n"+
			"  Pages Stored:      %d\n"+
			"  Parsing Failures:  %d\n"+
			"  Duplicated Pages:  %d\n"+
			"  Gigabytes Downloaded:  %.2f GB\n"+
			"  Still In Flight:   %d\n"+
			"  DNS Failures:      %d\n"+
			"  Time Elapsed:      %s\n",
		cm.URLsDiscovered.Load(),
		cm.URLsFetched.Load(),
		cm.URLsRejected.Load(),
		cm.URLsSkippedAtLimit.Load(),
		cm.URLDuplicates.Load(),
		cm.BlacklistedWebsites.Load(),
		cm.FetchFailures.Load(),
		cm.RequestCount.Load(),
		cm.PagesParsed.Load(),
		cm.PagesStored.Load(),
		cm.ParsingFailures.Load(),
		cm.PagesDuplicates.Load(),
		float64(cm.BytesDownloaded.Load()) * 1e-9,
		cm.InFlight.Load(),
		cm.DNSLookupFailures.Load(),
		duration,
	)

}

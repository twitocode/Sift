package crawler

import (
	"fmt"
	"runtime"
	"strconv"
	"sync/atomic"
	"time"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"go.uber.org/zap"
)

type CrawlMetrics struct {
	URLsDiscovered     atomic.Int64
	URLsRejected       atomic.Int64
	URLsFetched        atomic.Int64
	URLsSkippedAtLimit atomic.Int64
	URLDuplicates      atomic.Int64

	FetchFailures atomic.Int64
	PagesParsed   atomic.Int64
  PagesSkipped atomic.Int64
	PagesStored         atomic.Int64
	PagesDuplicates     atomic.Int64
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
	cm.log.Info("Crawling Summary")

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	rows := getRows(cm, mem, duration)

	var (
		purple    = lipgloss.Color("99")
		gray      = lipgloss.Color("245")
		lightGray = lipgloss.Color("241")

		headerStyle  = lipgloss.NewStyle().Foreground(purple).Bold(true).Align(lipgloss.Center)
		cellStyle    = lipgloss.NewStyle().Padding(0, 1).Width(14)
		oddRowStyle  = cellStyle.Foreground(gray)
		evenRowStyle = cellStyle.Foreground(lightGray)
	)

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(purple)).
		StyleFunc(func(row, col int) lipgloss.Style {
			switch {
			case row == table.HeaderRow:
				return headerStyle
			case row%2 == 0:
				return evenRowStyle
			default:
				return oddRowStyle
			}
		}).
		Headers("Data Point", "Value").
		Width(40).
		Rows(rows...)

	lipgloss.Println(t)
}

func getRows(cm *CrawlMetrics, mem runtime.MemStats, duration time.Duration) [][]string {
	return [][]string{
		{"URLs Discovered", strconv.FormatInt(cm.URLsDiscovered.Load(), 10)},
		{"URLs Fetched", strconv.FormatInt(cm.URLsFetched.Load(), 10)},
		{"URLs Rejected", strconv.FormatInt(cm.URLsRejected.Load(), 10)},
		{"URLs Skipped at Limit", strconv.FormatInt(cm.URLsSkippedAtLimit.Load(), 10)},
		{"URL Duplicates", strconv.FormatInt(cm.URLDuplicates.Load(), 10)},
		{"Blacklisted Websites", strconv.FormatInt(cm.BlacklistedWebsites.Load(), 10)},
		{"Fetch Failures", strconv.FormatInt(cm.FetchFailures.Load(), 10)},
		{"Total Requests", strconv.FormatInt(cm.RequestCount.Load(), 10)},
		{"Pages Parsed", strconv.FormatInt(cm.PagesParsed.Load(), 10)},
		{"Pages Stored", strconv.FormatInt(cm.PagesStored.Load(), 10)},
		{"Skipped Pages", strconv.FormatInt(cm.PagesSkipped.Load(), 10)},
		{"Parsing Failures", strconv.FormatInt(cm.ParsingFailures.Load(), 10)},
		{"Duplicated Pages", strconv.FormatInt(cm.PagesDuplicates.Load(), 10)},
		{"Gigabytes Downloaded", fmt.Sprintf("%.2f GB", float64(cm.BytesDownloaded.Load())*1e-9)},
		{"Still In Flight", strconv.FormatInt(cm.InFlight.Load(), 10)},
		{"DNS Failures", strconv.FormatInt(cm.DNSLookupFailures.Load(), 10)},
		{"Time Elapsed",fmt.Sprintf("%.2f", duration.String())},
		{"Heap Alloc (MB)", strconv.FormatUint(mem.HeapAlloc/1024/1024, 10)},
		{"Heap In-Use (MB)", strconv.FormatUint(mem.HeapInuse/1024/1024, 10)},
		{"Heap Objects", strconv.FormatUint(mem.HeapObjects, 10)},
		{"GC Cycles", strconv.FormatUint(uint64(mem.NumGC), 10)},
	}
}

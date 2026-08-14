package metrics

import (
	"fmt"
	"runtime"
	"sort"
	"strconv"
	"sync"
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

	FetchFailures       atomic.Int64
	PagesParsed         atomic.Int64
	PagesSkipped        atomic.Int64
	PagesStored         atomic.Int64
	PagesDuplicates     atomic.Int64
	BlacklistedWebsites atomic.Int64

	BytesDownloaded atomic.Int64
	InFlight        atomic.Int64

	DNSLookupFailures atomic.Int64
	ParsingFailures   atomic.Int64
	HTTP400Errors     atomic.Int64
	HTTP500Errors     atomic.Int64

	TimeElapsed        atomic.Int64
	RequestCount       atomic.Int64
	HostnameURLFetches atomic.Int64
	IPURLFetches       atomic.Int64

	fetchMu        sync.Mutex
	fetchDurations []time.Duration

	log *zap.Logger
}

type FetchSummary struct {
	Samples            int
	HostnameURLFetches int64
	IPURLFetches       int64
	SmallestFetchTime  time.Duration
	FirstQuartile      time.Duration
	MedianFetchTime    time.Duration
	ThirdQuartile      time.Duration
	LargestFetchTime   time.Duration
	InterquartileRange time.Duration
	AboveMedian        int
	BelowMedian        int
}

type FrontierSummary struct {
	UniqueHosts         int
	PendingURLs         int
	LargestQueue        int
	LargestQueueHost    string
	OldestQueueAge      time.Duration
	OldestQueueHost     string
	MostDispatchedCount int64
	MostDispatchedHost  string
	LockedHosts         int
	CooldownHosts       int
	AvailableHosts      int
}

func NewCrawlMetrics(log *zap.Logger) *CrawlMetrics {
	return &CrawlMetrics{
		log:            log,
		fetchDurations: make([]time.Duration, 0),
	}
}

func (cm *CrawlMetrics) RecordFetch(duration time.Duration, usedIP bool) {
	cm.fetchMu.Lock()
	cm.fetchDurations = append(cm.fetchDurations, duration)
	cm.fetchMu.Unlock()

	if usedIP {
		cm.IPURLFetches.Add(1)
	} else {
		cm.HostnameURLFetches.Add(1)
	}
}

func (cm *CrawlMetrics) FetchSummary() FetchSummary {
	cm.fetchMu.Lock()
	durations := append([]time.Duration(nil), cm.fetchDurations...)
	cm.fetchMu.Unlock()

	if len(durations) == 0 {
		return FetchSummary{}
	}

	sort.Slice(durations, func(i, j int) bool {
		return durations[i] < durations[j]
	})

	median := durations[len(durations)/2]
	if len(durations)%2 == 0 {
		median = (durations[len(durations)/2-1] + durations[len(durations)/2]) / 2
	}

	summary := FetchSummary{
		Samples:           len(durations),
		SmallestFetchTime: durations[0],
		FirstQuartile:     durationMedian(durations[:len(durations)/2]),
		LargestFetchTime:  durations[len(durations)-1],
		MedianFetchTime:   median,
		ThirdQuartile:     durationMedian(durations[(len(durations)+1)/2:]),
	}
	summary.InterquartileRange = summary.ThirdQuartile - summary.FirstQuartile
	for _, fetchTime := range durations {
		if fetchTime < median {
			summary.BelowMedian++
		} else if fetchTime > median {
			summary.AboveMedian++
		}
	}

	return summary
}

func durationMedian(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	middle := len(durations) / 2
	if len(durations)%2 == 1 {
		return durations[middle]
	}

	return (durations[middle-1] + durations[middle]) / 2
}

func (cm *CrawlMetrics) PrintSummary(duration time.Duration, frontier FrontierSummary) {
	cm.log.Info("Crawling Summary")

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	rows := cm.getRows(mem, duration, frontier, cm.FetchSummary())

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

func (cm *CrawlMetrics) getRows(mem runtime.MemStats, duration time.Duration, frontier FrontierSummary, fetch FetchSummary) [][]string {
	return [][]string{
		{"Unique Hosts", strconv.Itoa(frontier.UniqueHosts)},
		{"Pending URLs", strconv.Itoa(frontier.PendingURLs)},
		{"Largest Host Queue", strconv.Itoa(frontier.LargestQueue)},
		{"Largest Queue Host", frontier.LargestQueueHost},
		{"Oldest Queued URL Age", frontier.OldestQueueAge.String()},
		{"Oldest Queue Host", frontier.OldestQueueHost},
		{"Most Dispatched Host Count", strconv.FormatInt(frontier.MostDispatchedCount, 10)},
		{"Most Dispatched Host", frontier.MostDispatchedHost},
		{"Locked Hosts", strconv.Itoa(frontier.LockedHosts)},
		{"Cooldown Hosts", strconv.Itoa(frontier.CooldownHosts)},
		{"Available Hosts", strconv.Itoa(frontier.AvailableHosts)},
		{"URLs Discovered", strconv.FormatInt(cm.URLsDiscovered.Load(), 10)},
		{"URLs Fetched", strconv.FormatInt(cm.URLsFetched.Load(), 10)},
		{"URLs Rejected", strconv.FormatInt(cm.URLsRejected.Load(), 10)},
		{"URLs Skipped at Limit", strconv.FormatInt(cm.URLsSkippedAtLimit.Load(), 10)},
		{"URL Duplicates", strconv.FormatInt(cm.URLDuplicates.Load(), 10)},
		{"Blacklisted Websites", strconv.FormatInt(cm.BlacklistedWebsites.Load(), 10)},
		{"Fetch Failures", strconv.FormatInt(cm.FetchFailures.Load(), 10)},
		{"Total Requests", strconv.FormatInt(cm.RequestCount.Load(), 10)},
		{"Hostname URL Fetches", strconv.FormatInt(cm.HostnameURLFetches.Load(), 10)},
		{"IP URL Fetches", strconv.FormatInt(cm.IPURLFetches.Load(), 10)},
		{"Fetch Time Samples", strconv.Itoa(fetch.Samples)},
		{"Fetch Time Min", fetch.SmallestFetchTime.String()},
		{"Fetch Time Q1", fetch.FirstQuartile.String()},
		{"Median Fetch Time", fetch.MedianFetchTime.String()},
		{"Fetch Time Q3", fetch.ThirdQuartile.String()},
		{"Fetch Time Max", fetch.LargestFetchTime.String()},
		{"Fetch Time IQR", fetch.InterquartileRange.String()},
		{"Fetches Below Median", strconv.Itoa(fetch.BelowMedian)},
		{"Fetches Above Median", strconv.Itoa(fetch.AboveMedian)},
		{"Pages Parsed", strconv.FormatInt(cm.PagesParsed.Load(), 10)},
		{"Pages Stored", strconv.FormatInt(cm.PagesStored.Load(), 10)},
		{"Skipped Pages", strconv.FormatInt(cm.PagesSkipped.Load(), 10)},
		{"Parsing Failures", strconv.FormatInt(cm.ParsingFailures.Load(), 10)},
		{"Duplicated Pages", strconv.FormatInt(cm.PagesDuplicates.Load(), 10)},
		{"Gigabytes Downloaded", fmt.Sprintf("%.2f GB", float64(cm.BytesDownloaded.Load())*1e-9)},
		{"Still In Flight", strconv.FormatInt(cm.InFlight.Load(), 10)},
		{"DNS Failures", strconv.FormatInt(cm.DNSLookupFailures.Load(), 10)},
		{"Time Elapsed", fmt.Sprintf("%s", duration.String())},
		{"Heap Alloc (MB)", strconv.FormatUint(mem.HeapAlloc/1024/1024, 10)},
		{"Heap In-Use (MB)", strconv.FormatUint(mem.HeapInuse/1024/1024, 10)},
		{"Heap Objects", strconv.FormatUint(mem.HeapObjects, 10)},
		{"GC Cycles", strconv.FormatUint(uint64(mem.NumGC), 10)},
	}
}

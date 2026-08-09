package crawler

import (
	"strings"
	"testing"
	"time"
)

func TestFormatCrawlingSummary(t *testing.T) {
	metrics := &CrawlMetrics{}
	metrics.URLsDiscovered.Store(500)
	metrics.PagesStored.Store(498)

	summary := formatCrawlingSummary(metrics, 14*time.Second)

	for _, want := range []string{
		"Crawling Summary",
		"  URLs Discovered:   500",
		"  Pages Stored:      498",
		"  Time Elapsed:      14s",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("expected summary to contain %q, got %q", want, summary)
		}
	}
}

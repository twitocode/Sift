package indexer

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

type IndexerMetrics struct {
	DocumentsRead    atomic.Int64
	DocumentsIndexed atomic.Int64
	DocumentsSkipped atomic.Int64

	BodyTokens    atomic.Int64
	TitleTokens   atomic.Int64
	TotalTokens   atomic.Int64
	UniqueTerms   atomic.Int64
	TotalPostings atomic.Int64
	TitlePostings atomic.Int64

	DocumentsStored atomic.Int64
	Flushes         atomic.Int64
	StoreErrors     atomic.Int64

	TimeElapsed atomic.Int64

	log *zap.Logger
}

func NewIndexerMetrics(log *zap.Logger) *IndexerMetrics {
	return &IndexerMetrics{
		log: log,
	}
}

func (cm *IndexerMetrics) PrintSummary(duration time.Duration) {
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

func getRows(cm *IndexerMetrics, mem runtime.MemStats, duration time.Duration) [][]string {
	return [][]string{
		{"Documents Read", strconv.FormatInt(cm.DocumentsRead.Load(), 10)},
		{"Documents Indexed", strconv.FormatInt(cm.DocumentsIndexed.Load(), 10)},
		{"Body Tokens", strconv.FormatInt(cm.BodyTokens.Load(), 10)},
		{"Title Tokens", strconv.FormatInt(cm.TitleTokens.Load(), 10)},
		{"Total Tokens", strconv.FormatInt(cm.TotalTokens.Load(), 10)},
		{"Unique Terms", strconv.FormatInt(cm.UniqueTerms.Load(), 10)},
		{"Total Postings", strconv.FormatInt(cm.TotalPostings.Load(), 10)},
		{"Title Postings", strconv.FormatInt(cm.TitlePostings.Load(), 10)},
		{"Documents Stored", strconv.FormatInt(cm.DocumentsStored.Load(), 10)},
		{"Flushes", strconv.FormatInt(cm.Flushes.Load(), 10)},
		{"Store Errors", strconv.FormatInt(cm.StoreErrors.Load(), 10)},
		{"Time Elapsed", fmt.Sprintf("%.2f", duration.String())},
	}
}

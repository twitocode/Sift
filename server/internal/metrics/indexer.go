package metrics

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"go.uber.org/zap"
)

type IndexerMetrics struct {
	DocumentsTotal   atomic.Int64
	DocumentsRead    atomic.Int64
	DocumentsIndexed atomic.Int64
	DocumentsSkipped atomic.Int64
	BatchesRead      atomic.Int64
	CurrentBatch     atomic.Int64
	BatchSize        atomic.Int64

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

func (im *IndexerMetrics) PrintSummary(duration time.Duration) {
	im.log.Info("Indexing Summary")

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	rows := im.getRows(mem, duration)

	var (
		purple    = lipgloss.Color("99")
		gray      = lipgloss.Color("245")
		lightGray = lipgloss.Color("241")

		headerStyle  = lipgloss.NewStyle().Foreground(purple).Bold(true).Align(lipgloss.Center)
		cellStyle    = lipgloss.NewStyle().Padding(0, 1)
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
		Rows(rows...)

	// Use explicit CRLF endings so the table starts at column zero even if a
	// preceding terminal UI temporarily disabled newline carriage returns.
	fmt.Fprint(os.Stdout, strings.ReplaceAll(t.String(), "\n", "\r\n"), "\r\n")
}

func (im *IndexerMetrics) getRows(mem runtime.MemStats, duration time.Duration) [][]string {
	return [][]string{
		{"Documents Read", strconv.FormatInt(im.DocumentsRead.Load(), 10)},
		{"Documents Indexed", strconv.FormatInt(im.DocumentsIndexed.Load(), 10)},
		{"Body Tokens", strconv.FormatInt(im.BodyTokens.Load(), 10)},
		{"Title Tokens", strconv.FormatInt(im.TitleTokens.Load(), 10)},
		{"Total Tokens", strconv.FormatInt(im.TotalTokens.Load(), 10)},
		{"Unique Terms", strconv.FormatInt(im.UniqueTerms.Load(), 10)},
		{"Total Postings", strconv.FormatInt(im.TotalPostings.Load(), 10)},
		{"Title Postings", strconv.FormatInt(im.TitlePostings.Load(), 10)},
		{"Documents Stored", strconv.FormatInt(im.DocumentsStored.Load(), 10)},
		{"Flushes", strconv.FormatInt(im.Flushes.Load(), 10)},
		{"Store Errors", strconv.FormatInt(im.StoreErrors.Load(), 10)},
		{"Time Elapsed", fmt.Sprintf("%s", duration.String())}}
}

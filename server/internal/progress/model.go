package progress

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	tea "github.com/charmbracelet/bubbletea"
)

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

type tickMsg struct{}
type doneMsg struct{ err error }

type Model struct {
	title    string
	snapshot func() Snapshot
	current  Snapshot
	started  time.Time
	finished bool
	err      error
	width    int
	height   int
}

func NewModel(title string, snapshot func() Snapshot) Model {
	return Model{
		title:    title,
		snapshot: snapshot,
		current:  snapshot(),
		started:  time.Now(),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(func() tea.Msg { return tickMsg{} }, tick())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case doneMsg:
		m.current = m.snapshot()
		m.finished = true
		m.err = msg.err
		return m, tea.Quit
	case tickMsg:
		m.current = m.snapshot()
		return m, tick()
	}

	return m, nil
}

func (m Model) View() string {
	status := "running"
	if m.finished {
		status = "complete"
		if m.err != nil {
			status = "failed"
		}
	}

	c := m.current.Crawl
	i := m.current.Index
	elapsed := time.Since(m.started).Round(time.Second)
	showCrawl := c.Limit > 0 || c.PagesCrawled > 0 || c.URLsDiscovered > 0 || c.PagesStored > 0
	showIndex := !showCrawl || i.DocumentsTotal > 0 || i.DocumentsIndexed > 0 || i.DocumentsRead > 0

	var b strings.Builder
	b.WriteString(mutedStyle.Render(fmt.Sprintf("%s · %s", status, elapsed)))
	b.WriteString("\n\n")

	if showCrawl {
		b.WriteString(sectionStyle.Render("Crawl progress"))
		b.WriteString("\n")
		b.WriteString(progressBar(c.PagesCrawled, c.Limit, m.width))
		b.WriteString(fmt.Sprintf("  %d / %d pages\n\n", c.PagesCrawled, c.Limit))

		b.WriteString(panel("Crawl activity",
			metric("URLs discovered", c.URLsDiscovered),
			metric("URLs fetched", c.URLsFetched),
			metric("Pages stored", c.PagesStored),
			metric("Fetch failures", c.FetchFailures),
			metric("In flight", c.InFlight),
		))
		b.WriteString("\n")
		b.WriteString(panel("Frontier",
			metric("Pending URLs", c.PendingURLs),
			metric("Unique hosts", c.UniqueHosts),
			metric("Available hosts", c.AvailableHosts),
			metric("Locked hosts", c.LockedHosts),
			metric("Cooldown hosts", c.CooldownHosts),
		))
		b.WriteString("\n")
	}

	if showIndex {
		b.WriteString(sectionStyle.Render("Index progress"))
		b.WriteString("\n")
		if i.DocumentsTotal == 0 && !m.finished {
			b.WriteString("loading pages from sqlite...\n")
		} else {
			b.WriteString(progressBar(int(i.DocumentsIndexed), int(i.DocumentsTotal), m.width))
			b.WriteString(fmt.Sprintf("  %d / %d pages indexed\n", i.DocumentsIndexed, i.DocumentsTotal))
		}
		b.WriteString(mutedStyle.Render(fmt.Sprintf(
			"Loaded: %d   Batch: %d   Batch size: %d   Terms: %d   Tokens: %d   Stored: %d   Flushes: %d",
			i.DocumentsRead, i.CurrentBatch, i.BatchSize, i.UniqueTerms, i.TotalTokens, i.DocumentsStored, i.Flushes,
		)))
		b.WriteString("\n\n")
	}

	b.WriteString(mutedStyle.Render("ctrl+c quit"))
	if m.err != nil {
		b.WriteString(fmt.Sprintf("\n%s", errorStyle.Render(m.err.Error())))
	}
	return b.String()
}

func tick() tea.Cmd {
	return tea.Tick(400*time.Millisecond, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

func progressBar(value, total, width int) string {
	if width < 40 {
		width = 40
	}
	width -= 10
	if width > 70 {
		width = 70
	}
	if total <= 0 {
		return "[" + strings.Repeat("░", width) + "]"
	}
	filled := value * width / total
	if filled > width {
		filled = width
	}
	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", width-filled) + "]"
}

func metric(label string, value any) string {
	return fmt.Sprintf("%-18s %v", label, value)
}

func panel(title string, rows ...string) string {
	content := titleStyle.Render(title) + "\n" + strings.Join(rows, "\n")
	return panelStyle.Render(content)
}

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("220"))
	sectionStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("220"))
	mutedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	panelStyle   = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(0, 1).Width(42)
)

package progress

import (
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/x/term"
	"github.com/pterm/pterm"
)

func Run(title string, snapshot func() Snapshot, done <-chan error) error {
	if !term.IsTerminal(os.Stderr.Fd()) {
		return waitPlain(snapshot, done)
	}

	completion := make(chan error, 1)
	go func() {
		completion <- <-done
	}()

	total := 0
	for total <= 0 {
		total = int(snapshot().Index.DocumentsTotal)
		if total > 0 {
			break
		}

		select {
		case err := <-completion:
			return err
		case <-time.After(50 * time.Millisecond):
		}
	}

	bar := pterm.DefaultProgressbar.
		WithTitle(title).
		WithTotal(total).
		WithWriter(os.Stderr).
		WithShowCount().
		WithShowPercentage().
		WithShowElapsedTime()

	startedBar, err := bar.Start()
	if err != nil {
		return err
	}

	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()

	lastProcessed := snapshot().Index.DocumentsRead
	startedBar.UpdateTitle(formatTitle(title, snapshot()))

	for {
		select {
		case err := <-completion:
			current := snapshot()
			if current.Index.DocumentsRead > lastProcessed {
				startedBar.Add(int(current.Index.DocumentsRead - lastProcessed))
			}

			remaining := startedBar.Total - startedBar.Current
			if remaining > 0 {
				startedBar.Add(remaining)
			}

			status := "complete"
			if err != nil {
				status = "failed"
			}
			startedBar.UpdateTitle(formatTitle(title+" "+status, current))

			_, stopErr := startedBar.Stop()
			if err != nil {
				return err
			}
			return stopErr

		case <-ticker.C:
			current := snapshot()
			startedBar.UpdateTitle(formatTitle(title, current))
			if current.Index.DocumentsRead > lastProcessed {
				startedBar.Add(int(current.Index.DocumentsRead - lastProcessed))
				lastProcessed = current.Index.DocumentsRead
			}
		}
	}
}

func formatTitle(title string, snapshot Snapshot) string {
	index := snapshot.Index
	return fmt.Sprintf(
		"%s  loaded=%d  batch=%d  size=%d  terms=%d  tokens=%d  stored=%d  flushes=%d",
		title,
		index.DocumentsRead,
		index.CurrentBatch,
		index.BatchSize,
		index.UniqueTerms,
		index.TotalTokens,
		index.DocumentsStored,
		index.Flushes,
	)
}

func waitPlain(snapshot func() Snapshot, done <-chan error) error {
	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()

	printSnapshot := func() {
		s := snapshot()
		fmt.Fprintf(
			os.Stderr,
			"\r[%d/%d indexed] loaded=%d terms=%d tokens=%d stored=%d",
			s.Index.DocumentsIndexed,
			s.Index.DocumentsTotal,
			s.Index.DocumentsRead,
			s.Index.UniqueTerms,
			s.Index.TotalTokens,
			s.Index.DocumentsStored,
		)
	}

	printSnapshot()
	for {
		select {
		case err := <-done:
			printSnapshot()
			fmt.Fprintln(os.Stderr)
			return err
		case <-ticker.C:
			printSnapshot()
		}
	}
}

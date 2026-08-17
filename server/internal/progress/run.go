package progress

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"
)

func Run(title string, snapshot func() Snapshot, done <-chan error) error {
	if !term.IsTerminal(os.Stdout.Fd()) {
		return runPlain(snapshot, done)
	}

	p := tea.NewProgram(NewModel(title, snapshot), programOptions()...)
	go func() {
		p.Send(doneMsg{err: <-done})
	}()

	_, err := p.Run()

	if err != nil {
		return runPlain(snapshot, done)
	}
	fmt.Fprint(os.Stdout, "\r\n")
	return nil
}

func programOptions() []tea.ProgramOption {
	return []tea.ProgramOption{
		tea.WithOutput(os.Stdout),
		tea.WithAltScreen(),
		// This progress display does not need interactive input. Leaving input
		// disabled prevents Bubble Tea from putting the TTY in raw mode.
		tea.WithInput(nil),
	}
}

func runPlain(snapshot func() Snapshot, done <-chan error) error {
	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()

	printSnap := func(finished bool) {
		s := snapshot()
		status := "running"
		if finished {
			status = "complete"
		}
		if s.Index.DocumentsTotal == 0 && !finished {
			fmt.Fprintf(os.Stderr, "\r[%s] loading pages from sqlite...   ", status)
			return
		}
		fmt.Fprintf(os.Stderr, "\r[%s] %d/%d indexed  loaded=%d batch=%d size=%d terms=%d tokens=%d stored=%d   ",
			status,
			s.Index.DocumentsIndexed,
			s.Index.DocumentsTotal,
			s.Index.DocumentsRead,
			s.Index.CurrentBatch,
			s.Index.BatchSize,
			s.Index.UniqueTerms,
			s.Index.TotalTokens,
			s.Index.DocumentsStored,
		)
	}

	printSnap(false)
	for {
		select {
		case err := <-done:
			printSnap(true)
			fmt.Fprintln(os.Stderr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "index failed: %v\n", err)
				return err
			}
			return nil
		case <-ticker.C:
			printSnap(false)
		}
	}
}

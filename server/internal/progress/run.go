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
	return nil
}

func programOptions() []tea.ProgramOption {
	opts := []tea.ProgramOption{tea.WithOutput(os.Stdout)}
	if !term.IsTerminal(os.Stdin.Fd()) {
		opts = append(opts, tea.WithInput(nil))
	}
	return opts
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
		if s.Index.DocumentsRead == 0 && !finished {
			fmt.Fprintf(os.Stderr, "\r[%s] loading pages from sqlite...   ", status)
			return
		}
		fmt.Fprintf(os.Stderr, "\r[%s] %d/%d pages indexed  terms=%d tokens=%d stored=%d   ",
			status,
			s.Index.DocumentsIndexed,
			s.Index.DocumentsRead,
			s.Index.UniqueTerms,
			s.Index.TotalTokens,
			s.Index.DocumentsStored,
		)
	}

	printSnap(false)
	for {
		select {
		case <-done:
			printSnap(true)
			fmt.Fprintln(os.Stderr)
			return nil
		case <-ticker.C:
			printSnap(false)
		}
	}
}

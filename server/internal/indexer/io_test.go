package indexer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/twitocode/sift/internal/common"
)

func TestDumpAndLoadIndexTermContainingSpaces(t *testing.T) {
	previousIndexDir := indexDir
	indexDir = t.TempDir()
	t.Cleanup(func() {
		indexDir = previousIndexDir
	})

	term := "github official mcp server"
	index := map[string][]common.Posting{
		term: {{PageID: 42, TitleFrequency: 1}},
	}

	if err := DumpIndex(&common.IndexStats{}, index); err != nil {
		t.Fatalf("DumpIndex() error = %v", err)
	}

	terms := LoadTerms()
	got, ok := terms[term]
	if !ok {
		t.Fatalf("LoadTerms() = %v, want term %q", terms, term)
	}
	if got.Count != 1 || got.ByteOffset != 0 {
		t.Fatalf("LoadTerms()[%q] = %+v, want count 1 and offset 0", term, got)
	}
}

func TestLoadTermsRejectsIndexWithoutCurrentFormatMetadata(t *testing.T) {
	previousIndexDir := indexDir
	indexDir = t.TempDir()
	t.Cleanup(func() {
		indexDir = previousIndexDir
	})

	if err := os.WriteFile(
		filepath.Join(indexDir, "terms.dat"),
		[]byte("github 0 1\n"),
		0o644,
	); err != nil {
		t.Fatalf("write legacy terms: %v", err)
	}

	if terms := LoadTerms(); len(terms) != 0 {
		t.Fatalf("LoadTerms() = %v, want incompatible index rejected", terms)
	}
}

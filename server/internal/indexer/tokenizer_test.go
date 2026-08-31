package indexer

import (
	"slices"
	"testing"
)

func TestTokenizeDoesNotSplitPercentEncodedSpaces(t *testing.T) {
	got := Tokenize("in%20another%20dimension")
	want := []string{"in", "anoth", "dimens"}

	if !slices.Equal(got, want) {
		t.Fatalf("Tokenize() = %v, want %v", got, want)
	}
}

func TestTokenizeKeepsContractionsTogether(t *testing.T) {
	got := Tokenize("don't")
	want := []string{"don't"}

	if !slices.Equal(got, want) {
		t.Fatalf("Tokenize() = %v, want %v", got, want)
	}
}

func TestTokenizeKeepsDecimalsTogether(t *testing.T) {
	got := Tokenize("pi is 3.14")
	if !slices.Contains(got, "3.14") {
		t.Fatalf("Tokenize() = %v, want to contain 3.14", got)
	}
	if slices.Contains(got, "3") || slices.Contains(got, "14") {
		t.Fatalf("Tokenize() = %v, should not split 3.14", got)
	}
}

func TestTokenizeKeepsVersionNumbersTogether(t *testing.T) {
	got := Tokenize("1.2.3")
	want := []string{"1.2.3"}

	if !slices.Equal(got, want) {
		t.Fatalf("Tokenize() = %v, want %v", got, want)
	}
}

func TestTokenizeKeepsHyphenatedPhrase(t *testing.T) {
	got := Tokenize("state-of-the-art")
	for _, want := range []string{"state-of-the-art", "state", "of", "the", "art"} {
		if !slices.Contains(got, want) {
			t.Fatalf("Tokenize() = %v, want to contain %q", got, want)
		}
	}
	if slices.Contains(got, "stateoftheart") {
		t.Fatalf("Tokenize() = %v, should keep original hyphens", got)
	}
}

func TestTokenizeKeepsHyphenatedTerm(t *testing.T) {
	got := Tokenize("covid-19")
	for _, want := range []string{"covid-19", "covid", "19"} {
		if !slices.Contains(got, want) {
			t.Fatalf("Tokenize() = %v, want to contain %q", got, want)
		}
	}
}

func TestTokenizeKeepsUnderscoreTermAndParts(t *testing.T) {
	got := Tokenize("b_b")
	for _, want := range []string{"b_b", "b"} {
		if !slices.Contains(got, want) {
			t.Fatalf("Tokenize() = %v, want to contain %q", got, want)
		}
	}
}

func TestTokenizeKeepsDomainWithTLD(t *testing.T) {
	got := Tokenize("github.com")
	want := []string{"github.com"}

	if !slices.Equal(got, want) {
		t.Fatalf("Tokenize() = %v, want %v", got, want)
	}
}

func TestTokenizeKeepsSubdomainWithTLD(t *testing.T) {
	got := Tokenize("docs.github.com")
	want := []string{"docs.github.com"}

	if !slices.Equal(got, want) {
		t.Fatalf("Tokenize() = %v, want %v", got, want)
	}
}

func TestTokenizeKeepsProgrammingTokens(t *testing.T) {
	got := Tokenize("C++ and C#")
	for _, want := range []string{"c++", "c#"} {
		if !slices.Contains(got, want) {
			t.Fatalf("Tokenize() = %v, want to contain %q", got, want)
		}
	}
}

func TestTokenizeSplitsPlusBetweenWords(t *testing.T) {
	got := Tokenize("foo+bar")
	for _, want := range []string{"foo", "bar"} {
		if !slices.Contains(got, want) {
			t.Fatalf("Tokenize() = %v, want to contain %q", got, want)
		}
	}
	if slices.Contains(got, "foo+bar") {
		t.Fatalf("Tokenize() = %v, should split plus between words", got)
	}
}

func TestTokenizeSplitsCamelCaseAndKeepsWholeWord(t *testing.T) {
	got := Tokenize("GitHub")
	for _, want := range []string{"github", "git", "hub"} {
		if !slices.Contains(got, want) {
			t.Fatalf("Tokenize() = %v, want to contain %q", got, want)
		}
	}
}

func TestTokenizeQueryIncludesOriginalAndParts(t *testing.T) {
	got := TokenizeQuery("Hello World")
	for _, want := range []string{"hello world", "hello", "world"} {
		if !slices.Contains(got, want) {
			t.Fatalf("TokenizeQuery() = %v, want to contain %q", got, want)
		}
	}
}

func TestTokenizeQueryDoesNotDuplicateSingleToken(t *testing.T) {
	got := TokenizeQuery("github.com")
	count := 0
	for _, token := range got {
		if token == "github.com" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("TokenizeQuery() = %v, want github.com once", got)
	}
}

func TestTokenizeQueryNormalizesWhitespaceInOriginal(t *testing.T) {
	got := TokenizeQuery("  Hello\n\tWorld  ")
	if !slices.Contains(got, "hello world") {
		t.Fatalf("TokenizeQuery() = %v, want normalized original query", got)
	}
	if slices.Contains(got, "hello\n\tworld") {
		t.Fatalf("TokenizeQuery() = %v, should not contain raw whitespace", got)
	}
}

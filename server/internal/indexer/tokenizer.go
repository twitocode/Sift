package indexer

import (
	"strings"
	"unicode"

	"github.com/kljensen/snowball"
)

func isApostrophe(r rune) bool {
	return r == '\'' || r == '\u2019' || r == '\u2018'
}

func Tokenize(raw string) []string {
	raw = strings.ToLower(raw)
	raw = strings.Map(func(r rune) rune {
    //sometimes you may get weird web apostrophes so i just convert them to ascii
		if r == '\u2019' || r == '\u2018' {
			return '\''
		}
		return r
	}, raw)

	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r) && r != '\''
	})
	out := make([]string, 0, len(parts))

	for _, p := range parts {
		p = strings.Trim(p, "'")
		if p == "" {
			continue
		}
		out = append(out, p)
	}

	Normalize(out)
	return out
}

func Normalize(tokens []string) {
	for i, token := range tokens {
		stemmed, err := snowball.Stem(token, "english", true)

		if err == nil {
			tokens[i] = stemmed
		}
	}
}

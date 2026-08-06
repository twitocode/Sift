package indexer

import (
	"strings"
	"unicode"

	"github.com/kljensen/snowball"
)

func Tokenize(raw string) []string {
	var out []string

	raw = strings.ToLower(raw)

	out = strings.FieldsFunc(raw, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
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

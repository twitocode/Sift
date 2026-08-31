package indexer

import (
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/kljensen/snowball"
	"golang.org/x/net/publicsuffix"
)

func isApostrophe(r rune) bool {
	return r == '\'' || r == '\u2019' || r == '\u2018'
}

func isHex(r byte) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

func decodePercentEncoding(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) && isHex(s[i+1]) && isHex(s[i+2]) {
			hi := unhex(s[i+1])
			lo := unhex(s[i+2])
			b.WriteByte((hi << 4) | lo)
			i += 2
			continue
		}
		b.WriteByte(s[i])
	}

	return b.String()
}

func unhex(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	default:
		return c - 'A' + 10
	}
}

func isTokenChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r) || isApostrophe(r)
}

func isConnector(r rune) bool {
	return r == '.' || r == '-' || r == '_' || r == '@'
}

func isAttachedSymbol(r rune) bool {
	return r == '+' || r == '#'
}

func Tokenize(raw string) []string {
	raw = decodePercentEncoding(raw)
	raw = strings.Map(func(r rune) rune {
		if isApostrophe(r) {
			return '\''
		}
		return r
	}, raw)

	parts := extractRawTokens(raw)
	out := make([]string, 0, len(parts)*2)
	seen := make(map[string]struct{}, len(parts)*2)

	add := func(token string) {
		token = strings.Trim(token, "'")
		if token == "" {
			return
		}
		if _, ok := seen[token]; ok {
			return
		}
		seen[token] = struct{}{}
		out = append(out, token)
	}

	for _, part := range parts {
		for _, token := range expandToken(part) {
			add(token)
		}
	}

	Normalize(out)
	return out
}

func TokenizeQuery(raw string) []string {
	tokens := Tokenize(raw)
	original := canonicalQuery(raw)
	if original == "" {
		return tokens
	}
	for _, token := range tokens {
		if token == original {
			return tokens
		}
	}
	return append([]string{original}, tokens...)
}

func TokenizeURL(raw string) (domainTokens, pathTokens []string) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, Tokenize(raw)
	}

	host := strings.ToLower(parsed.Hostname())
	if host != "" {
		domainTokens = append(domainTokens, host)

		suffix, _ := publicsuffix.PublicSuffix(host)
		name := strings.TrimSuffix(host, "."+suffix)
		for _, label := range strings.Split(name, ".") {
			if label != "" {
				domainTokens = append(domainTokens, label)
			}
		}
	}

	pathTokens = Tokenize(strings.Join([]string{
		parsed.Path,
		parsed.RawQuery,
		parsed.Fragment,
	}, " "))
	return domainTokens, pathTokens
}

func canonicalQuery(raw string) string {
	raw = decodePercentEncoding(raw)
	raw = strings.Map(func(r rune) rune {
		if isApostrophe(r) {
			return '\''
		}
		return r
	}, raw)
	return strings.ToLower(strings.Join(strings.Fields(raw), " "))
}

func extractRawTokens(raw string) []string {
	runes := []rune(raw)
	var out []string

	for i := 0; i < len(runes); {
		r := runes[i]
		if !isTokenChar(r) {
			i++
			continue
		}

		start := i
		i++
		for i < len(runes) {
			cur := runes[i]
			if isTokenChar(cur) {
				i++
				continue
			}
			if isAttachedSymbol(cur) {
				j := i
				for j < len(runes) && isAttachedSymbol(runes[j]) {
					j++
				}
				if j == len(runes) || !isTokenChar(runes[j]) {
					i = j
					continue
				}
				break
			}
			if isConnector(cur) && i+1 < len(runes) && isTokenChar(runes[i+1]) {
				i++
				continue
			}
			break
		}

		out = append(out, string(runes[start:i]))
	}

	return out
}

func expandToken(raw string) []string {
	lower := strings.ToLower(raw)
	if isNumericToken(raw) || isDomainToken(lower) {
		return []string{lower}
	}

	out := []string{lower}
	if strings.ContainsAny(raw, "-_") {
		for _, part := range strings.FieldsFunc(lower, func(r rune) bool {
			return r == '-' || r == '_'
		}) {
			if part != "" {
				out = append(out, part)
			}
		}
	}
	for _, part := range splitCamelCase(raw) {
		out = append(out, strings.ToLower(part))
	}
	return out
}

func isDomainToken(s string) bool {
	labels := strings.Split(s, ".")
	if len(labels) < 2 {
		return false
	}
	if _, ok := tlds[labels[len(labels)-1]]; !ok {
		return false
	}
	for _, label := range labels {
		if label == "" {
			return false
		}
		for _, r := range label {
			if !unicode.IsLetter(r) && !unicode.IsNumber(r) && r != '-' {
				return false
			}
		}
	}
	return true
}

func isNumericToken(s string) bool {
	hasDigit := false
	for _, r := range s {
		switch {
		case unicode.IsNumber(r):
			hasDigit = true
		case r == '.' || r == ',':
			continue
		default:
			return false
		}
	}
	return hasDigit
}

func splitCamelCase(s string) []string {
	if !utf8.ValidString(s) {
		return nil
	}

	runes := []rune(s)
	if len(runes) < 2 {
		return nil
	}

	hasLower := false
	hasUpper := false
	for _, r := range runes {
		if !unicode.IsLetter(r) {
			return nil
		}
		if unicode.IsLower(r) {
			hasLower = true
		}
		if unicode.IsUpper(r) {
			hasUpper = true
		}
	}
	if !hasLower || !hasUpper {
		return nil
	}

	var parts []string
	start := 0
	for i := 1; i < len(runes); i++ {
		prev := runes[i-1]
		cur := runes[i]
		split := false

		if unicode.IsLower(prev) && unicode.IsUpper(cur) {
			split = true
		}
		if unicode.IsUpper(prev) && unicode.IsUpper(cur) && i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
			split = true
		}

		if split {
			parts = append(parts, string(runes[start:i]))
			start = i
		}
	}
	parts = append(parts, string(runes[start:]))
	if len(parts) < 2 {
		return nil
	}
	return parts
}

func Normalize(tokens []string) {
	for i, token := range tokens {
		if !shouldStem(token) {
			continue
		}
		stemmed, err := snowball.Stem(token, "english", true)
		if err == nil {
			tokens[i] = stemmed
		}
	}
}

func shouldStem(token string) bool {
	for _, r := range token {
		if unicode.IsNumber(r) || isAttachedSymbol(r) || r == '.' || r == '-' || r == '_' {
			return false
		}
	}
	return true
}

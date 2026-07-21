package crawler

import (
	"errors"
	"net/url"
	"strings"

	"golang.org/x/net/publicsuffix"
)

type URL string

func (u URL) normalizeString() URL {
	s := strings.ToLower(string(u))
	s = strings.Split(s, "?")[0]
	s = strings.Split(s, "#")[0]
	return URL(s)
}

func (u URL) GetDomain() (string, error) {
	parsed, err := url.Parse(string(u))
	if err != nil {
		return "", err
	}

	return publicsuffix.EffectiveTLDPlusOne(parsed.Hostname())
}

func (u URL) String() string {
	return string(u)
}

func (u URL) GetHost() (URL, error) {
	parsed, err := url.Parse(string(u))
	if err != nil {
		return "", err
	}

	return URL(parsed.Hostname()), nil
}

func (u URL) ResolveUrl(base URL) (URL, error) {
	baseUrl, _ := url.Parse(string(base))
	parsed, err := url.Parse(string(u))
	if err != nil {
		return "", errors.New("Invalid link")
	}
	resolved := baseUrl.ResolveReference(parsed)

	if resolved.Scheme != "http" && resolved.Scheme != "https" {
		return "", errors.New("Invalid link")
	}
	if resolved.Hostname() == "" {
		return "", errors.New("Invalid link")
	}

	return URL(resolved.String()), nil
}

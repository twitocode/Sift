package crawler

import (
	"net/url"

	"golang.org/x/net/publicsuffix"
)

func GetDomain(href string) (string, error) {
	u, err := url.Parse(href)
	if err != nil {
		return "", err
	}

	return publicsuffix.EffectiveTLDPlusOne(u.Hostname())
}

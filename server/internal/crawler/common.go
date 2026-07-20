package crawler

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"golang.org/x/net/publicsuffix"
)

func GetDomain(href string) (string, error) {
	u, err := url.Parse(href)
	if err != nil {
		return "", err
	}

	return publicsuffix.EffectiveTLDPlusOne(u.Hostname())
}

func ResolveUrl(newUrl string, host string) (string, error) {
	hostUrl, _ := url.Parse(host)
	parsed, err := url.Parse(newUrl)
	if err != nil {
		return "", errors.New("Invalid link")
	}
	resolved := hostUrl.ResolveReference(parsed)

	if resolved.Scheme != "http" && resolved.Scheme != "https" {
		return "", errors.New("Invalid link")
	}
	if resolved.Hostname() == "" {
		return "", errors.New("Invalid link")
	}

	return resolved.String(), nil
}

func GetHost(href string) (string, error) {
	u, err := url.Parse(href)
	if err != nil {
		return "", err
	}

	return u.Hostname(), nil

}

func SaveResultsToFile(links []string) error {
	if len(links) == 0 {
		return nil
	}

	content := strings.Join(links, "\n") + "\n"

	err := os.WriteFile("result.txt", []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("failed to write results to file: %w", err)
	}

	fmt.Println("Wrote results to file")
	return nil
}

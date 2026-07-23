package crawler

import (
	"net/http"
	"time"
)

func newHttpClient(dialerContext DialerContext) *http.Client {
	transport := &http.Transport{
		DialContext:         dialerContext,
		MaxIdleConns:        300,
		MaxIdleConnsPerHost: 50,
		IdleConnTimeout:     90 * time.Second,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
	}
}

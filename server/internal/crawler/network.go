package crawler

import (
	"net/http"
	"time"
)

func newHttpClient(dialerContext DialerContext) *http.Client {
	transport := &http.Transport{
		DialContext: dialerContext,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
	}
}

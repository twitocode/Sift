package networking

import (
	"net/http"
	"time"
)

func NewHttpClient(dialerContext DialerContext, spiderCount int) *http.Client {
	maxIdle := spiderCount
	if maxIdle < 64 {
		maxIdle = 64
	}

	transport := &http.Transport{
		DialContext:         dialerContext,
		MaxIdleConns:        maxIdle,
		MaxIdleConnsPerHost: 2,
		MaxConnsPerHost:     2,
		IdleConnTimeout:     90 * time.Second,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
	}
}

package crawler

import (
	"context"
	"errors"
	"time"

	"github.com/twitocode/sift/internal/common"
	"go.uber.org/zap"
)

type FrontierStore struct {
	bufferQueues *common.SafeMap[URL, *BQueue]
	readyQueue   chan Payload
	dispatched   *common.SafeMap[URL, struct{}]
	bloomFilter  *BloomFilter
	crawledURLs  *common.SafeMap[URL, struct{}]
	dnsCache     *DNSCache
	metrics      *CrawlMetrics
	cfg          *common.Config

	blacklist map[string]struct{}
	workers   int
	log       *zap.Logger
}

func NewFrontierStore(log *zap.Logger, dnsCache *DNSCache, cfg *common.Config, metrics *CrawlMetrics) *FrontierStore {
	return &FrontierStore{
		bufferQueues: common.NewSafeMap[URL, *BQueue](),
		readyQueue:   make(chan Payload, cfg.SpiderCount),
		dispatched:   common.NewSafeMap[URL, struct{}](),
		crawledURLs:  common.NewSafeMap[URL, struct{}](),
		bloomFilter:  NewBloomFilter(float64(cfg.CrawlCount*2), 0.01),
		dnsCache:     dnsCache,
		workers:      cfg.SpiderCount,
		log:          log,
		blacklist:    GenerateBlacklistMap(DefaultBlacklistedDomains),

		metrics: metrics,
		cfg:     cfg,
	}
}

func (fs *FrontierStore) AddUrl(ctx context.Context, rawUrl URL) {
	hostname, err := rawUrl.GetHost()

	if err != nil {
		fs.log.Warn("Invalid url given", zap.String("url", rawUrl.String()))
		return
	}

	domain, _ := rawUrl.GetDomain()
	if IsDomainBlacklisted(domain, fs.blacklist) {
		fs.metrics.BlacklistedWebsites.Add(1)
    return
	}

	if !fs.bufferQueues.Contains(hostname) && fs.bufferQueues.Length() < fs.cfg.MaxHostQueues {
		//fs.log.Debug("BQueue cache miss", zap.String("host", hostname.String()))
		queue := &BQueue{
			Host:       hostname,
			URLs:       []URL{},
			Locked:     false,
			StaleUntil: time.Now(),
		}

		fs.bufferQueues.Set(hostname, queue)
	}

}

func (fs *FrontierStore) TryDispatchJob(ctx context.Context, job *Payload) {
	bQueue, exists := fs.bufferQueues.Get(job.host)
	if exists {
		select {
		case <-ctx.Done():
			return
		case fs.readyQueue <- *job:
			bQueue.Lock()
			fs.dispatched.Set(job.url, struct{}{})
		}
	}
}

func (fs *FrontierStore) HasLinkBeenCrawled(link URL) bool {
	if fs.bloomFilter.ProbablyContains(link) {
		return fs.crawledURLs.Contains(link)
	}

	return false
}

func (fs *FrontierStore) ProcessLink(ctx context.Context, link URL) {
	sanitizedURL, err := fs.SanitizeURL(link)
	if err != nil {
		fs.log.Debug("Website not allowed", zap.String("url", link.String()))

		fs.metrics.URLsRejected.Add(1)
		return
	}

	hostname, err := sanitizedURL.GetHost()
	if err != nil {
		fs.log.Warn("Invalid url given (linkReceiveChan)", zap.String("url", sanitizedURL.String()))
		return
	}

	bQueue, exists := fs.bufferQueues.Get(hostname)
	if !exists {
		return
	}

	dispatchedAlready := fs.dispatched.Contains(sanitizedURL)
	hostQueueAvailable := bQueue.IsAvailable()
	readyQueueAvailable := len(fs.readyQueue) < fs.workers

	if !dispatchedAlready && hostQueueAvailable && readyQueueAvailable {
		payload := Payload{
			url:           sanitizedURL,
			host:          hostname,
			dialerContext: fs.dnsCache.DialContext,
		}

		select {
		case <-ctx.Done():
			return
		case fs.readyQueue <- payload:
			fs.dispatched.Set(sanitizedURL, struct{}{})
			bQueue.Lock()
		}
		return
	} else {
		//fs.log.Debug("Adding back to buffer", zap.String("host", hostname.String()), zap.String("url", link.String()))
		if !bQueue.Enqueue(sanitizedURL, fs.cfg.MaxURLsPerHost) {
			fs.metrics.URLsSkippedAtLimit.Add(1)
		}
	}
}

func (fs *FrontierStore) ProcessPage(ctx context.Context, page *Page) error {
	if !fs.IsLinkDispatched(page.URL) {
		return errors.New("Link for some reason not available")
	}

	fs.bloomFilter.Insert(page.URL)
	fs.crawledURLs.Set(page.URL, struct{}{})
	fs.dispatched.Delete(page.URL)

	bQueue, exists := fs.bufferQueues.Get(page.Host)
	if !exists {
		fs.log.Fatal("Buffer queue should exist but doesn't", zap.String("host", page.Host.String()))
	}

	bQueue.Timeout(4000)
	allowedURLs := make([]URL, 0)

	for _, link := range page.Links {
		if fs.HasLinkBeenCrawled(link) {
			fs.metrics.URLDuplicates.Add(1)
			fs.log.Debug("Duplicate link crawled", zap.String("url", page.URL.String()))
			continue
		}

		allowedURLs = append(allowedURLs, link)
	}

	page.Links = allowedURLs
	return nil
}

func (fs *FrontierStore) Shutdown() {
	//close(fs.readyQueue)

}

// used for timer
func (fs *FrontierStore) FindAvailableJob() (*Payload, error) {
	var job *Payload

	fs.bufferQueues.Range(func(host URL, queue *BQueue) bool {
		if queue.IsAvailable() {
			url := queue.Dequeue()
			if url == "" {
				return true
			}

			job = &Payload{
				url:           url,
				host:          host,
				dialerContext: fs.dnsCache.DialContext,
			}

			queue.Lock()
			return false
		}

		return true
	})

	return job, nil
}

func (fs *FrontierStore) HandleSpiderFail(payload Payload) {
	fs.dispatched.Delete(payload.url)

	bQueue, exists := fs.bufferQueues.Get(payload.host)
	if exists {
		bQueue.TryUnlock()
	}
}

func (fs *FrontierStore) IsLinkAvailable(link URL) bool {
	//bloom filter and stuff
	return !fs.HasLinkBeenCrawled(link) && !fs.dispatched.Contains(link)
}

func (fs *FrontierStore) IsLinkDispatched(link URL) bool {
	//bloom filter and stuff
	return !fs.HasLinkBeenCrawled(link) && fs.dispatched.Contains(link)
}

func (fs *FrontierStore) SanitizeURL(link URL) (URL, error) {
	if fs.bloomFilter.ProbablyContains(link) {
		if fs.crawledURLs.Contains(link) {
			return "", errors.New("Link already crawled")
		}
	}

	return link.normalizeString(), nil
}

func (fs *FrontierStore) FreeHosts() {
	fs.bufferQueues.Range(func(url URL, bQueue *BQueue) bool {
		bQueue.mu.Lock()
		defer bQueue.mu.Unlock()

		if bQueue.Locked && time.Now().After(bQueue.StaleUntil) {
			bQueue.Locked = false
		}

		return true
	})
}

func (fs *FrontierStore) GetBufferCount() int {
	return fs.bufferQueues.Length()
}

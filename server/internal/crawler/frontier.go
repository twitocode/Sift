package crawler

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/twitocode/sift/internal/common"
	"go.uber.org/zap"
)

type FrontierStore struct {
	bufferQueues *common.SafeMap[URL, *BQueue]
	readyQueue   chan Payload
	seenURLs     *common.SafeMap[URL, struct{}]
	bloomFilter  *BloomFilter
	crawledURLs  *common.SafeMap[URL, struct{}]
	dnsCache     *DNSCache
	metrics      *CrawlMetrics
	cfg          *common.Config

	blacklist map[string]struct{}
	workers   int
	log       *zap.Logger

	pending atomic.Int32

	hostMu sync.Mutex
}

type FrontierStats struct {
	HostQueues   int
	PendingURLs  int
	LargestQueue int
}

func NewFrontierStore(log *zap.Logger, dnsCache *DNSCache, cfg *common.Config, metrics *CrawlMetrics) *FrontierStore {
	return &FrontierStore{
		bufferQueues: common.NewSafeMap[URL, *BQueue](),
		readyQueue:   make(chan Payload, cfg.SpiderCount),
		seenURLs:     common.NewSafeMap[URL, struct{}](),
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

func (fs *FrontierStore) Stats() FrontierStats {
	stats := FrontierStats{}

	fs.bufferQueues.Range(func(_ URL, queue *BQueue) bool {
		stats.HostQueues++
		pending := queue.Len()
		stats.PendingURLs += pending
		if pending > stats.LargestQueue {
			stats.LargestQueue = pending
		}

		return true
	})

	return stats
}

func (fs *FrontierStore) AddHost(ctx context.Context, newUrlToRequest URL) {
	hostname, err := newUrlToRequest.GetHost()

	if err != nil {
		fs.log.Warn("Invalid url given", zap.String("url", newUrlToRequest.String()))
		return
	}

	domain, _ := newUrlToRequest.GetDomain()
	if IsDomainBlacklisted(domain, fs.blacklist) {
		fs.metrics.BlacklistedWebsites.Add(1)
		return
	}

	if fs.bufferQueues.Contains(hostname) {
		return
	}

	fs.hostMu.Lock()
	defer fs.hostMu.Unlock()

	if fs.bufferQueues.Contains(hostname) || fs.bufferQueues.Length() >= fs.cfg.MaxHostQueues {
		return
	}

	queue := &BQueue{
		Host:       hostname,
		URLs:       []URL{},
		Locked:     false,
		StaleUntil: time.Now(),
	}

	fs.bufferQueues.Set(hostname, queue)
}

func (fs *FrontierStore) TryDispatchJob(ctx context.Context, job *Payload) {
	_, exists := fs.bufferQueues.Get(job.host)
	if exists {
		select {
		case <-ctx.Done():
			return
		case fs.readyQueue <- *job:
		}
	}
}

func (fs *FrontierStore) HasLinkBeenCrawled(link URL) bool {
	if fs.bloomFilter.ProbablyContains(link) {
		return fs.crawledURLs.Contains(link)
	}

	return false
}

func (fs *FrontierStore) ProcessLink(ctx context.Context, newUrlToRequest URL) {
	sanitizedURL, err := fs.SanitizeURL(newUrlToRequest)
	if err != nil {
		fs.log.Debug("Website not allowed", zap.String("url", newUrlToRequest.String()))

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

	readyQueueAvailable := len(fs.readyQueue) < fs.workers

	if !fs.claimURL(sanitizedURL) {
		return
	}

	if readyQueueAvailable && bQueue.TryLock() {
		payload := Payload{
			url:           sanitizedURL,
			host:          hostname,
			dialerContext: fs.dnsCache.DialContext,
		}

		select {
		case <-ctx.Done():
			fs.seenURLs.Delete(sanitizedURL)
			bQueue.Unlock()
			return
		case fs.readyQueue <- payload:
		}
		return
	} else {
		//TODO: might want to remove per host
		if fs.pending.Load() < int32(fs.cfg.MaxPendingURLs) {
			if bQueue.Enqueue(sanitizedURL, fs.cfg.MaxURLsPerHost) {
				fs.pending.Add(1)
			} else {
				fs.seenURLs.Delete(sanitizedURL)
				bQueue.SkippedCount.Add(1)
				fs.metrics.URLsSkippedAtLimit.Add(1)
			}

			bQueue.DiscoveredCount.Add(1)
		} else {
			fs.seenURLs.Delete(sanitizedURL)
			fs.metrics.URLsSkippedAtLimit.Add(1)
		}
	}
}

func (fs *FrontierStore) ProcessPage(ctx context.Context, page *Page) error {
	requestedURL := page.RequestedURL.normalizeString()
	finalURL := page.FinalURL.normalizeString()

	if !fs.IsLinkDispatched(requestedURL) {
		return errors.New("Link for some reason not available")
	}

	fs.bloomFilter.Insert(requestedURL)
	fs.crawledURLs.Set(requestedURL, struct{}{})

	fs.bloomFilter.Insert(finalURL)
	fs.crawledURLs.Set(finalURL, struct{}{})

	bQueue, exists := fs.bufferQueues.Get(page.Host)
	if !exists {
		fs.log.Fatal("Buffer queue should exist but doesn't", zap.String("host", page.Host.String()))
	}

	bQueue.Timeout()
	allowedURLs := make([]URL, 0)

	for _, link := range page.Links {
		if fs.HasLinkBeenCrawled(link) {
			fs.metrics.URLDuplicates.Add(1)
			fs.log.Debug("Duplicate link crawled", zap.String("url", page.FinalURL.String()))
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
func (fs *FrontierStore) FindAvailableJobs() ([]Payload, error) {
	jobs := make([]Payload, 0)
	i := 0

	fs.bufferQueues.Range(func(host URL, queue *BQueue) bool {
		availableSlots := cap(fs.readyQueue) - len(fs.readyQueue)
		if i >= availableSlots {
			return false
		}
		if queue.TryLock() {
			for {
				url := queue.Dequeue()
				if url == "" {
					queue.Unlock()
					break
				}

				fs.pending.Add(-1)
				if fs.HasLinkBeenCrawled(url) {
					continue
				}

				jobs = append(jobs, Payload{
					url:           url,
					host:          host,
					dialerContext: fs.dnsCache.DialContext,
				})
				i++
				break
			}
		}

		return true
	})

	return jobs, nil
}

func (fs *FrontierStore) HandleSpiderFail(payload Payload) {
	fs.seenURLs.Delete(payload.url)

	bQueue, exists := fs.bufferQueues.Get(payload.host)
	if exists {
		bQueue.TryUnlock()
	}
}

func (fs *FrontierStore) IsLinkAvailable(link URL) bool {
	//bloom filter and stuff
	return !fs.HasLinkBeenCrawled(link) && !fs.seenURLs.Contains(link)
}

func (fs *FrontierStore) IsLinkDispatched(link URL) bool {
	return fs.seenURLs.Contains(link.normalizeString())
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

		if bQueue.Locked && !bQueue.StaleUntil.IsZero() && time.Now().After(bQueue.StaleUntil) {
			bQueue.Locked = false
		}

		return true
	})
}

func (fs *FrontierStore) GetBufferCount() int {
	return fs.bufferQueues.Length()
}

func (fs *FrontierStore) claimURL(url URL) bool {
	if fs.crawledURLs.Contains(url) {
		return false
	}

	return fs.seenURLs.SetIfAbsent(url, struct{}{})
}

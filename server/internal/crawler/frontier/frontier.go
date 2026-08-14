package frontier

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/twitocode/sift/internal/common"
	"github.com/twitocode/sift/internal/crawler/dedup"
	"github.com/twitocode/sift/internal/crawler/networking"
	"github.com/twitocode/sift/internal/metrics"
	"go.uber.org/zap"
)

type FrontierStore struct {
	hosts *common.SafeMap[common.URL, *BQueue]
  readyHosts *common.SafeMap[common.URL, *BQueue]
  cooldownHosts *common.SafeMap[common.URL, *BQueue] //TODO: convert to min heap
	readyQueue   chan SpiderPayload
	seenURLs     *common.SafeMap[common.URL, struct{}]
	bloomFilter  *dedup.BloomFilter
	crawledURLs  *common.SafeMap[common.URL, struct{}]
	dnsCache     *networking.DNSCache
	metrics      *metrics.CrawlMetrics
	cfg          *common.Config

	blacklist map[string]struct{}
	workers   int
	log       *zap.Logger

	pending atomic.Int32
  rrIndex atomic.Uint64

	hostMu sync.Mutex
}

type FrontierStats struct {
	HostQueues          int
	UniqueHosts         int
	PendingURLs         int
	LargestQueue        int
	LargestQueueHost    common.URL
	OldestQueueAge      time.Duration
	OldestQueueHost     common.URL
	MostDispatchedHost  common.URL
	MostDispatchedCount int64
	LockedHosts         int
	CooldownHosts       int
	AvailableHosts      int
}

func (fs *FrontierStore) ReadyQueue() <-chan SpiderPayload {
	return fs.readyQueue
}

func NewFrontierStore(log *zap.Logger, dnsCache *networking.DNSCache, cfg *common.Config, metrics *metrics.CrawlMetrics, blacklist map[string]struct{}) *FrontierStore {
	return &FrontierStore{
		hosts: common.NewSafeMap[common.URL, *BQueue](),
		readyQueue:   make(chan SpiderPayload, cfg.SpiderCount),
		seenURLs:     common.NewSafeMap[common.URL, struct{}](),
		crawledURLs:  common.NewSafeMap[common.URL, struct{}](),
		bloomFilter:  dedup.NewBloomFilter(float64(cfg.CrawlCount*2), 0.01),
		dnsCache:     dnsCache,
		workers:      cfg.SpiderCount,
		log:          log,
		blacklist:    blacklist,

		metrics: metrics,
		cfg:     cfg,
	}
}

func (fs *FrontierStore) Stats() FrontierStats {
	stats := FrontierStats{
		UniqueHosts: fs.hosts.Length(),
	}
	now := time.Now()

	fs.hosts.Range(func(_ common.URL, queue *BQueue) bool {
		queue.mu.Lock()
		pending := len(queue.URLs)
		locked := queue.Locked
		staleUntil := queue.NextEligibleAt
		var queueAge time.Duration
		if len(queue.QueuedAt) > 0 {
			queueAge = now.Sub(queue.QueuedAt[0])
		}
		queue.mu.Unlock()

		stats.HostQueues++
		stats.PendingURLs += pending
		if pending > stats.LargestQueue {
			stats.LargestQueue = pending
			stats.LargestQueueHost = queue.Host
		}
		if queueAge > stats.OldestQueueAge {
			stats.OldestQueueAge = queueAge
			stats.OldestQueueHost = queue.Host
		}
		if dispatches := queue.DispatchCount.Load(); dispatches > stats.MostDispatchedCount {
			stats.MostDispatchedCount = dispatches
			stats.MostDispatchedHost = queue.Host
		}
		if locked {
			stats.LockedHosts++
			if !staleUntil.IsZero() && now.Before(staleUntil) {
				stats.CooldownHosts++
			}
		} else if staleUntil.IsZero() || now.After(staleUntil) {
			stats.AvailableHosts++
		}

		return true
	})

	return stats
}

func (fs *FrontierStore) AddHost(ctx context.Context, newUrlToRequest common.URL) {
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

	if fs.hosts.Contains(hostname) {
		return
	}

	fs.hostMu.Lock()
	defer fs.hostMu.Unlock()

	if fs.hosts.Contains(hostname) || fs.hosts.Length() >= fs.cfg.MaxHostQueues {
		return
	}

	queue := &BQueue{
		Host:       hostname,
		URLs:       []common.URL{},
		Locked:     false,
		NextEligibleAt: time.Now(),
	}

	fs.hosts.Set(hostname, queue)
}

func (fs *FrontierStore) TryDispatchJob(ctx context.Context, job *SpiderPayload) {
	_, exists := fs.hosts.Get(job.host)
	if exists {
		select {
		case <-ctx.Done():
			return
		case fs.readyQueue <- *job:
		}
	}
}

func (fs *FrontierStore) HasLinkBeenCrawled(link common.URL) bool {
	if fs.bloomFilter.ProbablyContains(link) {
		return fs.crawledURLs.Contains(link)
	}

	return false
}

func (fs *FrontierStore) ProcessLink(ctx context.Context, newUrlToRequest common.URL) {
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

	bQueue, exists := fs.hosts.Get(hostname)
	if !exists {
		return
	}

	readyQueueAvailable := len(fs.readyQueue) < fs.workers

	if !fs.claimURL(sanitizedURL) {
		return
	}

	if readyQueueAvailable && bQueue.TryLock() {
		bQueue.DiscoveredCount.Add(1)
		payload := SpiderPayload{
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
			bQueue.DispatchCount.Add(1)
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

func (fs *FrontierStore) ProcessPage(ctx context.Context, page *common.Page) error {
	requestedURL := page.RequestedURL.NormalizeString()
	finalURL := page.FinalURL.NormalizeString()

	if !fs.IsLinkDispatched(requestedURL) {
		return errors.New("Link for some reason not available")
	}

	fs.bloomFilter.Insert(requestedURL)
	fs.crawledURLs.Set(requestedURL, struct{}{})

	fs.bloomFilter.Insert(finalURL)
	fs.crawledURLs.Set(finalURL, struct{}{})

	bQueue, exists := fs.hosts.Get(page.Host)
	if !exists {
		fs.log.Fatal("Buffer queue should exist but doesn't", zap.String("host", page.Host.String()))
	}

	bQueue.Timeout()
	allowedURLs := make([]common.URL, 0)

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
func (fs *FrontierStore) FindAvailableJobs() ([]SpiderPayload, error) {
	jobs := make([]SpiderPayload, 0)
  
  availableSlots := cap(fs.readyQueue) - len(fs.readyQueue)
  if availableSlots <= 0 {
    return []SpiderPayload{}, nil
  } 

  hosts := make([]common.URL, 0)
  fs.hosts.Range(func(host common.URL, queue *BQueue) bool {
    hosts =append(hosts, host)
    return true
  })

  n := len(hosts)

  if n == 0 {
    return []SpiderPayload{}, nil
  }
  
  //discrete math class ugh
  start := int(fs.rrIndex.Add(1) - 1) % n


  for i := 0; i < n && len(jobs) < availableSlots; i++ {
    host := hosts[(start + i)%n]
    queue, exists := fs.hosts.Get(host)

    if !exists {
      continue
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

				jobs = append(jobs, SpiderPayload{
					url:           url,
					host:          host,
					dialerContext: fs.dnsCache.DialContext,
				})
				queue.DispatchCount.Add(1)
				break
			}
    }
  }

	return jobs, nil
}

func (fs *FrontierStore) HandleSpiderFail(payload SpiderPayload) {
	fs.seenURLs.Delete(payload.url)

	bQueue, exists := fs.hosts.Get(payload.host)
	if exists {
		bQueue.TryUnlock()
	}
}

func (fs *FrontierStore) IsLinkAvailable(link common.URL) bool {
	//bloom filter and stuff
	return !fs.HasLinkBeenCrawled(link) && !fs.seenURLs.Contains(link)
}

func (fs *FrontierStore) IsLinkDispatched(link common.URL) bool {
	return fs.seenURLs.Contains(link.NormalizeString())
}

func (fs *FrontierStore) SanitizeURL(link common.URL) (common.URL, error) {
	if fs.bloomFilter.ProbablyContains(link) {
		if fs.crawledURLs.Contains(link) {
			return "", errors.New("Link already crawled")
		}
	}

	return link.NormalizeString(), nil
}

func (fs *FrontierStore) FreeHosts() {
	fs.hosts.Range(func(url common.URL, bQueue *BQueue) bool {
		bQueue.mu.Lock()
		defer bQueue.mu.Unlock()

		if bQueue.Locked && !bQueue.NextEligibleAt.IsZero() && time.Now().After(bQueue.NextEligibleAt) {
			bQueue.Locked = false
		}

		return true
	})
}

func (fs *FrontierStore) GetBufferCount() int {
	return fs.hosts.Length()
}

func (fs *FrontierStore) claimURL(url common.URL) bool {
	if fs.crawledURLs.Contains(url) {
		return false
	}

	return fs.seenURLs.SetIfAbsent(url, struct{}{})
}

func IsDomainBlacklisted(domain string, blacklist map[string]struct{}) bool {
	_, found := blacklist[domain]
	return found
}

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
	hosts         *common.SafeMap[common.URL, *HostState]
	readyHosts    *HostQueue
	cooldownHosts *CooldownHeap
	wakeScheduler chan struct{}

	readyQueue  chan SpiderJob
	seenURLs    *common.SafeMap[common.URL, struct{}]
	bloomFilter *dedup.BloomFilter
	crawledURLs *common.SafeMap[common.URL, struct{}]
	dnsCache    *networking.DNSCache
	metrics     *metrics.CrawlMetrics
	cfg         *common.Config

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

func (fs *FrontierStore) ReadyQueue() <-chan SpiderJob {
	return fs.readyQueue
}

func NewFrontierStore(log *zap.Logger, dnsCache *networking.DNSCache, cfg *common.Config, metrics *metrics.CrawlMetrics, blacklist map[string]struct{}) *FrontierStore {
	return &FrontierStore{
		hosts:         common.NewSafeMap[common.URL, *HostState](),
		readyHosts:    NewHostQueue(),
		readyQueue:    make(chan SpiderJob, cfg.SpiderCount),
		wakeScheduler: make(chan struct{}, 1),
		seenURLs:      common.NewSafeMap[common.URL, struct{}](),
		crawledURLs:   common.NewSafeMap[common.URL, struct{}](),
		bloomFilter:   dedup.NewBloomFilter(float64(cfg.CrawlCount*2), 0.01),
		cooldownHosts: NewCooldownHeap(cfg.MaxHostQueues),
		dnsCache:      dnsCache,
		workers:       cfg.SpiderCount,
		log:           log,
		blacklist:     blacklist,

		metrics: metrics,
		cfg:     cfg,
	}
}

func (fs *FrontierStore) Stats() FrontierStats {
	stats := FrontierStats{
		UniqueHosts: fs.hosts.Length(),
	}
	now := time.Now()

	fs.hosts.Range(func(_ common.URL, queue *HostState) bool {
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

func (fs *FrontierStore) AddOrRetrieveHost(ctx context.Context, newUrlToRequest common.URL) (*HostState, error) {
	hostname, err := newUrlToRequest.GetHost()

	if err != nil {
		fs.log.Warn("Invalid url given", zap.String("url", newUrlToRequest.String()))
		return nil, errors.New("Invalid url")
	}

	domain, _ := newUrlToRequest.GetDomain()
	if IsDomainBlacklisted(domain, fs.blacklist) {
		fs.metrics.BlacklistedWebsites.Add(1)
		return nil, errors.New("Blacklisted host")
	}

	state, exists := fs.hosts.Get(hostname)

	if exists {
		if state.IsAvailable() &&
			state.IsScheduled.CompareAndSwap(false, true) {
			fs.readyHosts.Push(state)
			fs.signalScheduler()
		}

		return state, nil
	}

	fs.hostMu.Lock()
	defer fs.hostMu.Unlock()

	if fs.hosts.Length() >= fs.cfg.MaxHostQueues {
		return nil, errors.New("text string")
	}

	state = &HostState{
		Host:           hostname,
		URLs:           []common.URL{},
		Locked:         false,
		NextEligibleAt: time.Now(),
	}

	fs.hosts.Set(hostname, state)
	fs.readyHosts.Push(state)
	fs.signalScheduler()
	return state, nil
}

func (fs *FrontierStore) AddLink(ctx context.Context, host *HostState, newUrlToRequest common.URL) {
	sanitizedURL, err := fs.SanitizeURL(newUrlToRequest)
	if err != nil {
		fs.log.Debug("Website not allowed", zap.String("url", newUrlToRequest.String()))
		fs.metrics.URLsRejected.Add(1)

		return
	}

	//simplified so that it does not add to ready queue asap (might change back later)

	/*
		Policy: if the number of urls we are tracking is larger than the max OR the number of urls in the host is at max then DONT enqueue
	*/
	if fs.pending.Load() < int32(fs.cfg.MaxPendingURLs) {
		if host.Enqueue(sanitizedURL, fs.cfg.MaxURLsPerHost) {
			fs.pending.Add(1)
		} else {
			host.SkippedCount.Add(1)
			fs.metrics.URLsSkippedAtLimit.Add(1)
		}
	} else {
		fs.metrics.URLsSkippedAtLimit.Add(1)
	}
}

func (fs *FrontierStore) AfterPageProcessed(ctx context.Context, page *common.Page) error {
	requestedURL := page.RequestedURL.NormalizeString()
	finalURL := page.FinalURL.NormalizeString()

	if !fs.IsLinkDispatched(requestedURL) {
		return errors.New("Link for some reason not available")
	}

	fs.bloomFilter.Insert(requestedURL)
	fs.crawledURLs.Set(requestedURL, struct{}{})

	fs.bloomFilter.Insert(finalURL)
	fs.crawledURLs.Set(finalURL, struct{}{})

	host, exists := fs.hosts.Get(page.Host)
	if !exists {
		fs.log.Warn("Host removed from global list", zap.String("host", page.Host.String()))
		return nil
	}

	host.DiscoveredCount.Add(1)

	if host.Len() <= 0 {
		fs.hosts.Delete(host.Host)
		return nil
	}

	host.IsScheduled.Store(false)
	host.Timeout()
	fs.cooldownHosts.Add(host)
	fs.signalScheduler()

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

func (fs *FrontierStore) DispatchJob(ctx context.Context, job *SpiderJob) {
	select {
	case <-ctx.Done():
		return
	case fs.readyQueue <- *job:
	}
}

// used for timer
func (fs *FrontierStore) FindAvailableJobs() ([]SpiderJob, error) {
	jobs := make([]SpiderJob, 0)

	availableSlots := cap(fs.readyQueue) - len(fs.readyQueue)
	if availableSlots <= 0 {
		return []SpiderJob{}, nil
	}

	for range availableSlots {
		host := fs.readyHosts.Pop()
		if host == nil {
			continue
		}

		var earliestUrl common.URL

		for {
			if host.Len() <= 0 {
				earliestUrl = ""
				break
			}
			earliestUrl = host.Dequeue()
			if fs.HasLinkBeenCrawled(earliestUrl) {
				continue
			}

			break
		}

		if earliestUrl == "" {
			host.Unlock()
			break
		}

		newJob := SpiderJob{
			url:           earliestUrl,
			hostname:      host.Host,
			dialerContext: fs.dnsCache.DialContext,
		}

		fs.pending.Add(-1)

		jobs = append(jobs, newJob)
	}

	return jobs, nil
}

func (fs *FrontierStore) HandleSpiderFail(job SpiderJob) {
}

func (fs *FrontierStore) HasLinkBeenCrawled(link common.URL) bool {
	if fs.bloomFilter.ProbablyContains(link) {
		return fs.crawledURLs.Contains(link)
	}

	return false
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

func (fs *FrontierStore) FreeExpiredHosts(now time.Time) {
	for {
		rootHost, err := fs.cooldownHosts.Peek()
		if err != nil {
			return
		}

		if now.After(rootHost.NextEligibleAt) {
			rootHost, _ = fs.cooldownHosts.Poll()
			if rootHost.Len() > 0 {
				fs.readyHosts.Push(rootHost)
			} else {
				fs.hosts.Delete(rootHost.Host)
			}
			continue
		}

		break
	}
}

func (fs *FrontierStore) GetTimerCooldown() time.Time {
	rootHost, err := fs.cooldownHosts.Peek()
	if err != nil {
		return time.Now()
	}
	return rootHost.NextEligibleAt
}

func (fs *FrontierStore) SchedulerWake() <-chan struct{} {
	return fs.wakeScheduler
}

func (fs *FrontierStore) signalScheduler() {
	select {
	case fs.wakeScheduler <- struct{}{}:
	default:
	}
}

func IsDomainBlacklisted(domain string, blacklist map[string]struct{}) bool {
	_, found := blacklist[domain]
	return found
}

func (fs *FrontierStore) Shutdown() {
	//close(fs.readyQueue)
}

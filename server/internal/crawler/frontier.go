package crawler

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
)

type PageMetadata struct {
	URL         URL
	Host        URL
	Title       string
	Description string
	Text        string
	Links       []URL
	StatusCode  int // some pages return 429 stuff like that so i can filter out later if needed
	CrawledAt   time.Time
	ContentHash string //TODO: duplication detection, hash text form page (different urls same text)
}

type FrontierStore struct {
	bufferQueues *SafeMap[URL, *BQueue]
	readyQueue   chan SpiderPayload
	dispatched   *SafeMap[URL, struct{}]
	bloomFilter  *BloomFilter

	dnsCache *DNSCache

	workers int
	log     *zap.Logger
}

func NewFrontierStore(log *zap.Logger, dnsCache *DNSCache, workerCount, maxPagesCrawled int) *FrontierStore {
	return &FrontierStore{
		bufferQueues: NewSafeMap[URL, *BQueue](),
		readyQueue:   make(chan SpiderPayload, workerCount),
		dispatched:   NewSafeMap[URL, struct{}](),
		bloomFilter:  NewBloomFilter(float64(maxPagesCrawled), 0.1),
		dnsCache:     dnsCache,
		workers:      workerCount,
		log:          log,
	}
}

func (fs *FrontierStore) AddUrl(ctx context.Context, rawUrl URL, linkReceiveChan chan<- URL) {
	hostname, err := rawUrl.GetHost()
	if err != nil {
		fs.log.Warn("Invalid url given", zap.String("url", rawUrl.String()))
		return
	}

	if !fs.bufferQueues.Contains(hostname) {
		//fs.log.Debug("BQueue cache miss", zap.String("host", hostname.String()))
		queue := &BQueue{
			Host:       hostname,
			URLs:       []URL{},
			Locked:     false,
			StaleUntil: time.Now(),
		}

		fs.bufferQueues.Set(hostname, queue)
	}
	select {
	case <-ctx.Done():
		return
	case linkReceiveChan <- rawUrl:
	}
}

func (fs *FrontierStore) TryDispatchJob(ctx context.Context, job *SpiderPayload) {
	bQueue, exists := fs.bufferQueues.Get(job.host)
	if exists && bQueue.IsAvailable() {
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
	return fs.bloomFilter.ProbablyContains(link)
}

func (fs *FrontierStore) ProcessLink(ctx context.Context, link URL) {
	link, err := fs.SanitizeURL(link)
	if err != nil {
		fs.log.Warn("Website not allowed", zap.String("url", link.String()))
		return
	}

	hostname, err := link.GetHost()
	if err != nil {
		fs.log.Warn("Invalid url given (linkReceiveChan)", zap.String("url", link.String()))
		return
	}

	bQueue, exists := fs.bufferQueues.Get(hostname)
	if !exists {
		return
	}

	dispatchedAlready := fs.dispatched.Contains(link)
	hostQueueAvailable := bQueue.IsAvailable()
	readyQueueAvailable := len(fs.readyQueue) < fs.workers

	if !dispatchedAlready && hostQueueAvailable && readyQueueAvailable {
		payload := SpiderPayload{
			url:           link,
			host:          hostname,
			dialerContext: fs.dnsCache.DialContext,
		}

		select {
		case <-ctx.Done():
			return
		case fs.readyQueue <- payload:
			fs.dispatched.Set(link, struct{}{})
			bQueue.Lock()
		}
		return
	} else {
		//fs.log.Debug("Adding back to buffer", zap.String("host", hostname.String()), zap.String("url", link.String()))
		bQueue.Enqueue(link)
	}
}

func (fs *FrontierStore) ProcessPage(ctx context.Context, page *PageMetadata) error {
	if !fs.IsLinkDispatched(page.URL) {
		return errors.New("Link for some reason not available")
	}

	fs.bloomFilter.Insert(page.URL)
	fs.dispatched.Delete(page.URL)

	bQueue, exists := fs.bufferQueues.Get(page.Host)
	if !exists {
		fs.log.Fatal("Buffer queue should exist but doesn't", zap.String("host", page.Host.String()))
	}

	bQueue.Timeout(4000)
	allowedURLs := make([]URL, 0)

	for _, link := range page.Links {
		if fs.HasLinkBeenCrawled(link) {
			fs.log.Debug("Duplicate link crawled", zap.String("url", page.URL.String()))
			continue
		}

		allowedURLs = append(allowedURLs, link)
	}

	page.Links = allowedURLs
	return nil
}

func (fs *FrontierStore) Shutdown() {
	close(fs.readyQueue)
}

// used for timer
func (fs *FrontierStore) FindAvailableJob() (*SpiderPayload, error) {
	var job *SpiderPayload

	fs.bufferQueues.Range(func(host URL, queue *BQueue) bool {
		queue.mu.Lock()
		defer queue.mu.Unlock()

		if queue.IsAvailable() {
			url := queue.Dequeue()
			job = &SpiderPayload{
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

func (fs *FrontierStore) HandleSpiderFail(host URL) {
	bQueue, exists := fs.bufferQueues.Get(host)
	if exists {
		bQueue.Timeout(400)
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
		return "", errors.New("Link already crawled")
	}

	return link, nil
}

package crawler

import (
	"context"
	"sync"
	"time"

	"github.com/twitocode/sift/internal/common"
	"go.uber.org/zap"
)

type Engine struct {
	pageReceiveChan chan *Page
	linkReceiveChan chan URL
	spiderFailChan  chan Payload

	maxPagesCrawled int

	pagesCrawled int
	pagesFetched int
	workers      int

	metrics  *CrawlMetrics
	cfg      *common.Config
	dnsCache *DNSCache
	log      *zap.Logger
	workerWg sync.WaitGroup
	store    *PageStore
	frontier *FrontierStore

	crawlStartedAt  time.Time
	lastMilestoneAt time.Time
}

func NewEngine(log *zap.Logger, store *PageStore, cfg *common.Config) *Engine {
	metrics := NewCrawlMetrics(log)
	dnsCache := NewDNSCache(log, metrics)

	return &Engine{
		pageReceiveChan: make(chan *Page, int(float32(cfg.SpiderCount)*1.5)),
		linkReceiveChan: make(chan URL, 2048),
		spiderFailChan:  make(chan Payload, cfg.SpiderCount),
		maxPagesCrawled: cfg.CrawlCount,
		pagesCrawled:    0,
		workers:         cfg.SpiderCount,
		store:           store,
		cfg:             cfg,
		frontier:        NewFrontierStore(log, dnsCache, cfg, metrics),
		metrics:         metrics,
		dnsCache:        dnsCache,
		log:             log,
	}
}

func (e *Engine) Start() {
	ctx, cancel := context.WithCancel(context.Background())

	startTime := time.Now()
	e.crawlStartedAt = startTime
	e.lastMilestoneAt = startTime
	ticker := time.NewTicker(time.Millisecond * time.Duration(e.cfg.DispatchDelay))
	defer ticker.Stop()

	linkWorkers := 10
	// +1 from store
	e.workerWg.Add(e.workers + linkWorkers + 1)

	go e.store.RunTimer(ctx, &e.workerWg, e.metrics)
	go e.Seed(ctx)

	//TODO: collect from db first
	go e.startLinkWorkers(ctx, linkWorkers)
	go e.startSpiders(ctx, e.workers)
	go e.loop(ctx, ticker, cancel)

	e.workerWg.Wait()
	e.metrics.PrintSummary(time.Since(startTime))
}

func (e *Engine) loop(ctx context.Context, ticker *time.Ticker, cancel context.CancelFunc) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.frontier.FreeHosts()
			potentialJobs, err := e.frontier.FindAvailableJobs()

			if err != nil {
				continue
			}

			for _, job := range potentialJobs {
				e.frontier.TryDispatchJob(ctx, &job)
			}

		case page := <-e.pageReceiveChan:

			if page == nil {
				continue
			}

			if e.pagesFetched%250 == 0 {
				now := time.Now()
				//stats := e.frontier.Stats()

				e.log.Info("crawl milestone",
					zap.Int("pages_crawled", e.pagesCrawled),
					zap.Int("pages_fetched", e.pagesFetched),
					zap.Duration("milestone_elapsed", now.Sub(e.lastMilestoneAt)),
					zap.Duration("total_elapsed", now.Sub(e.crawlStartedAt)),
					// zap.Int("host_queues", stats.HostQueues),
					// zap.Int("pending_urls", stats.PendingURLs),
					// zap.Int("largest_host_queue", stats.LargestQueue),
					// zap.Int("ready_queue", len(e.frontier.readyQueue)),
					// zap.Int("link_queue", len(e.linkReceiveChan)),
					// zap.Int("page_queue", len(e.pageReceiveChan)),
					zap.Int("in flight", int(e.metrics.InFlight.Load())),
				)

				e.lastMilestoneAt = now
			}

			e.pagesFetched++
			if page.HasBeenCrawled {
				e.pagesCrawled++
			}

			e.store.Add(ctx, *page)

			e.frontier.ProcessPage(ctx, page)
			if e.pagesCrawled == e.maxPagesCrawled {
				e.shutdown(cancel)
				return
			}

			for _, link := range page.Links {
				select {
				case <-ctx.Done():
					return
				case e.linkReceiveChan <- link:
					e.metrics.URLsDiscovered.Add(1)
				}
			}

		case payload := <-e.spiderFailChan:
			e.frontier.HandleSpiderFail(payload)
		}
	}
}

func (e *Engine) Seed(ctx context.Context) {
	for _, link := range seed {
		e.frontier.AddUrl(ctx, link)
		e.frontier.ProcessLink(ctx, link)
	}
}

func (e *Engine) startSpiders(ctx context.Context, workerCount int) {
	e.log.Info("Starting up spiders")
	client := newHttpClient(e.dnsCache.DialContext, workerCount)

	go func() {
		for w := 1; w <= workerCount; w++ {
			spider := NewSpider(
				w,
				e.log,
				client,
				e.frontier.readyQueue,
				e.pageReceiveChan,
				e.spiderFailChan,
				e.dnsCache.DialContext,
				e.metrics,
			)

			go func() {
				defer e.workerWg.Done()
				spider.Walk(ctx)
			}()
		}
	}()
}

func (e *Engine) startLinkWorkers(ctx context.Context, count int) {
	for range count {
		go func() {
			defer e.workerWg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				case link, ok := <-e.linkReceiveChan:
					if !ok {
						return
					}
					e.frontier.AddUrl(ctx, link)
					e.frontier.ProcessLink(ctx, link)
				}
			}
		}()
	}
}

func (e *Engine) shutdown(cancel context.CancelFunc) {
	cancel()
	e.store.Shutdown()
	e.frontier.Shutdown()

	e.metrics.PagesSkipped.Add(int64(e.pagesFetched - e.pagesCrawled))

	//might not need to close
	//close(e.pageReceiveChan) //might be a code smell
	//close(e.linkReceiveChan)
}

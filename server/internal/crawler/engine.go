package crawler

import (
	"context"
	"sync"
	"time"

	"github.com/twitocode/sift/internal/common"
	"github.com/twitocode/sift/internal/crawler/frontier"
	"github.com/twitocode/sift/internal/crawler/networking"
	"github.com/twitocode/sift/internal/metrics"
	"github.com/twitocode/sift/internal/store"
	"go.uber.org/zap"
)

type Engine struct {
	pageReceiveChan chan *common.Page
	linkReceiveChan chan common.URL
	spiderFailChan  chan frontier.SpiderPayload

	maxPagesCrawled int

	pagesCrawled int
	pagesFetched int
	workers      int

	metrics  *metrics.CrawlMetrics
	cfg      *common.Config
	dnsCache *networking.DNSCache
	log      *zap.Logger
	workerWg sync.WaitGroup
	store    *store.PageStore
	frontier *frontier.FrontierStore

	crawlStartedAt  time.Time
	lastMilestoneAt time.Time
}

func NewEngine(log *zap.Logger, store *store.PageStore, cfg *common.Config) *Engine {
	metrics := metrics.NewCrawlMetrics(log)
	dnsCache := networking.NewDNSCache(log, metrics)

	return &Engine{
		pageReceiveChan: make(chan *common.Page, int(float32(cfg.SpiderCount)*1.5)),
		linkReceiveChan: make(chan common.URL, 2048),
		spiderFailChan:  make(chan frontier.SpiderPayload, cfg.SpiderCount),
		maxPagesCrawled: cfg.CrawlCount,
		pagesCrawled:    0,
		workers:         cfg.SpiderCount,
		store:           store,
		cfg:             cfg,
		frontier:        frontier.NewFrontierStore(log, dnsCache, cfg, metrics, GenerateBlacklistMap(DefaultBlacklistedDomains)),
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
				stats := e.frontier.Stats()

				e.log.Info("crawl milestone",
					zap.Int("pages_crawled", e.pagesCrawled),
					zap.Int("pages_fetched", e.pagesFetched),
					zap.Duration("milestone_elapsed", now.Sub(e.lastMilestoneAt)),
					zap.Duration("total_elapsed", now.Sub(e.crawlStartedAt)),
					zap.Int("host_queues", stats.HostQueues),
					zap.Int("pending_urls", stats.PendingURLs),
					zap.Int("largest_host_queue", stats.LargestQueue),
					zap.Int("locked_hosts", stats.LockedHosts),
					zap.Int("cooldown_hosts", stats.CooldownHosts),
					zap.Int("available_hosts", stats.AvailableHosts),
					zap.Int("ready_queue", len(e.frontier.ReadyQueue())),
					zap.Int("link_queue", len(e.linkReceiveChan)),
					zap.Int("page_queue", len(e.pageReceiveChan)),
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

			for _, newUrlToRequest := range page.Links {
				select {
				case <-ctx.Done():
					return
				case e.linkReceiveChan <- newUrlToRequest:
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
		e.frontier.AddHost(ctx, link)
		e.frontier.ProcessLink(ctx, link)
	}
}

func (e *Engine) startSpiders(ctx context.Context, workerCount int) {
	e.log.Info("Starting up spiders")
	client := networking.NewHttpClient(e.dnsCache.DialContext, workerCount)

	go func() {
		for w := 1; w <= workerCount; w++ {
			spider := frontier.NewSpider(
				w,
				e.log,
				client,
				e.frontier.ReadyQueue(),
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
				case newUrlToRequest, ok := <-e.linkReceiveChan:
					if !ok {
						return
					}
					e.frontier.AddHost(ctx, newUrlToRequest)
					e.frontier.ProcessLink(ctx, newUrlToRequest)
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

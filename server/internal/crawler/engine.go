package crawler

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/twitocode/sift/internal/common"
	"github.com/twitocode/sift/internal/crawler/frontier"
	"github.com/twitocode/sift/internal/crawler/networking"
	"github.com/twitocode/sift/internal/metrics"
	"github.com/twitocode/sift/internal/progress"
	"github.com/twitocode/sift/internal/store"
	"go.uber.org/zap"
)

type Engine struct {
	pageReceiveChan chan *common.Page
	linkReceiveChan chan common.URL
	spiderFailChan  chan frontier.SpiderJob

	maxPagesCrawled int

	pagesCrawled atomic.Int64
	pagesFetched atomic.Int64
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
		spiderFailChan:  make(chan frontier.SpiderJob, cfg.SpiderCount),
		maxPagesCrawled: cfg.CrawlCount,
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

	linkWorkers := 10
	// +1 from store
	e.workerWg.Add(e.workers + linkWorkers + 1)

	go e.store.RunTimer(ctx, &e.workerWg, e.metrics)
	go e.Seed(ctx)

	//TODO: collect from db first
	go e.startLinkWorkers(ctx, linkWorkers)
	go e.startSpiders(ctx, e.workers)
	go e.loop(ctx, cancel)

	e.workerWg.Wait()
	stats := e.frontier.Stats(true)
	e.metrics.PrintSummary(time.Since(startTime), metrics.FrontierSummary{
		UniqueHosts:         stats.UniqueHosts,
		PendingURLs:         stats.PendingURLs,
		LargestQueue:        stats.LargestQueue,
		LargestQueueHost:    stats.LargestQueueHost.String(),
		OldestQueueAge:      stats.OldestQueueAge,
		OldestQueueHost:     stats.OldestQueueHost.String(),
		MostDispatchedCount: stats.MostDispatchedCount,
		MostDispatchedHost:  stats.MostDispatchedHost.String(),
		LockedHosts:         stats.LockedHosts,
		CooldownHosts:       stats.CooldownHosts,
		AvailableHosts:      stats.AvailableHosts,
	})
}

func (e *Engine) loop(ctx context.Context, cancel context.CancelFunc) {
	var timerC <-chan time.Time

	timer := time.NewTimer(time.Hour)
	timer.Stop()

	timerC = timer.C

	for {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}

		deadline, hasCooldown := e.frontier.GetTimerCooldown()
		if hasCooldown {
			timer.Reset(max(0, time.Until(deadline)))
			timerC = timer.C
		} else {
			timerC = nil
		}

		select {
		case <-ctx.Done():
			return
		case <-timerC:
			e.frontier.FreeExpiredHosts(time.Now())
			e.dispatchAvailableJobs(ctx)

		case <-e.frontier.SchedulerWake():
			e.dispatchAvailableJobs(ctx)

		case page := <-e.pageReceiveChan:
			if page == nil {
				continue
			}

			pagesFetched := e.pagesFetched.Load()
			if pagesFetched%250 == 0 {
				now := time.Now()
				stats := e.frontier.Stats(e.cfg.ShowCrawlStats)

				e.log.Info("crawl milestone",
					zap.Int64("pages_crawled", e.pagesCrawled.Load()),
					zap.Int64("pages_fetched", pagesFetched),
					zap.Duration("milestone_elapsed", now.Sub(e.lastMilestoneAt)),
					zap.Duration("total_elapsed", now.Sub(e.crawlStartedAt)),
					zap.Int("host_queues", stats.HostQueues),
					zap.Int("unique_hosts", stats.UniqueHosts),
					zap.Int("pending_urls", stats.PendingURLs),
					zap.Int("largest_host_queue", stats.LargestQueue),
					zap.String("largest_host_queue_host", stats.LargestQueueHost.String()),
					zap.Duration("oldest_queued_url_age", stats.OldestQueueAge),
					zap.String("oldest_queued_url_host", stats.OldestQueueHost.String()),
					zap.Int64("most_dispatched_host_count", stats.MostDispatchedCount),
					zap.String("most_dispatched_host", stats.MostDispatchedHost.String()),
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

			e.pagesFetched.Add(1)
			if page.HasBeenCrawled {
				e.pagesCrawled.Add(1)
			}

			e.store.Add(ctx, *page)
			e.frontier.AfterPageProcessed(ctx, page)

			if e.pagesCrawled.Load() == int64(e.maxPagesCrawled) {
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

		case job := <-e.spiderFailChan:
			e.frontier.HandleSpiderFail(job)
		}
	}
}

func (e *Engine) dispatchAvailableJobs(ctx context.Context) {
	potentialJobs, err := e.frontier.FindAvailableJobs()
	if err != nil {
		return
	}

	for _, job := range potentialJobs {
		e.frontier.DispatchJob(ctx, &job)
	}
}

func (e *Engine) Seed(ctx context.Context) {
	for _, link := range seed {
		queue, err := e.frontier.AddOrRetrieveHost(ctx, link)

		if err == nil {
			e.frontier.AddLink(ctx, queue, link)
		}
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
					queue, err := e.frontier.AddOrRetrieveHost(ctx, newUrlToRequest)

					if err == nil {
						e.frontier.AddLink(ctx, queue, newUrlToRequest)
					}
				}
			}
		}()
	}
}

func (e *Engine) shutdown(cancel context.CancelFunc) {
	cancel()
	e.store.Shutdown()
	e.frontier.Shutdown()

	e.metrics.PagesSkipped.Add(e.pagesFetched.Load() - e.pagesCrawled.Load())

	//might not need to close
	//close(e.pageReceiveChan) //might be a code smell
	//close(e.linkReceiveChan)
}

func (e *Engine) Snapshot() progress.Snapshot {
	stats := e.frontier.Stats(true)
	return progress.Snapshot{
		Crawl: progress.CrawlSnapshot{
			Limit:          e.maxPagesCrawled,
			PagesCrawled:   int(e.pagesCrawled.Load()),
			PagesFetched:   int(e.pagesFetched.Load()),
			PagesStored:    e.metrics.PagesStored.Load(),
			URLsDiscovered: e.metrics.URLsDiscovered.Load(),
			URLsFetched:    e.metrics.URLsFetched.Load(),
			FetchFailures:  e.metrics.FetchFailures.Load(),
			InFlight:       e.metrics.InFlight.Load(),
			PendingURLs:    stats.PendingURLs,
			UniqueHosts:    stats.UniqueHosts,
			AvailableHosts: stats.AvailableHosts,
			LockedHosts:    stats.LockedHosts,
			CooldownHosts:  stats.CooldownHosts,
		},
	}
}

package crawler

import (
	"context"
	"fmt"
	"net/http"
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
	workers      int

	metrics  *CrawlMetrics
	cfg      *common.Config
	dnsCache *DNSCache
	log      *zap.Logger
	workerWg sync.WaitGroup
	store    *PageStore
	frontier *FrontierStore
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
	ticker := time.NewTicker(time.Millisecond * 1000)
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
			potentialJob, err := e.frontier.FindAvailableJob()

			if err != nil {
				continue
			}

			if potentialJob != nil {
				e.frontier.TryDispatchJob(ctx, potentialJob)
			}

		case page := <-e.pageReceiveChan:
			if page == nil {
				continue
			}
			e.pagesCrawled++

			if e.pagesCrawled%250 == 0 {
				e.log.Info(fmt.Sprintf("Finished Job %d", e.pagesCrawled), zap.Int("hosts tracked", e.frontier.GetBufferCount()))
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
	go func() {
		var client *http.Client
		for w := 1; w <= workerCount; w++ {

			client = newHttpClient(e.dnsCache.DialContext)

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

	//might not need to close
	//close(e.pageReceiveChan) //might be a code smell
	//close(e.linkReceiveChan)
}

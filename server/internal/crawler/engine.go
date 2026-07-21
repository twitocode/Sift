package crawler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

type Engine struct {
	pageReceiveChan chan *PageMetadata
	linkReceiveChan chan URL
	spiderFailChan  chan URL

	maxPagesCrawled int
	pagesCrawled    int
	workers         int

	dnsCache *DNSCache
	log      *zap.Logger
	workerWg sync.WaitGroup
	pageRepo *PageRepository
	frontier *FrontierStore
}

func NewEngine(log *zap.Logger, pageRepo *PageRepository) *Engine {
	const workers = 150

	dnsCache := NewDNSCache(log)
	maxPagesCrawled := 10_000

	return &Engine{
		pageReceiveChan: make(chan *PageMetadata, 256),
		linkReceiveChan: make(chan URL, 2048),
		spiderFailChan:  make(chan URL, workers),
		maxPagesCrawled: maxPagesCrawled,
		pagesCrawled:    0,
		workers:         workers,
		pageRepo:        pageRepo,
		frontier:        NewFrontierStore(log, dnsCache, workers, maxPagesCrawled),
		dnsCache:        dnsCache,
		log:             log,
	}
}

func (e *Engine) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	startTime := time.Now()

	e.workerWg.Add(e.workers)

	go e.pageRepo.RunTimer(ctx)

	go e.Seed(ctx)
	go e.startWorkers(ctx, e.workers)
	go e.loop(ctx, cancel)

	e.workerWg.Wait()
	e.log.Info("Finished Crawling", zap.Int("count", e.pagesCrawled), zap.Duration("elapsed", time.Since(startTime)))
}

func (e *Engine) loop(ctx context.Context, cancel context.CancelFunc) {
	ticker := time.NewTicker(time.Millisecond * 1000)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				potentialJob, err := e.frontier.FindAvailableJob()
				if err != nil {
					continue
				}

				e.frontier.TryDispatchJob(ctx, potentialJob)

			case page := <-e.pageReceiveChan:
				if e.pagesCrawled >= e.maxPagesCrawled {
					e.shutdown(cancel)
					return
				}

				e.pagesCrawled++
				e.log.Info(fmt.Sprintf("Finished Job %d", e.pagesCrawled), zap.String("url", page.URL.String()))

				e.pageRepo.Add(ctx, *page)
				e.frontier.ProcessPage(ctx, page)

				for _, link := range page.Links {
					go e.frontier.AddUrl(ctx, link, e.linkReceiveChan)
				}
			case link := <-e.linkReceiveChan:
				e.frontier.ProcessLink(ctx, link)
			case host := <-e.spiderFailChan:
				e.frontier.HandleSpiderFail(host)
			}
		}
	}()
}

func (e *Engine) Seed(ctx context.Context) {
	for _, link := range seed {
		go e.frontier.AddUrl(ctx, link, e.linkReceiveChan)
	}
}

func (e *Engine) startWorkers(ctx context.Context, workerCount int) {
	e.log.Info("Starting up workers")
	go func() {
		for w := 1; w <= workerCount; w++ {
			spider := NewSpider(
				w,
				e.log,
				e.frontier.readyQueue,
				e.pageReceiveChan,
				e.spiderFailChan,
				e.dnsCache.DialContext,
			)

			go func() {
				defer e.workerWg.Done()
				spider.Walk(ctx)
			}()
		}
	}()
}

func (e *Engine) shutdown(cancel context.CancelFunc) {
	cancel()
	e.frontier.Shutdown()

	//might not need to close
	//close(e.pageReceiveChan) //might be a code smell
	//close(e.linkReceiveChan)
}

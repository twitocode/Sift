package crawler

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

var MAX_DEPTH = 5

type Job struct {
	url   string
	depth int
}

type domainEntry struct {
	ch   chan Job
	stop chan struct{}
}

type ProcessingQueue struct {
	//domain as key
	filteredJobs  *SafeMap[string, domainEntry]
	pending       chan Job
	maxSubPoolCap int
	processFunc   func(job Job)
	siteRepo      *SiteRepository

	shutdownMu sync.Mutex
	closed     bool

	log *zap.Logger
	wg  sync.WaitGroup
}

func NewProcessingQueue(buffer int, siteRepo *SiteRepository, processFunc func(job Job), logger *zap.Logger) *ProcessingQueue {
	return &ProcessingQueue{
		filteredJobs:  NewSafeMap[string, domainEntry](),
		pending:       make(chan Job, buffer),
		log:           logger,
		maxSubPoolCap: 4,
		processFunc:   processFunc,
		siteRepo:      siteRepo,
	}
}
func (q *ProcessingQueue) Push(job Job) {
	q.shutdownMu.Lock()
	if q.closed {
		q.shutdownMu.Unlock()
		return
	}

	exists, err := q.siteRepo.Contains(context.Background(), job.url)
	if err != nil {
		q.log.Error("Sqlite error (push)", zap.Error(err))
	}

	if exists {
		q.shutdownMu.Unlock()
		return
	}

	domain, err := GetDomain(job.url)
	if err != nil {
		q.log.Warn("Invalid domain name", zap.String("url", job.url))
		q.shutdownMu.Unlock()
		return
	}

	entry, ok := q.filteredJobs.Get(domain)
	if !ok {
		entry = domainEntry{
			ch:   make(chan Job, q.maxSubPoolCap),
			stop: make(chan struct{}),
		}
		q.log.Debug("Proceessing new domain", zap.String("domain", domain))
		q.filteredJobs.Set(domain, entry)
		q.wg.Add(1)
		go q.consumeDomain(domain, entry)
	}
	q.shutdownMu.Unlock()

	select {
	case entry.ch <- job:
	case <-entry.stop:
	}
}

func (q *ProcessingQueue) consumeDomain(domain string, entry domainEntry) {
	defer q.wg.Done()
	idleTimeout := 20 * time.Second
	timer := time.NewTimer(idleTimeout)
	defer timer.Stop()

	for {
		select {
		case job := <-entry.ch:
			timer.Reset(idleTimeout)
			if job.depth > MAX_DEPTH {
				continue
			}
			//time.Sleep(2 * time.Second)
			q.processFunc(job)

		case <-timer.C:
			q.shutdownMu.Lock()
			if len(entry.ch) == 0 {
				q.filteredJobs.Delete(domain)
				close(entry.stop) // unblocks any Push currently racing to send here
			}
			q.shutdownMu.Unlock()
			return

		case <-entry.stop:
			return
		}
	}
}

func (q *ProcessingQueue) Run(ctx context.Context) {
	<-ctx.Done()
}

func (q *ProcessingQueue) Close() {
	q.shutdownMu.Lock()
	q.closed = true
	close(q.pending)
	q.filteredJobs.Range(func(k string, v domainEntry) {
		close(v.stop)
	})
	q.shutdownMu.Unlock()
	q.wg.Wait()
}

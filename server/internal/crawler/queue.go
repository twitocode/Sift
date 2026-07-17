package crawler

import (
	"context"
	"sync"

	"go.uber.org/zap"
)

type Job struct {
	url string
}

type ProcessingQueue struct {
	jobs chan Job
	log  *zap.Logger
	wg   sync.WaitGroup
}

func NewProcessingQueue(buffer int, logger *zap.Logger) *ProcessingQueue {
	return &ProcessingQueue{
		jobs: make(chan Job, buffer),
		log:  logger,
	}
}

func (q *ProcessingQueue) Push(job Job) {
	//q.log.Debug("URL added to queue", zap.String("url", job.url))
	q.jobs <- job
}

func (q *ProcessingQueue) Run(ctx context.Context, processFunc func(job Job)) {
	for {
		select {
		case job, ok := <-q.jobs:
			if !ok {
				return
			}
			q.wg.Add(1)

			go func() {
				defer q.wg.Done()
				processFunc(job)
			}()

		case <-ctx.Done():
			return
		}
	}
}

func (q *ProcessingQueue) Close() {
	close(q.jobs) //stops accepting
	q.wg.Wait()
}

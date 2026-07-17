package crawler

import (
	"context"
	"sync"

	"go.uber.org/zap"
)

type Item struct {
	url string
}

type ProcessingQueue struct {
	items chan Item
	log   *zap.Logger
	wg    sync.WaitGroup
}

func NewProcessingQueue(buffer int, logger *zap.Logger) *ProcessingQueue {
	return &ProcessingQueue{
		items: make(chan Item, buffer),
		log:   logger,
	}
}

func (q *ProcessingQueue) Push(item Item) {
	//q.log.Debug("URL added to queue", zap.String("url", item.url))
	q.items <- item
}

func (q *ProcessingQueue) Run(ctx context.Context, processFunc func(item Item)) {
	count := 0
	for {
		select {
		case item, ok := <-q.items:
			if !ok {
				return
			}

			if count > 10 {
        break
			}
			q.wg.Add(1)

			go func() {
				defer q.wg.Done()
				processFunc(item)
        count++
			}()

		case <-ctx.Done():
			return
		}
	}
}

func (q *ProcessingQueue) Close() {
	close(q.items) //stops accepting
	q.wg.Wait()
}

package frontier

import "sync"

type HostQueue struct {
	items []*HostState
	head  int

	mu sync.Mutex
}

func NewHostQueue() *HostQueue {
	return &HostQueue{
		items: make([]*HostState, 0),
		head:  0,
	}
}

func (q *HostQueue) Push(host *HostState) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = append(q.items, host)
}

func (q *HostQueue) Pop() *HostState {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.head >= len(q.items) {
		return nil
	}

	host := q.items[q.head]
	q.items[q.head] = nil
	q.head++

	return host
}

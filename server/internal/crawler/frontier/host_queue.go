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

	if q.head == len(q.items) {
		q.items = q.items[:0]
		q.head = 0
	} else if q.head >= 1024 && q.head*2 >= len(q.items) {
		remaining := copy(q.items, q.items[q.head:])
		clear(q.items[remaining:])
		q.items = q.items[:remaining]
		q.head = 0
	}

	return host
}

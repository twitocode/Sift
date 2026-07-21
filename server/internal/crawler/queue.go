package crawler

import (
	"errors"
	"slices"
	"sync"
	"time"
)

type BQueue struct {
	Host       URL
	URLs       []URL
	Locked     bool
	StaleUntil time.Time

	mu sync.Mutex
}

func (b *BQueue) IsAvailable() bool {
	return !b.Locked && time.Now().After(b.StaleUntil)
}

func (b *BQueue) Dequeue() URL {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.URLs) == 0 {
		return ""
	}

	url := b.URLs[0]
	b.URLs = b.URLs[1:]
	return url
}

func (b *BQueue) Enqueue(newUrl URL) {
	b.mu.Lock()
	defer b.mu.Unlock()

	normalized := newUrl.normalizeString()
	if slices.Contains(b.URLs, normalized) {
		return
	}

	b.URLs = append(b.URLs, normalized)
}

func (b *BQueue) Timeout(milliseconds int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.StaleUntil = time.Now().Add(time.Millisecond * time.Duration(milliseconds))
}

func (b *BQueue) Lock() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.Locked = true
}

func (b *BQueue) TryUnlock() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !time.Now().After(b.StaleUntil) {
		return errors.New("Buffer queue not available yet")
	}
	b.Locked = true
	return nil
}

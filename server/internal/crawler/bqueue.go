package crawler

import (
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

type BQueue struct {
	Host            URL
	URLs            []URL
	Locked          bool
	StaleUntil      time.Time
	DiscoveredCount atomic.Int64
	SkippedCount    atomic.Int64

	mu sync.Mutex
}

func (b *BQueue) IsAvailable() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return !b.Locked && time.Now().After(b.StaleUntil)
}

func (b *BQueue) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()

	return len(b.URLs)
}

func (b *BQueue) Dequeue() URL {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.URLs) == 0 {
		return ""
	}

	url := b.URLs[0]
	b.URLs[0] = ""
	b.URLs = b.URLs[1:]
	return url
}

func (b *BQueue) Enqueue(newUrl URL, max int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.URLs) >= max {
		return false
	}

	normalized := newUrl.normalizeString()
	if slices.Contains(b.URLs, normalized) {
		return false
	}
	b.URLs = append(b.URLs, normalized)
	return true
}

func (b *BQueue) TryLock() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.Locked || !time.Now().After(b.StaleUntil) {
		return false
	}

	b.Locked = true
	b.StaleUntil = time.Time{}
	return true
}

func (b *BQueue) Unlock() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.Locked = false
}

func (b *BQueue) Timeout() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.StaleUntil = time.Now().Add(b.GetDelay())
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
	b.Locked = false
	return nil
}

func (b *BQueue) CanDelete(now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	return len(b.URLs) == 0 &&
		!b.Locked &&
		now.After(b.StaleUntil)
}

func (b *BQueue) GetDelay() time.Duration {
	const baseDelay = 2
	const growthFactor = 3
	const maxDelay = 300

	skipped := b.SkippedCount.Load()
	discovered := b.DiscoveredCount.Load()

	if discovered == 0 {
		return time.Second * baseDelay
	}

	skipRatio := skipped / discovered

	cooldown := baseDelay + (1 + skipRatio) ^ growthFactor
	cooldown = max(min(cooldown, maxDelay), baseDelay)

	return time.Second * time.Duration(cooldown)
}

package frontier

import (
	"errors"
	"math"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/twitocode/sift/internal/common"
)

type HostState struct {
	Host           common.URL
	URLs           []common.URL
	QueuedAt       []time.Time
	Locked         bool
	NextEligibleAt time.Time
	IsScheduled    atomic.Bool

	//stats
	DiscoveredCount atomic.Int64
	SkippedCount    atomic.Int64
	DispatchCount   atomic.Int64

	mu sync.Mutex
}

func (hs *HostState) IsAvailable() bool {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	return !hs.Locked && time.Now().After(hs.NextEligibleAt)
}

func (hs *HostState) Len() int {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	return len(hs.URLs)
}

func (hs *HostState) Dequeue() common.URL {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	if len(hs.URLs) == 0 {
		return ""
	}

	url := hs.URLs[0]
	hs.URLs[0] = ""
	hs.URLs = hs.URLs[1:]
	hs.QueuedAt = hs.QueuedAt[1:]
	return url
}

func (hs *HostState) Enqueue(newUrl common.URL, max int) bool {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	if len(hs.URLs) >= max {
		return false
	}

	normalized := newUrl.NormalizeString()
	if slices.Contains(hs.URLs, normalized) {
		return false
	}
	hs.URLs = append(hs.URLs, normalized)
	hs.QueuedAt = append(hs.QueuedAt, time.Now())
	return true
}

func (hs *HostState) TryLock() bool {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	if hs.Locked || !time.Now().After(hs.NextEligibleAt) {
		return false
	}

	hs.Locked = true
	hs.NextEligibleAt = time.Time{}
	return true
}

func (hs *HostState) Unlock() {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	hs.Locked = false
}

func (hs *HostState) HasWorkOrDeactivate() bool {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	if len(hs.URLs) > 0 {
		return true
	}

	hs.IsScheduled.Store(false)
	return false
}

func (hs *HostState) Timeout() {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	hs.NextEligibleAt = time.Now().Add(hs.GetDelay())
}

func (hs *HostState) CooldownUntil(t time.Time) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	hs.NextEligibleAt = t
}

func (hs *HostState) Lock() {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	hs.Locked = true
}

func (hs *HostState) TryUnlock() error {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	if !time.Now().After(hs.NextEligibleAt) {
		return errors.New("Buffer queue not available yet")
	}
	hs.Locked = false
	return nil
}

func (hs *HostState) CanDelete(now time.Time) bool {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	return len(hs.URLs) == 0 &&
		!hs.Locked &&
		now.After(hs.NextEligibleAt)
}

func (hs *HostState) GetDelay() time.Duration {
	const baseDelay = 2
	const growthFactor = 3
	const maxDelay = 300

	skipped := hs.SkippedCount.Load()
	discovered := hs.DiscoveredCount.Load()

	if discovered == 0 {
		return time.Second * baseDelay
	}

	skipRatio := float64(skipped) / float64(discovered)
	cooldown := baseDelay + math.Pow(1+skipRatio, growthFactor)
	cooldown = max(min(cooldown, maxDelay), baseDelay)

	return time.Second * time.Duration(cooldown)
}

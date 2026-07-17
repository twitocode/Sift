package crawler

import "sync"

type SafeMap[K comparable, V any] struct {
	m  map[K]V
	mu sync.RWMutex
}

func NewSafeMap[K comparable, V any]() *SafeMap[K, V] {
	return &SafeMap[K, V]{
		m: make(map[K]V),
	}
}

func (sm *SafeMap[K, V]) Get(key K) (V, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	v, ok := sm.m[key]
	return v, ok
}

func (sm *SafeMap[K, V]) Set(key K, value V) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.m[key] = value
}

func (sm *SafeMap[K, V]) Contains(key K) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	_, ok := sm.m[key]
	return ok
}

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

func (sm *SafeMap[K, V]) Range(callback func(k K, v V)) {
	sm.mu.RLock()
	for k, v := range sm.m {
		callback(k, v)
	}
	sm.mu.RUnlock()
}


func (sm *SafeMap[K, V]) Delete(k K) {
	sm.mu.Lock()
	delete(sm.m, k)
	sm.mu.Unlock()
}


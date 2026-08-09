package common

import (
	"maps"
	"slices"
	"sync"
)

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

func (sm *SafeMap[K, V]) Range(callback func(k K, v V) bool) {
	sm.mu.RLock()
	snapshot := make(map[K]V, len(sm.m))
	for k, v := range sm.m {
		snapshot[k] = v
	}
	sm.mu.RUnlock()

	for k, v := range snapshot {
		if !callback(k, v) {
			break
		}
	}
}

func (sm *SafeMap[K, V]) Delete(k K) {
	sm.mu.Lock()
	delete(sm.m, k)
	sm.mu.Unlock()
}

func (sm *SafeMap[K, V]) Keys() []K {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	keys := slices.Collect(maps.Keys(sm.m))
	return keys
}

func (sm *SafeMap[K, V]) Values() []V {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	values := slices.Collect(maps.Values(sm.m))
	return values
}

func (sm *SafeMap[K, V]) Length() int {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	keys := slices.Collect(maps.Keys(sm.m))
	return len(keys)
}

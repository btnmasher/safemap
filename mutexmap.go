package safemap

import (
	"errors"
	"sync"
)

// NewMutexMap returns a generic SafeMap underpinned by a Go map with common methods
// wrapped in a sync.RWMutex
//
// This map type is preferred for general use for concurrency-safe operations, but not for
// situations where there could be high read lock contention on an individual instance of
// this type of Map. For low write, high read contention variant of SafeMap, try NewSyncMap
func NewMutexMap[K comparable, V any]() SafeMap[K, V] {
	return &mutexMap[K, V]{
		m: make(map[K]V),
	}
}

type mutexMap[K comparable, V any] struct {
	mu sync.RWMutex
	m  map[K]V
}

func (cm *mutexMap[K, V]) Length() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.m)
}

func (cm *mutexMap[K, V]) Get(key K) (V, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	value, ok := cm.m[key]
	return value, ok
}

func (cm *mutexMap[K, V]) Set(key K, value V) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.m[key] = value
}

func (cm *mutexMap[K, V]) ChangeKey(oldKey K, newKey K) bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if value, exists := cm.m[oldKey]; exists {
		delete(cm.m, oldKey)
		cm.m[newKey] = value
		return true
	}
	return false
}

func (cm *mutexMap[K, V]) Delete(key K) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	delete(cm.m, key)
}

func (cm *mutexMap[K, V]) Exists(key K) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	_, ok := cm.m[key]
	return ok
}

func (cm *mutexMap[K, V]) Keys() []K {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	keys := make([]K, 0, len(cm.m))
	for k := range cm.m {
		keys = append(keys, k)
	}
	return keys
}

func (cm *mutexMap[K, V]) Values() []V {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	vals := make([]V, 0, len(cm.m))
	for _, v := range cm.m {
		vals = append(vals, v)
	}
	return vals
}

func (cm *mutexMap[K, V]) KeysIter() <-chan K {
	cm.mu.RLock()
	ch := make(chan K, len(cm.m))
	go func() {
		defer close(ch)
		defer cm.mu.RUnlock()
		for k := range cm.m {
			ch <- k
		}
	}()
	return ch
}

func (cm *mutexMap[K, V]) ValuesIter() <-chan V {
	cm.mu.RLock()
	ch := make(chan V, len(cm.m))
	go func() {
		defer close(ch)
		defer cm.mu.RUnlock()
		for _, v := range cm.m {
			ch <- v
		}
	}()
	return ch
}

func (cm *mutexMap[K, V]) ForEach(do func(K, V) error) error {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	var errs error
	for k, v := range cm.m {
		if err := do(k, v); err != nil {
			errs = errors.Join(errs, err)
		}
	}
	return errs
}

func (cm *mutexMap[K, V]) Clear() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	clear(cm.m)
}

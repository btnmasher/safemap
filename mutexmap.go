package safemap

import (
	"errors"
	"iter"
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

// Length returns the number of key/value pairs currently stored in the map.
// It acquires a read lock for the duration of the call.
func (cm *mutexMap[K, V]) Length() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.m)
}

// Get retrieves the value associated with the given key.
// The boolean result reports whether the key was found.
// It acquires a read lock for the duration of the call.
func (cm *mutexMap[K, V]) Get(key K) (V, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	value, ok := cm.m[key]
	return value, ok
}

// Set stores the value for the given key, overwriting any existing value.
// It acquires a write lock for the duration of the call.
func (cm *mutexMap[K, V]) Set(key K, value V) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.m[key] = value
}

// ChangeKey renames an existing entry from oldKey to newKey.
// Returns true if the oldKey was present and the move succeeded.
// It acquires a write lock for the duration of the call.
func (cm *mutexMap[K, V]) ChangeKey(oldKey, newKey K) bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if value, exists := cm.m[oldKey]; exists {
		delete(cm.m, oldKey)
		cm.m[newKey] = value
		return true
	}
	return false
}

// Delete removes the entry with the specified key, if it exists.
// It acquires a write lock for the duration of the call.
func (cm *mutexMap[K, V]) Delete(key K) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	delete(cm.m, key)
}

// Exists reports whether the given key is present in the map.
// It acquires a read lock for the duration of the call.
func (cm *mutexMap[K, V]) Exists(key K) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	_, ok := cm.m[key]
	return ok
}

// KeysSlice returns a slice of all keys currently in the map.
// It acquires a read lock for the duration of the call.
func (cm *mutexMap[K, V]) KeysSlice() []K {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	keys := make([]K, 0, len(cm.m))
	for key := range cm.m {
		keys = append(keys, key)
	}
	return keys
}

// ValuesSlice returns a slice of all values currently in the map.
// It acquires a read lock for the duration of the call.
func (cm *mutexMap[K, V]) ValuesSlice() []V {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	values := make([]V, 0, len(cm.m))
	for _, value := range cm.m {
		values = append(values, value)
	}
	return values
}

// KeysChan returns a buffered channel that yields all keys in the map.
// The channel is closed after all keys have been sent. A read lock is held
// for the entire iteration.
func (cm *mutexMap[K, V]) KeysChan() <-chan K {
	cm.mu.RLock()
	ch := make(chan K, len(cm.m))
	go func() {
		defer close(ch)
		defer cm.mu.RUnlock()
		for key := range cm.m {
			ch <- key
		}
	}()
	return ch
}

// ValuesChan returns a buffered channel that yields all values in the map.
// The channel is closed after all values have been sent. A read lock is held
// for the entire iteration.
func (cm *mutexMap[K, V]) ValuesChan() <-chan V {
	cm.mu.RLock()
	ch := make(chan V, len(cm.m))
	go func() {
		defer close(ch)
		defer cm.mu.RUnlock()
		for _, value := range cm.m {
			ch <- value
		}
	}()
	return ch
}

// ForEach invokes the provided function for each key/value pair in the map.
// If the function returns an error for any entry, all errors are joined and
// returned. It acquires a read lock for the duration of the iteration.
func (cm *mutexMap[K, V]) ForEach(do func(K, V) error) error {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	var errs error
	for key, value := range cm.m {
		if err := do(key, value); err != nil {
			errs = errors.Join(errs, err)
		}
	}
	return errs
}

// All returns an iterator over key/value pairs.
// Usage:
//
//	for key, value := range sm.All() {
//	    // ...
//	}
//
// It acquires a read lock for the duration of the iteration.
func (cm *mutexMap[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		cm.mu.RLock()
		defer cm.mu.RUnlock()
		for key, value := range cm.m {
			if !yield(key, value) {
				return
			}
		}
	}
}

// Keys returns an iterator over keys only.
// Usage:
//
//	for key := range sm.Keys() {
//	    // ...
//	}
//
// It acquires a read lock for the duration of the iteration.
func (cm *mutexMap[K, V]) Keys() iter.Seq[K] {
	return func(yield func(K) bool) {
		cm.mu.RLock()
		defer cm.mu.RUnlock()
		for key := range cm.m {
			if !yield(key) {
				return
			}
		}
	}
}

// Values returns an iterator over values only.
// Usage:
//
//	for value := range sm.Values() {
//	    // ...
//	}
//
// It acquires a read lock for the duration of the iteration.
func (cm *mutexMap[K, V]) Values() iter.Seq[V] {
	return func(yield func(V) bool) {
		cm.mu.RLock()
		defer cm.mu.RUnlock()
		for _, value := range cm.m {
			if !yield(value) {
				return
			}
		}
	}
}

// Clear removes all entries from the map.
// It acquires a write lock for the duration of the call.
func (cm *mutexMap[K, V]) Clear() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	clear(cm.m)
}

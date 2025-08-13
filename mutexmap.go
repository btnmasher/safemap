package safemap

import (
	"context"
	"errors"
	"iter"
	"maps"
	"slices"
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
func (sm *mutexMap[K, V]) Length() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return len(sm.m)
}

// Get retrieves the value associated with the given key.
// The boolean result reports whether the key was found.
// It acquires a read lock for the duration of the call.
func (sm *mutexMap[K, V]) Get(key K) (V, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	value, ok := sm.m[key]

	return value, ok
}

// Set stores the value for the given key, overwriting any existing value.
// It acquires a write lock for the duration of the call.
func (sm *mutexMap[K, V]) Set(key K, value V) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.m[key] = value
}

// ChangeKey renames an existing entry from oldKey to newKey.
// Returns true if the oldKey was present and the move succeeded.
// Returns false if oldKey missing or newKey occupied.
// It acquires a write lock for the duration of the call.
func (sm *mutexMap[K, V]) ChangeKey(oldKey, newKey K) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if oldKey == newKey { // no-op, but successful
		return true
	}

	value, ok := sm.m[oldKey]
	if !ok {
		return false
	}

	if _, clash := sm.m[newKey]; clash {
		return false
	}

	sm.m[newKey] = value
	delete(sm.m, oldKey)

	return true
}

// Delete removes the entry with the specified key, if it exists.
// Returns true if the key existed and was deleted.
// It acquires a write lock for the duration of the call.
func (sm *mutexMap[K, V]) Delete(key K) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.m[key]; exists {
		delete(sm.m, key)
		return true
	}

	return false
}

// Exists reports whether the given key is present in the map.
// It acquires a read lock for the duration of the call.
func (sm *mutexMap[K, V]) Exists(key K) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	_, ok := sm.m[key]

	return ok
}

// KeysSlice returns a slice of all keys currently in the map.
// It acquires a read lock for the duration of the call.
func (sm *mutexMap[K, V]) KeysSlice() []K {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return slices.Collect(maps.Keys(sm.m))
}

// ValuesSlice returns a slice of all values currently in the map.
// It acquires a read lock for the duration of the call.
func (sm *mutexMap[K, V]) ValuesSlice() []V {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return slices.Collect(maps.Values(sm.m))
}

// AllSlice returns a snapshot of key/value pairs.
func (sm *mutexMap[K, V]) AllSlice() []KeyValue[K, V] {
	sm.mu.RLock()
	snapshot := maps.Clone(sm.m)
	sm.mu.RUnlock()

	pairs := make([]KeyValue[K, V], 0, len(snapshot))
	for key, value := range snapshot {
		pairs = append(pairs, KeyValue[K, V]{Key: key, Value: value})
	}

	return pairs
}

// KeysChan returns a buffered channel that yields all keys in the map.
// The channel is closed after all keys have been sent.
// A read lock is held  during snapshotting of the current state of the
// map keys into the channel buffer.
func (sm *mutexMap[K, V]) KeysChan() <-chan K {
	sm.mu.RLock()
	ch := make(chan K, len(sm.m))

	go func() {
		defer close(ch)
		defer sm.mu.RUnlock()
		for key := range sm.m {
			ch <- key
		}
	}()

	return ch
}

// ValuesChan returns a buffered channel that yields all values in the map.
// The channel is closed after all values have been sent.
// A read lock is held  during snapshotting of the current state of the
// map values into the channel buffer.
func (sm *mutexMap[K, V]) ValuesChan() <-chan V {
	sm.mu.RLock()
	ch := make(chan V, len(sm.m))

	go func() {
		defer close(ch)
		defer sm.mu.RUnlock()
		for _, value := range sm.m {
			ch <- value
		}
	}()

	return ch
}

// AllChan returns a buffered channel of KeyValue pairs.
// The producer goroutine holds an RLock only long enough to fill the buffer,
// then unlocks and closes the channel. No extra slice allocation needed.
func (sm *mutexMap[K, V]) AllChan() <-chan KeyValue[K, V] {
	sm.mu.RLock()
	ch := make(chan KeyValue[K, V], len(sm.m))

	go func() {
		defer close(ch)
		defer sm.mu.RUnlock()
		for key, value := range sm.m {
			ch <- KeyValue[K, V]{Key: key, Value: value}
		}
	}()

	return ch
}

// ForEach invokes the provided function for each key/value pair in the map.
// If the function returns an error for any entry, all errors are joined and returned.
// A read lock is held for the duration of snapshotting the map prior to iteration.
func (sm *mutexMap[K, V]) ForEach(do func(K, V) error) error {
	sm.mu.RLock()
	snapshot := maps.Clone(sm.m) // shallow copy
	sm.mu.RUnlock()

	var errs error

	for key, value := range maps.All(snapshot) {
		if err := do(key, value); err != nil {
			errs = errors.Join(errs, err)
		}
	}
	return errs
}

// ForEachContext invokes the provided function for each key/value pair in the map.
// If the function returns an error for any entry, all errors are joined and returned.
// This function accepts a context for early cancellation of the iteration by the caller.
// A read lock is held for the duration of snapshotting the map prior to iteration.
func (sm *mutexMap[K, V]) ForEachContext(ctx context.Context, do func(K, V) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	sm.mu.RLock()
	snapshot := maps.Clone(sm.m)
	sm.mu.RUnlock()

	var errs error
	for key, value := range snapshot {
		if err := ctx.Err(); err != nil {
			return errors.Join(errs, err)
		}
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
// It acquires a read lock while snapshotting the current state
// of the map keys and values before yielding the iterator.
func (sm *mutexMap[K, V]) All() iter.Seq2[K, V] {
	sm.mu.RLock()
	snapshot := maps.Clone(sm.m)
	sm.mu.RUnlock()

	return maps.All(snapshot)
}

// Keys returns an iterator over keys only.
// Usage:
//
//	for key := range sm.Keys() {
//	    // ...
//	}
//
// It acquires a read lock while snapshotting the current state
// of the map keys before yielding the iterator.
func (sm *mutexMap[K, V]) Keys() iter.Seq[K] {
	sm.mu.RLock()
	snapshot := maps.Clone(sm.m)
	sm.mu.RUnlock()

	return maps.Keys(snapshot)
}

// Values returns an iterator over values only.
// Usage:
//
//	for value := range sm.Values() {
//	    // ...
//	}
//
// It acquires a read lock while snapshotting the current state
// of the map values before yielding the iterator.
func (sm *mutexMap[K, V]) Values() iter.Seq[V] {
	sm.mu.RLock()
	snapshot := maps.Clone(sm.m)
	sm.mu.RUnlock()

	return maps.Values(snapshot)
}

// Clear removes all entries from the map.
// It acquires a write lock for the duration of the call.
func (sm *mutexMap[K, V]) Clear() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	clear(sm.m)
}

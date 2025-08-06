package safemap

import (
	"errors"
	"iter"
	"sync"
	"sync/atomic"
)

type syncMap[K comparable, V any] struct {
	m   sync.Map
	len atomic.Uint32
}

// NewSyncMap returns a generic SafeMap underpinned by a sync.Map.
//
// The Map type is optimized for two common use cases: (1) when the entry for a given key is
// only ever written once but read many times, as in caches that only grow, or (2) when multiple
// goroutines read, write, and overwrite entries for disjointed sets of key
func NewSyncMap[K comparable, V any]() SafeMap[K, V] {
	return &syncMap[K, V]{
		m: sync.Map{},
	}
}

// Length returns the number of entries in the map.
func (sm *syncMap[K, V]) Length() int {
	return int(sm.len.Load())
}

// Get retrieves the value associated with the given key.
// The boolean result reports whether the key was found.
func (sm *syncMap[K, V]) Get(key K) (V, bool) {
	value, ok := sm.m.Load(key)
	if !ok {
		var zero V
		return zero, false
	}
	return value.(V), true
}

// Set stores the value for the given key, overwriting any existing value.
// If the key did not already exist, the length counter is incremented.
func (sm *syncMap[K, V]) Set(key K, value V) {
	if _, existed := sm.m.LoadOrStore(key, value); existed {
		sm.m.Store(key, value)
	} else {
		sm.len.Add(1)
	}
}

// ChangeKey renames an existing entry from oldKey to newKey.
// Returns true if oldKey was present and the move succeeded; false otherwise.
func (sm *syncMap[K, V]) ChangeKey(oldKey, newKey K) bool {
	actual, loaded := sm.m.LoadAndDelete(oldKey)
	if !loaded {
		return false
	}
	// Attempt to store under newKey; if present, restore oldKey and return false.
	if _, exists := sm.m.LoadOrStore(newKey, actual); exists {
		// restore
		sm.m.Store(oldKey, actual)
		return false
	}
	return true
}

// Delete removes the entry with the specified key, if it exists.
// If the key was present, the length counter is decremented.
func (sm *syncMap[K, V]) Delete(key K) {
	if _, loaded := sm.m.LoadAndDelete(key); loaded {
		sm.len.Add(^uint32(0))
	}
}

// Exists reports whether the given key is present in the map.
func (sm *syncMap[K, V]) Exists(key K) bool {
	_, ok := sm.m.Load(key)
	return ok
}

// KeysSlice returns a slice of all keys currently in the map.
func (sm *syncMap[K, V]) KeysSlice() []K {
	var keys []K
	sm.m.Range(func(key, _ any) bool {
		keys = append(keys, key.(K))
		return true
	})
	return keys
}

// ValuesSlice returns a slice of all values currently in the map.
func (sm *syncMap[K, V]) ValuesSlice() []V {
	var values []V
	sm.m.Range(func(_, value any) bool {
		values = append(values, value.(V))
		return true
	})
	return values
}

// KeysChan returns a channel that yields all keys in the map.
// The channel is closed after all keys have been sent.
func (sm *syncMap[K, V]) KeysChan() <-chan K {
	ch := make(chan K)
	go func() {
		sm.m.Range(func(k, _ any) bool {
			ch <- k.(K)
			return true
		})
		close(ch)
	}()
	return ch
}

// ValuesChan returns a channel that yields all values in the map.
// The channel is closed after all values have been sent.
func (sm *syncMap[K, V]) ValuesChan() <-chan V {
	ch := make(chan V)
	go func() {
		sm.m.Range(func(_, v any) bool {
			ch <- v.(V)
			return true
		})
		close(ch)
	}()
	return ch
}

// ForEach invokes the provided function for each key/value pair in the map.
// If the function returns an error for any entry, all errors are joined and returned.
func (sm *syncMap[K, V]) ForEach(do func(K, V) error) error {
	var errs error
	sm.m.Range(func(k, v any) bool {
		if err := do(k.(K), v.(V)); err != nil {
			errs = errors.Join(errs, err)
		}
		return true
	})
	return errs
}

// All returns an iterator over key/value pairs.
// Usage:
//
//	for k, v := range sm.All() {
//	    // ...
//	}
func (sm *syncMap[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		sm.m.Range(func(k, v any) bool {
			return yield(k.(K), v.(V))
		})
	}
}

// Keys returns an iterator over keys only.
// Usage:
//
//	for k := range sm.Keys() {
//	    // ...
//	}
func (sm *syncMap[K, V]) Keys() iter.Seq[K] {
	return func(yield func(K) bool) {
		sm.m.Range(func(k, _ any) bool {
			return yield(k.(K))
		})
	}
}

// Values returns an iterator over values only.
// Usage:
//
//	for v := range sm.Values() {
//	    // ...
//	}
func (sm *syncMap[K, V]) Values() iter.Seq[V] {
	return func(yield func(V) bool) {
		sm.m.Range(func(_, v any) bool {
			return yield(v.(V))
		})
	}
}

// Clear removes all entries from the map and resets the length counter.
func (sm *syncMap[K, V]) Clear() {
	sm.m.Range(func(k, _ any) bool {
		sm.m.Delete(k)
		return true
	})
	sm.len.Store(0)
}

package safemap

import (
	"errors"
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

func (sm *syncMap[K, V]) Length() int {
	return int(sm.len.Load())
}

func (sm *syncMap[K, V]) Get(key K) (V, bool) {
	value, ok := sm.m.Load(key)
	var val V
	if ok {
		val, ok = value.(V)
	}
	return val, ok
}

func (sm *syncMap[K, V]) Set(key K, value V) {
	if _, exists := sm.m.Swap(key, value); !exists {
		sm.len.Add(1)
	}
}

func (sm *syncMap[K, V]) ChangeKey(oldKey K, newKey K) bool {
	if value, exists := sm.m.LoadAndDelete(oldKey); exists {
		if _, present := sm.m.LoadOrStore(newKey, value); present {
			// new value exists, restore the old value
			sm.m.Store(oldKey, value)
			return false
		}
		return true
	}
	return false
}

func (sm *syncMap[K, V]) Delete(key K) {
	_, ok := sm.m.LoadAndDelete(key)
	if ok {
		sm.len.Add(^uint32(0))
	}
}

func (sm *syncMap[K, V]) Exists(key K) bool {
	_, ok := sm.m.Load(key)
	return ok
}

func (sm *syncMap[K, V]) Keys() []K {
	keys := make([]K, 0)
	sm.m.Range(func(key, _ any) bool {
		keys = append(keys, key.(K))
		return true
	})
	return keys
}

func (sm *syncMap[K, V]) Values() []V {
	values := make([]V, 0)
	sm.m.Range(func(_ any, value any) bool {
		values = append(values, value.(V))
		return true
	})
	return values
}

func (sm *syncMap[K, V]) KeysIter() <-chan K {
	ch := make(chan K)
	go func() {
		sm.m.Range(func(key, _ any) bool {
			ch <- key.(K)
			return true
		})
		close(ch)
	}()
	return ch
}

func (sm *syncMap[K, V]) ValuesIter() <-chan V {
	ch := make(chan V)
	go func() {
		sm.m.Range(func(_ any, value any) bool {
			ch <- value.(V)
			return true
		})
		close(ch)
	}()
	return ch
}

func (sm *syncMap[K, V]) ForEach(do func(K, V) error) error {
	var errs error
	sm.m.Range(func(key, value any) bool {
		if err := do(key.(K), value.(V)); err != nil {
			errs = errors.Join(errs, err)
		}
		return true
	})
	return errs
}

func (sm *syncMap[K, V]) Clear() {
	sm.m.Range(func(key, _ any) bool {
		sm.m.Delete(key)
		sm.len.Store(0)
		return true
	})
}

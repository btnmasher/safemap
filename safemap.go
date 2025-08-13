/*
   Copyright (c) 2025, btnmasher
   All rights reserved.
   Use of this source code is governed by a license
   that can be found in the LICENSE file.
*/

package safemap

import (
	"context"
	"iter"
)

// KeyValue is a simple pair for snapshotting or sorting.
type KeyValue[K comparable, V any] struct {
	Key   K
	Value V
}

// SafeMap defines a concurrency-safe map interface with common operations
type SafeMap[K comparable, V any] interface {
	// Classic
	Length() int
	Get(K) (V, bool)
	Set(K, V)
	ChangeKey(K, K) bool
	Delete(K) bool
	Exists(K) bool

	// Neat
	KeysSlice() []K
	ValuesSlice() []V
	AllSlice() []KeyValue[K, V]

	// Communiucative
	KeysChan() <-chan K
	ValuesChan() <-chan V
	AllChan() <-chan KeyValue[K, V]

	// Spicy
	ForEach(func(K, V) error) error
	ForEachContext(context.Context, func(K, V) error) error

	// Magical
	All() iter.Seq2[K, V]
	Keys() iter.Seq[K]
	Values() iter.Seq[V]

	// Fatal
	Clear()
}

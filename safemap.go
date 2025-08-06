/*
   Copyright (c) 2025, btnmasher
   All rights reserved.
   Use of this source code is governed by a license
   that can be found in the LICENSE file.
*/

package safemap

import "iter"

// SafeMap defines a concurrency-safe map interface with common operations
type SafeMap[K comparable, V any] interface {
	Length() int
	Get(K) (V, bool)
	Set(K, V)
	ChangeKey(K, K) bool
	Delete(K)
	Exists(K) bool
	KeysSlice() []K
	ValuesSlice() []V
	KeysChan() <-chan K
	ValuesChan() <-chan V
	ForEach(func(K, V) error) error
	All() iter.Seq2[K, V]
	Keys() iter.Seq[K]
	Values() iter.Seq[V]
	Clear()
}

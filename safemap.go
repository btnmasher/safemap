/*
   Copyright (c) 2025, btnmasher
   All rights reserved.
   Use of this source code is governed by a license
   that can be found in the LICENSE file.
*/

package safemap

type SafeMap[K comparable, V any] interface {
	Length() int
	Get(K) (V, bool)
	Set(K, V)
	ChangeKey(K, K) bool
	Delete(K)
	Exists(K) bool
	Keys() []K
	Values() []V
	KeysIter() <-chan K
	ValuesIter() <-chan V
	ForEach(func(K, V) error) error
	Clear()
}

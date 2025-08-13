package safemap

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

// ---- helpers ---------------------------------------------------------------

type mapFactory func() SafeMap[string, int]

var impls = []struct {
	name    string
	factory mapFactory
}{
	{"MutexMap", func() SafeMap[string, int] { return NewMutexMap[string, int]() }},
	{"SyncMap", func() SafeMap[string, int] { return NewSyncMap[string, int]() }},
}

func prepopulate(b *testing.B, f mapFactory, n int) (SafeMap[string, int], []string) {
	sm := f()
	keys := make([]string, n)
	for i := 0; i < n; i++ {
		k := "k" + fmt.Sprintf("%08d", i)
		keys[i] = k
		sm.Set(k, i)
	}
	return sm, keys
}

func rndKeys(n int) []string {
	r := rand.New(rand.NewSource(1))
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = "r" + strconv.Itoa(r.Int())
	}
	return out
}

const (
	popSmall = 1_000
	popMed   = 10_000
)

// Consume a channel fully to avoid compiler elision.
func drain[T any](ch <-chan T) (n int) {
	for range ch {
		n++
	}
	return
}

// ---- core operations -------------------------------------------------------

// Get (hit)
func BenchmarkGet_Hit(b *testing.B) {
	for _, impl := range impls {
		for _, pop := range []int{popSmall, popMed} {
			b.Run(fmt.Sprintf("%s/%d", impl.name, pop), func(b *testing.B) {
				sm, keys := prepopulate(b, impl.factory, pop)
				b.ReportAllocs()
				b.ResetTimer()

				var sum int
				for i := 0; i < b.N; i++ {
					v, ok := sm.Get(keys[i%pop])
					if !ok {
						b.Fatal("unexpected miss")
					}
					sum += v
				}
				_ = sum
			})
		}
	}
}

// Get (miss)
func BenchmarkGet_Miss(b *testing.B) {
	for _, impl := range impls {
		for _, pop := range []int{popSmall, popMed} {
			b.Run(fmt.Sprintf("%s/%d", impl.name, pop), func(b *testing.B) {
				sm, _ := prepopulate(b, impl.factory, pop)
				miss := rndKeys(pop) // guaranteed not present
				b.ReportAllocs()
				b.ResetTimer()

				var cnt int
				for i := 0; i < b.N; i++ {
					if _, ok := sm.Get(miss[i%pop]); ok {
						b.Fatal("unexpected hit")
					}
					cnt++
				}
				_ = cnt
			})
		}
	}
}

// Exists (hit)
func BenchmarkExists_Hit(b *testing.B) {
	for _, impl := range impls {
		for _, pop := range []int{popSmall, popMed} {
			b.Run(fmt.Sprintf("%s/%d", impl.name, pop), func(b *testing.B) {
				sm, keys := prepopulate(b, impl.factory, pop)
				b.ReportAllocs()
				b.ResetTimer()
				var cnt int
				for i := 0; i < b.N; i++ {
					if !sm.Exists(keys[i%pop]) {
						b.Fatal("unexpected miss")
					}
					cnt++
				}
				_ = cnt
			})
		}
	}
}

// Exists (miss)
func BenchmarkExists_Miss(b *testing.B) {
	for _, impl := range impls {
		for _, pop := range []int{popSmall, popMed} {
			b.Run(fmt.Sprintf("%s/%d", impl.name, pop), func(b *testing.B) {
				sm, _ := prepopulate(b, impl.factory, pop)
				miss := rndKeys(pop)
				b.ReportAllocs()
				b.ResetTimer()
				var cnt int
				for i := 0; i < b.N; i++ {
					if sm.Exists(miss[i%pop]) {
						b.Fatal("unexpected hit")
					}
					cnt++
				}
				_ = cnt
			})
		}
	}
}

// Set (overwrite existing)
func BenchmarkSet_Update(b *testing.B) {
	for _, impl := range impls {
		for _, pop := range []int{popSmall, popMed} {
			b.Run(fmt.Sprintf("%s/%d", impl.name, pop), func(b *testing.B) {
				sm, keys := prepopulate(b, impl.factory, pop)
				b.ReportAllocs()
				b.ResetTimer()

				for i := 0; i < b.N; i++ {
					sm.Set(keys[i%pop], i)
				}
			})
		}
	}
}

// Set (insert new)
func BenchmarkSet_New(b *testing.B) {
	for _, impl := range impls {
		b.Run(impl.name, func(b *testing.B) {
			sm := impl.factory()
			var ctr atomic.Int64
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				id := ctr.Add(1)
				sm.Set("n"+strconv.FormatInt(id, 10), i)
			}
		})
	}
}

// ChangeKey round-trip (always succeeds: a→b then b→a)
func BenchmarkChangeKey_RoundTrip(b *testing.B) {
	for _, impl := range impls {
		for _, pop := range []int{popSmall, popMed} {
			b.Run(fmt.Sprintf("%s/%d", impl.name, pop), func(b *testing.B) {
				sm, keys := prepopulate(b, impl.factory, pop)
				news := make([]string, len(keys))
				for i, k := range keys {
					news[i] = "x" + k
				}
				b.ReportAllocs()
				b.ResetTimer()

				for i := 0; i < b.N; i++ {
					idx := i % pop
					oldKey := keys[idx]
					newKey := news[idx]
					if !sm.ChangeKey(oldKey, newKey) {
						b.Fatal("expected success (old→new)")
					}
					if !sm.ChangeKey(newKey, oldKey) {
						b.Fatal("expected success (new→old)")
					}
				}
			})
		}
	}
}

// ChangeKey collision (always fails: move into an existing key)
func BenchmarkChangeKey_Collision(b *testing.B) {
	for _, impl := range impls {
		for _, pop := range []int{popSmall, popMed} {
			b.Run(fmt.Sprintf("%s/%d", impl.name, pop), func(b *testing.B) {
				sm, keys := prepopulate(b, impl.factory, pop)
				b.ReportAllocs()
				b.ResetTimer()

				for i := 0; i < b.N; i++ {
					src := keys[i%pop]
					dst := keys[(i+1)%pop] // guaranteed occupied
					if sm.ChangeKey(src, dst) {
						b.Fatal("expected collision failure")
					}
				}
			})
		}
	}
}

// Delete (hit, followed by reinsert to keep steady-state)
func BenchmarkDelete_Hit(b *testing.B) {
	for _, impl := range impls {
		for _, pop := range []int{popSmall, popMed} {
			b.Run(fmt.Sprintf("%s/%d", impl.name, pop), func(b *testing.B) {
				sm, keys := prepopulate(b, impl.factory, pop)
				b.ReportAllocs()
				b.ResetTimer()

				for i := 0; i < b.N; i++ {
					k := keys[i%pop]
					if !sm.Delete(k) {
						b.Fatal("expected delete hit")
					}
					sm.Set(k, i) // restore
				}
			})
		}
	}
}

// Delete (miss)
func BenchmarkDelete_Miss(b *testing.B) {
	for _, impl := range impls {
		for _, pop := range []int{popSmall, popMed} {
			b.Run(fmt.Sprintf("%s/%d", impl.name, pop), func(b *testing.B) {
				sm, _ := prepopulate(b, impl.factory, pop)
				miss := rndKeys(pop)
				b.ReportAllocs()
				b.ResetTimer()

				var cnt int
				for i := 0; i < b.N; i++ {
					if sm.Delete(miss[i%pop]) {
						b.Fatal("unexpected delete hit")
					}
					cnt++
				}
				_ = cnt
			})
		}
	}
}

// Length
func BenchmarkLength(b *testing.B) {
	for _, impl := range impls {
		for _, pop := range []int{popSmall, popMed} {
			b.Run(fmt.Sprintf("%s/%d", impl.name, pop), func(b *testing.B) {
				sm, _ := prepopulate(b, impl.factory, pop)
				b.ReportAllocs()
				b.ResetTimer()
				var s int
				for i := 0; i < b.N; i++ {
					s += sm.Length()
				}
				_ = s
			})
		}
	}
}

// Clear (with re-populate to amortize)
func BenchmarkClear(b *testing.B) {
	for _, impl := range impls {
		for _, pop := range []int{popSmall} {
			b.Run(fmt.Sprintf("%s/%d", impl.name, pop), func(b *testing.B) {
				sm := impl.factory()
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					// populate
					for j := 0; j < pop; j++ {
						sm.Set("k"+strconv.Itoa(j), j)
					}
					sm.Clear()
				}
			})
		}
	}
}

// ---- snapshots/iterators/channels -----------------------------------------

func BenchmarkKeysSlice(b *testing.B) {
	for _, impl := range impls {
		for _, pop := range []int{popSmall, popMed} {
			b.Run(fmt.Sprintf("%s/%d", impl.name, pop), func(b *testing.B) {
				sm, _ := prepopulate(b, impl.factory, pop)
				b.ReportAllocs()
				b.ResetTimer()
				var n int
				for i := 0; i < b.N; i++ {
					n += len(sm.KeysSlice())
				}
				_ = n
			})
		}
	}
}

func BenchmarkValuesSlice(b *testing.B) {
	for _, impl := range impls {
		for _, pop := range []int{popSmall, popMed} {
			b.Run(fmt.Sprintf("%s/%d", impl.name, pop), func(b *testing.B) {
				sm, _ := prepopulate(b, impl.factory, pop)
				b.ReportAllocs()
				b.ResetTimer()
				var n int
				for i := 0; i < b.N; i++ {
					n += len(sm.ValuesSlice())
				}
				_ = n
			})
		}
	}
}

func BenchmarkAllSlice(b *testing.B) {
	for _, impl := range impls {
		for _, pop := range []int{popSmall, popMed} {
			b.Run(fmt.Sprintf("%s/%d", impl.name, pop), func(b *testing.B) {
				sm, _ := prepopulate(b, impl.factory, pop)
				b.ReportAllocs()
				b.ResetTimer()
				var n int
				for i := 0; i < b.N; i++ {
					n += len(sm.AllSlice())
				}
				_ = n
			})
		}
	}
}

func BenchmarkKeysChan(b *testing.B) {
	for _, impl := range impls {
		for _, pop := range []int{popSmall, popMed} {
			b.Run(fmt.Sprintf("%s/%d", impl.name, pop), func(b *testing.B) {
				sm, _ := prepopulate(b, impl.factory, pop)
				b.ReportAllocs()
				b.ResetTimer()
				var n int
				for i := 0; i < b.N; i++ {
					n += drain(sm.KeysChan())
				}
				_ = n
			})
		}
	}
}

func BenchmarkValuesChan(b *testing.B) {
	for _, impl := range impls {
		for _, pop := range []int{popSmall, popMed} {
			b.Run(fmt.Sprintf("%s/%d", impl.name, pop), func(b *testing.B) {
				sm, _ := prepopulate(b, impl.factory, pop)
				b.ReportAllocs()
				b.ResetTimer()
				var n int
				for i := 0; i < b.N; i++ {
					n += drain(sm.ValuesChan())
				}
				_ = n
			})
		}
	}
}

func BenchmarkAllChan(b *testing.B) {
	for _, impl := range impls {
		for _, pop := range []int{popSmall, popMed} {
			b.Run(fmt.Sprintf("%s/%d", impl.name, pop), func(b *testing.B) {
				sm, _ := prepopulate(b, impl.factory, pop)
				b.ReportAllocs()
				b.ResetTimer()
				var n int
				for i := 0; i < b.N; i++ {
					n += drain(sm.AllChan())
				}
				_ = n
			})
		}
	}
}

func BenchmarkAllIterator(b *testing.B) {
	for _, impl := range impls {
		for _, pop := range []int{popSmall, popMed} {
			b.Run(fmt.Sprintf("%s/%d", impl.name, pop), func(b *testing.B) {
				sm, _ := prepopulate(b, impl.factory, pop)
				b.ReportAllocs()
				b.ResetTimer()
				var n int
				for i := 0; i < b.N; i++ {
					for _, _ = range sm.All() {
						n++
					}
				}
				_ = n
			})
		}
	}
}

func BenchmarkKeysIterator(b *testing.B) {
	for _, impl := range impls {
		for _, pop := range []int{popSmall, popMed} {
			b.Run(fmt.Sprintf("%s/%d", impl.name, pop), func(b *testing.B) {
				sm, _ := prepopulate(b, impl.factory, pop)
				b.ReportAllocs()
				b.ResetTimer()
				var n int
				for i := 0; i < b.N; i++ {
					for range sm.Keys() {
						n++
					}
				}
				_ = n
			})
		}
	}
}

func BenchmarkValuesIterator(b *testing.B) {
	for _, impl := range impls {
		for _, pop := range []int{popSmall, popMed} {
			b.Run(fmt.Sprintf("%s/%d", impl.name, pop), func(b *testing.B) {
				sm, _ := prepopulate(b, impl.factory, pop)
				b.ReportAllocs()
				b.ResetTimer()
				var n int
				for i := 0; i < b.N; i++ {
					for range sm.Values() {
						n++
					}
				}
				_ = n
			})
		}
	}
}

// ForEach (no context)
func BenchmarkForEach(b *testing.B) {
	for _, impl := range impls {
		for _, pop := range []int{popSmall, popMed} {
			b.Run(fmt.Sprintf("%s/%d", impl.name, pop), func(b *testing.B) {
				sm, _ := prepopulate(b, impl.factory, pop)
				b.ReportAllocs()
				b.ResetTimer()
				var cnt int
				for i := 0; i < b.N; i++ {
					err := sm.ForEach(func(key string, value int) error {
						cnt += value
						return nil
					})
					if err != nil {
						b.Fatal(err)
					}
				}
				_ = cnt
			})
		}
	}
}

// ForEachContext (no cancel)
func BenchmarkForEachContext_NoCancel(b *testing.B) {
	ctx := context.Background()
	for _, impl := range impls {
		for _, pop := range []int{popSmall, popMed} {
			b.Run(fmt.Sprintf("%s/%d", impl.name, pop), func(b *testing.B) {
				sm, _ := prepopulate(b, impl.factory, pop)
				b.ReportAllocs()
				b.ResetTimer()
				var cnt int
				for i := 0; i < b.N; i++ {
					err := sm.ForEachContext(ctx, func(key string, value int) error {
						cnt += value
						return nil
					})
					if err != nil {
						b.Fatal(err)
					}
				}
				_ = cnt
			})
		}
	}
}

// ForEachContext (cancel mid-iteration)
func BenchmarkForEachContext_CancelHalf(b *testing.B) {
	for _, impl := range impls {
		for _, pop := range []int{popMed} {
			b.Run(fmt.Sprintf("%s/%d", impl.name, pop), func(b *testing.B) {
				sm, _ := prepopulate(b, impl.factory, pop)
				b.ReportAllocs()
				b.ResetTimer()

				for i := 0; i < b.N; i++ {
					ctx, cancel := context.WithCancel(context.Background())
					var seen int
					err := sm.ForEachContext(ctx, func(key string, value int) error {
						seen++
						// cancel around halfway through typical size
						if seen == pop/2 {
							cancel()
						}
						return nil
					})
					// After caller cancel, ForEachContext should return ctx.Err() in its joined error set (or nil for mutex snapshot impl if cancel happens after iteration completes).
					if err == nil {
						// In mutex snapshot impl, if the map is small or iteration finishes before cancel is observed,
						// it's OK to get nil. Make cancel early to increase the chance:
						_ = seen
					}
				}
			})
		}
	}
}

// ---- parallelized hot paths -----------------------------------------------

// Parallel Get (hit)
func BenchmarkParallel_Get_Hit(b *testing.B) {
	for _, impl := range impls {
		for _, pop := range []int{popMed} {
			b.Run(fmt.Sprintf("%s/%d", impl.name, pop), func(b *testing.B) {
				sm, keys := prepopulate(b, impl.factory, pop)
				b.ReportAllocs()
				b.ResetTimer()
				var idx atomic.Int64
				b.RunParallel(func(pb *testing.PB) {
					local := int(idx.Add(1)) % pop
					for pb.Next() {
						_, _ = sm.Get(keys[local])
						local++
						if local == pop {
							local = 0
						}
					}
				})
			})
		}
	}
}

// Parallel Set (update)
func BenchmarkParallel_Set_Update(b *testing.B) {
	for _, impl := range impls {
		for _, pop := range []int{popMed} {
			b.Run(fmt.Sprintf("%s/%d", impl.name, pop), func(b *testing.B) {
				sm, keys := prepopulate(b, impl.factory, pop)
				b.ReportAllocs()
				b.ResetTimer()
				var idx atomic.Int64
				b.RunParallel(func(pb *testing.PB) {
					local := int(idx.Add(1)) % pop
					for pb.Next() {
						sm.Set(keys[local], local)
						local++
						if local == pop {
							local = 0
						}
					}
				})
			})
		}
	}
}

// Parallel Exists (hit)
func BenchmarkParallel_Exists_Hit(b *testing.B) {
	for _, impl := range impls {
		for _, pop := range []int{popMed} {
			b.Run(fmt.Sprintf("%s/%d", impl.name, pop), func(b *testing.B) {
				sm, keys := prepopulate(b, impl.factory, pop)
				b.ReportAllocs()
				b.ResetTimer()
				var idx atomic.Int64
				b.RunParallel(func(pb *testing.PB) {
					local := int(idx.Add(1)) % pop
					for pb.Next() {
						_ = sm.Exists(keys[local])
						local++
						if local == pop {
							local = 0
						}
					}
				})
			})
		}
	}
}

// ---- misc ------------------------------------------------------------------

func init() {
	// Avoid benchmark skew from timer granularity on some platforms
	rand.Seed(time.Now().UnixNano())
}

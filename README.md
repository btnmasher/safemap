# safemap

*Boilerplate reduction for concurrent maps*

Two flavors of (mostly) thread-safe maps for Go, unified behind a single interface:

- **MutexMap** – classic `map` with a `sync.RWMutex`. Fast, simple, boring (in a good way).
- **SyncMap** – wrapper around `sync.Map`. Great for “write once, read a zillion times” or disjoint key sets.

Pick your fighter, keep your code the same.

---

## Why this exists

Because sometimes you want:
- The convenience of **helpers** like `ChangeKey`, `ForEach`, and slices/channels/iterators of keys/values/**pairs**.
- A **single interface** so you can swap implementations without touching call sites.

Also because typing `mu.RLock()`/`mu.RUnlock()` for the 400th time is how keyboards die.

---

## Quick Start

```go
things := safemap.NewMutexMap[string, int]() // Or: safemap.NewSyncMap[string, int]()

things.Set("thing1", 123)

if things.Exists("thing1") {
    // woa
    if things.ChangeKey("thing1", "thing2") {
        // amazing!
    }
}

value, ok := things.Get("thing2")
if !ok {
    // bummer
}
fmt.Println("Got:", value)

// Slices
keys := things.KeysSlice()
fmt.Printf("keys len: %v", len(keys))

values := things.ValuessSlice()
fmt.Printf("keys len: %v", len(values))

all := things.AllSlice()
fmt.Printf("keys len: %v", len(all))

// Iterate (error-aware)
if err := things.ForEach(func(key string, value int) error {
    fmt.Printf("%s => %v\n", key, value)
    return nil
}); err != nil {
    fmt.Println("iteration problems:", err)
}

// Context-aware iteration (cancel early)
ctx, cancel := context.WithCancel(context.Background())
defer cancel()
err = things.ForEachContext(ctx, func(key string, value int) error {
    fmt.Printf("%s => %v\n", key, value)
    if value > 420 {
        cancel()
    }
    return nil
}); err != nil {
    fmt.Println("iteration problems:", err)
}

// Channels (closed when done)
for key := range things.KeysChan() {
    fmt.Println("key via chan:", key)
}
for value := range things.ValuesChan() {
    fmt.Println("value via chan:", value)
}
for kv := range things.AllChan() {
    fmt.Printf("pair via chan: %s => %v\n", kv.Key, kv.Value)
}

// Modern go iterators
for key, value := range things.All() {
    fmt.Println("iter pair:", key, value)
}
for key := range things.Keys() {
    fmt.Println("iter key:", key)
}
for value := range things.Values() {
    fmt.Println("iter value:", value)
}

if things.Delete("thing2") {
    // Goodbye!
}
```

---

## API (short version)

```go
type KeyValue[K comparable, V any] struct {
	Key   K
	Value V
}

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

	// Communicative
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
```

#### Highlights
- **`ChangeKey`**: rename a key atomically (**no overwrite** semantics).
- **`ForEach`/`ForEachContext`**: errors are collected with `errors.Join`; `ctx` lets you bail early.
- **Iterators** (`All`/`Keys`/`Values`): clean `for … range` style.
- **`AllSlice` / `AllChan`**: convenient pair snapshots or buffered streams.
- **Channels & slices**: consistent, buffered channels; `MutexMap` fills under `RLock` then unlocks; `SyncMap` snapshots to avoid producer blocking if the consumer stops.
---

## Choose your map

**`NewMutexMap[K, V]()`**
- Backed by a standard `map[K]V` with a `sync.RWMutex` for reduced boilerplate.
- Good general-purpose default.
- Predictable performance, especially when reads/writes contend on the *same* keys.
- **Semantics**: slice/channel/iterator helpers snapshot under a read lock and then operate without holding locks.

**`NewSyncMap[K, V]()`**
- Backed by `sync.Map`.
- Best when keys are mostly **write-once/read-many** or when many goroutines touch **disjoint key sets**.
- Includes an internal atomic length counter so `Length()` is O(1).
- **Semantics**: iterator helpers (`All`/`Keys`/`Values`) reflect a **live** view like `sync.Map.Range`; channel helpers take a **snapshot** first to avoid producer blocking if the consumer stops. This may mean large allocations depending on the size of the underlying map.
---

## Notes & Gotchas

- **Locking behavior (MutexMap)**: reads use an `RLock`. Iterators/slices snapshot before iterating. Channels are buffered to the snapshot size so sends don’t block or hold the lock long.
  This is a shallow copy, and thus once the lock is released prior to iteration
  If your value type is a reference type or contains references (pointer, slice, map, channel, func, or a struct/array containing any of those), mutating that underlying data
  on a different thread may cause a data race during iteration.
  If your values are plain values (ints, bools, strings, structs of only value fields, etc.), mutation/reassignment during iteration from any thread is safe.
- **`sync.Map` semantics (SyncMap)**: excels at write-once/read-many and disjoint key workloads; not ideal for heavy hot-key churn. Iteration observes a live view and may miss/include entries based on timing (per `Range` contract).
- **`Length()`**: exact for `MutexMap`. For `SyncMap`, `Length()` is **best-effort** and can **drift under races**. We increment on first successful `Set` of a new key and decrement on `Delete`. Concurrent `Set`/`Delete` on the same key (or overlapping operations) can momentarily over/under-count. If you need an exact count, do a `Range` and count.

### Contribution

IF for some reason you think this is at all even modestly okayish software and want to add to it, freely open an Issue or PR.

---

## License

See `LICENSE`. Short version: be cool.

---

## FAQ (slightly sarcastic edition)

**"Why not just use `map` + `RWMutex`?"**  
You can! This *is* that - plus a tidy interface and some ergonomic helpers you’d probably have ended up writing anyway.

**"Is this a cache?”**  
No. It’s a map. You provide the policy, it provides reduced boilerplate and utilities.

**"Compatibility promise?"**  
No. Though I will always bump the minor version and indicate breaking change(s) in the commit.

**"Benchmarks?"**  
I'm not making any claims that this is stupid fast or anything. It's for developer friendliness more than it is for speed.

But I did abuse an LLM to write some benchmarks, they're probably okay. See `benchmarks_test.go`

Use the one that matches your workload, your numbers.

---

Happy mapping. Try not to fight over the keys.

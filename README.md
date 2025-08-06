# safemap
Yet some more concurrency-safe map implementations for my own use cases

```go
// Can also be NewSyncMap backed by sync.Map, which has its own performance characterists
things := safemap.NewMutexMap[string, *SomeStruct]()  

things.Set("thing1", &SomeStruct{})

if things.Exists("thing1") {
    // woa
    if things.ChangeKey("thing1", "thing2") {
        // amazing!
    }
}

thing, ok := things.Get("thing1")
if !ok {
	// bummer
}

things.ForEach(func(key string, value *SomeStruct) {
	// fantastic!
})

```
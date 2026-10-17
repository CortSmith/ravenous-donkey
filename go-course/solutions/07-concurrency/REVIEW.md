# Module 07: Concurrency Review Guide

This guide covers readable vs. optimized solutions, trade-offs, and grading checklists for each exercise and the project.

## Exercise 1: Counter

### Readable Solution (counter.go)

**What it does**: Three counter types (Mutex-based, Atomic, Keyed) that are safe for concurrent access.

```go
// Counter with Mutex
type Counter struct {
    mu sync.Mutex
    n  int64
}

func (c *Counter) Inc() {
    c.Add(1)
}

func (c *Counter) Add(delta int64) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.n += delta
}

func (c *Counter) Value() int64 {
    c.mu.Lock()
    defer c.mu.Unlock()
    return c.n
}

var c Counter
var wg sync.WaitGroup
for range 100 {
	wg.Add(1)
	go func() { defer wg.Done(); c.Inc() }()
}
wg.Wait()
fmt.Println(c.Value()) // 100
```

**Why it's idiomatic**:
- Mutex protects a single `int64` field, not shared across methods.
- Uses `defer mu.Unlock()` to ensure the lock is always released.
- `Inc()` delegates to `Add()` to avoid duplication.

**Atomic alternative** (fast.go):
```go
type AtomicCounter struct {
    n atomic.Int64
}

func (c *AtomicCounter) Add(delta int64) {
    c.n.Add(delta)
}

func (c *AtomicCounter) Value() int64 {
    return c.n.Load()
}

var c AtomicCounter
var wg sync.WaitGroup
for range 100 {
	wg.Add(1)
	go func() { defer wg.Done(); c.Add(2) }()
}
wg.Wait()
fmt.Println(c.Value()) // 200
```

### Trade-off

- **Mutex**: clearer intent, easier to extend (e.g., add per-operation logging), supports arbitrary updates.
- **Atomic**: faster (no lock/unlock overhead), but only for single values; can't atomically update two related fields.

For a simple counter, atomic is measurably faster (benchmark shows ~4x throughput on high contention). Use atomic when performance matters and the interface is simple; use Mutex when clarity or complexity matter more.

### KeyedCounter notes

- The zero value must be usable: `c.m` is `nil` initially, so `Get()` must handle reading from `nil` maps (safe in Go).
- `Snapshot()` must return a copy; mutating the copy must not affect the counter.

### Grading Checklist (Counter)

- [ ] `Counter.Add()` and `Value()` correctly use `mu.Lock()` + `defer mu.Unlock()`.
- [ ] `AtomicCounter` uses `sync/atomic` (not a Mutex disguised as atomic).
- [ ] `KeyedCounter.Inc()` initializes the map if needed.
- [ ] `KeyedCounter.Snapshot()` returns a copy, not the live map.
- [ ] All three types have pointer receivers (required to avoid copying the Mutex).
- [ ] Tests pass with `-race` (no data races).
- [ ] `Inc()` and `Add()` are not nested (no deadlock if `Inc()` calls `Add()`).

---

## Exercise 2: Parallel Sum

### Readable Solution (parallelsum.go)

**What it does**: Splits a slice into chunks and sums each chunk in a goroutine.

```go
import "gocourse/solutions/07-concurrency/exercises/parallelsum"

var Chunks = parallelsum.Chunks

func Sum(nums []int64, workers int) int64 {
    if workers < 1 {
        workers = runtime.GOMAXPROCS(0)
    }
    chunks := Chunks(len(nums), workers)
    partials := make([]int64, len(chunks))  // one slot per goroutine

    var wg sync.WaitGroup
    for i, r := range chunks {
        wg.Add(1)
        go func() {
            defer wg.Done()
            var s int64
            for _, v := range nums[r.Lo:r.Hi] {
                s += v
            }
            partials[i] = s
        }()
    }
    wg.Wait()

    var total int64
    for _, p := range partials {
        total += p
    }
    return total
}

func main() {
	nums := make([]int64, 1_000_000)
	for i := range nums {
		nums[i] = int64(i)
	}
	fmt.Println(Sum(nums, 4)) // 499999500000
}
```

**Why it's idiomatic**:
- `wg.Add(1)` is in the parent goroutine before spawning (no race with `Wait()`).
- Each goroutine writes to its own `partials[i]`, so no contention.
- `wg.Wait()` acts as a happens-before point: all writes to `partials` are visible.
- Local accumulation (`var s int64`) before writing the shared slice is an optimization.

**Chunks implementation**:
- Divides [0, n) evenly among parts. If n % parts != 0, earlier ranges get the extra element.

### Optimizations (fast.go)

No significant optimization for this pattern; local accumulation is already done. Micro-optimizations like padding to cache line boundaries are not worth the code complexity.

### Trade-off

- **Local accumulation, then one shared write per goroutine**: avoids atomic operations and lock contention.
- **Atomic adds to a shared counter**: simpler code, slower (~2x overhead per increment due to atomic/memory barriers).

Use local accumulation here; it's both clear and fast.

### Grading Checklist (Parallel Sum)

- [ ] `Chunks(n, parts)` returns nil for n <= 0, otherwise parts non-overlapping ranges.
- [ ] `Chunks` lengths differ by at most one; earlier ranges are longer when needed.
- [ ] `Sum` calls `wg.Add(1)` before spawning (no race).
- [ ] `Sum` passes ranges correctly to chunk sums (correct slice bounds).
- [ ] `Sum` uses local accumulation (small variable s per goroutine).
- [ ] Each goroutine writes to its own `partials[i]` (no contention).
- [ ] `wg.Wait()` waits for all goroutines before summing partials.
- [ ] Tests pass with `-race` and `-count=3`.

---

## Exercise 3: Worker Pool

### Readable Solution (workerpool.go)

**What it does**: Applies a function to every element of a slice using a fixed number of worker goroutines, keeping results in order.

```go
import "gocourse/solutions/07-concurrency/exercises/workerpool"

var ErrInvalidWorkers = workerpool.ErrInvalidWorkers

// Map calls fn on every element of in using at most workers goroutines and
// returns the results in the same order as in: out[i] = fn(ctx, in[i]).
//
// If any call fails, Map cancels the context passed to the calls still
// running, starts no further calls, waits for every worker to return, and
// returns (nil, err) with the first error. If ctx is done before all calls
// finish, Map returns (nil, context.Cause(ctx)), which is ctx.Err() unless the
// caller canceled with a cause. Map never leaves goroutines running after it
// returns.
func Map[In, Out any](ctx context.Context, in []In, workers int, fn func(context.Context, In) (Out, error)) ([]Out, error) {
	if workers < 1 {
		return nil, ErrInvalidWorkers
	}
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	out := make([]Out, len(in))
	jobs := make(chan int) // indices into in; unbuffered so no job is "claimed" early

	var wg sync.WaitGroup
	for range min(workers, len(in)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				v, err := fn(ctx, in[i])
				if err != nil {
					cancel(err) // only the first cause sticks; later calls are no-ops
					return
				}
				out[i] = v // each index is written by exactly one worker
			}
		}()
	}

	// The calling goroutine is the producer, so it owns (and closes) jobs.
feed:
	for i := range in {
		select {
		case jobs <- i:
		case <-ctx.Done():
			break feed
		}
	}
	close(jobs)
	wg.Wait()

	if err := context.Cause(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func main() {
	double := func(ctx context.Context, n int) (int, error) {
		if n < 0 {
			return 0, fmt.Errorf("negative input %d", n)
		}
		return 2 * n, nil
	}
	fmt.Println(Map(context.Background(), []int{1, 2, 3, 4}, 2, double)) // [2 4 6 8] <nil>
	fmt.Println(Map(context.Background(), []int{1, -2, 3}, 2, double))   // [] negative input -2
}
```

**Why it's idiomatic**:
- `context.WithCancelCause` lets workers report errors back to the main goroutine.
- Unbuffered `jobs` channel: if a worker sends an error and cancels, new sends block immediately.
- Each output element is written by exactly one worker (no contention).
- The main goroutine is the producer; it owns and closes `jobs`.
- `select` on `jobs <- i` and `ctx.Done()` allows early exit if cancellation happens.

### Optimizations

No significant optimization beyond the design shown. The pattern is both clear and efficient.

### Trade-off

- **Cancel on first error**: simple, but stops immediately. Could collect all errors (with a mutex) and return them all at once.
- **Unbuffered jobs channel**: forces work to proceed in order (roughly); a buffered channel would allow more flexibility but also potential for goroutine leaks if the producer doesn't monitor cancellation.

For this exercise, cancel on first error is correct and clear. A production version might collect all errors.

### Grading Checklist (Worker Pool)

- [ ] Workers < 1 returns `ErrInvalidWorkers`.
- [ ] Spawns `min(workers, len(in))` goroutines (don't spawn more than items).
- [ ] `wg.Add(1)` is in the parent before spawning (no race).
- [ ] Each goroutine writes to its own `out[i]` (no contention).
- [ ] `jobs` channel is unbuffered (or buffered with care to avoid leaks).
- [ ] The main goroutine (producer) closes `jobs` after feeding all indices.
- [ ] Workers exit the loop when `jobs` closes (via `range jobs`).
- [ ] Cancellation stops feeding jobs and propagates to workers via `ctx.Done()`.
- [ ] No goroutines remain after `Map` returns (even on error or cancellation).
- [ ] Tests pass with `-race` and check no goroutine leaks via `runtime.NumGoroutine()`.

---

## Exercise 4: Pipeline

### Readable Solution (pipeline.go)

**What it does**: Builds a cancellable pipeline using generators, transformers, and mergers.

```go
func Generate(ctx context.Context, nums ...int) <-chan int {
    out := make(chan int)
    go func() {
        defer close(out)
        for _, n := range nums {
            select {
            case out <- n:
            case <-ctx.Done():
                return
            }
        }
    }()
    return out
}

func Square(ctx context.Context, in <-chan int) <-chan int {
    out := make(chan int)
    go func() {
        defer close(out)
        for {
            select {
            case x, ok := <-in:
                if !ok { return }
                select {
                case out <- x * x:
                case <-ctx.Done():
                    return
                }
            case <-ctx.Done():
                return
            }
        }
    }()
    return out
}

func Merge(ctx context.Context, cs ...<-chan int) <-chan int {
    out := make(chan int)
    var wg sync.WaitGroup
    for _, c := range cs {
        wg.Add(1)
        go func(c <-chan int) {
            defer wg.Done()
            for {
                select {
                case v, ok := <-c:
                    if !ok { return }
                    select {
                    case out <- v:
                    case <-ctx.Done():
                        return
                    }
                case <-ctx.Done():
                    return
                }
            }
        }(c)
    }
    go func() {
        wg.Wait()
        close(out)  // close once all senders are done
    }()
    return out
}

func SumSquares(ctx context.Context, nums []int, workers int) (int, error) {
    ctx, cancel := context.WithCancel(ctx)
    defer cancel()

    src := Generate(ctx, nums...)
    squares := make([]<-chan int, max(workers, 1))
    for i := range squares {
        squares[i] = Square(ctx, src)
    }

    total := 0
    for v := range Merge(ctx, squares...) {
        total += v
    }
    if err := ctx.Err(); err != nil {
        return 0, err
    }
    return total, nil
}

func main() {
	fmt.Println(SumSquares(context.Background(), []int{1, 2, 3, 4}, 3)) // 30 <nil>
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fmt.Println(SumSquares(ctx, []int{1, 2, 3}, 2)) // 0 context canceled
}
```

**Why it's idiomatic**:
- Each stage closes its output channel when it returns.
- Every blocking operation (send, receive) also checks `ctx.Done()`.
- `Merge` uses `sync.WaitGroup` to wait for all senders before closing the merged channel.
- A private context in `SumSquares` allows orchestration without affecting the caller's context.

### Key Pattern: Every Blocking Op Needs a Cancel Case

The most important lesson: in a concurrent pipeline, every `case` that could block must have a matching `case <-ctx.Done()`.

```go
func produce(ctx context.Context, out chan<- int, value int) {
	// BAD would be a send with no other case: it blocks forever if no one is
	// receiving and ctx is not done.
	//
	//	out <- value

	// GOOD: can exit on cancellation
	select {
	case out <- value:
	case <-ctx.Done():
		fmt.Println("gave up:", ctx.Err())
		return // no goroutine leak
	}
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	produce(ctx, make(chan int), 1) // nobody receives: gave up: context deadline exceeded
}
```

### Optimizations

No significant optimization. The design is both clear and efficient.

### Trade-off

- **Check `ctx.Done()` on every blocking operation**: code is longer but guarantees no leaks.
- **Ignore cancellation**: simpler code (fewer `select` statements), but goroutines may block forever.

Always check `ctx.Done()`. It's a requirement for correctness, not an optimization.

### Grading Checklist (Pipeline)

- [ ] `Generate` closes its output channel (via `defer close(out)`).
- [ ] `Generate` checks `ctx.Done()` before sending (no leak on early cancel).
- [ ] `Square` reads from input until it's closed (`x, ok := <-in; if !ok { return }`).
- [ ] `Square` checks `ctx.Done()` on both receive and send (no leak).
- [ ] `Merge` uses `sync.WaitGroup` to wait for all input channels to be drained.
- [ ] `Merge` spawns one goroutine per input channel (no goroutines lost).
- [ ] `Merge` closes output only after all senders are done (via `wg.Wait()`).
- [ ] `SumSquares` creates a private context for pipeline orchestration.
- [ ] `SumSquares` checks `ctx.Err()` to distinguish normal completion from cancellation.
- [ ] No goroutines leak on cancellation (test with `runtime.NumGoroutine()`).
- [ ] Tests pass with `-race` and timeouts to catch deadlocks.

---

## Exercise 5: Fetcher

### Readable Solution (fetcher.go)

**What it does**: Fetches many URLs concurrently, bounding in-flight fetches with a semaphore channel.

```go
import "gocourse/solutions/07-concurrency/exercises/fetcher"

type (
	FetchFunc = fetcher.FetchFunc
	Result    = fetcher.Result
)

// FetchAll fetches every URL in urls with at most limit calls to fetch in
// flight at any moment (limit < 1 is treated as 1). It returns one Result per
// URL, in the same order as urls. A failed fetch does not stop the others.
//
// If ctx is canceled, FetchAll starts no new fetches; the Result for every
// URL that was never started has Err == ctx.Err(). FetchAll waits for all
// started fetches to return before it returns.
func FetchAll(ctx context.Context, urls []string, limit int, fetch FetchFunc) []Result {
	results := make([]Result, len(urls))
	sem := make(chan struct{}, max(limit, 1)) // a token per in-flight fetch

	var wg sync.WaitGroup
	for i, u := range urls {
		results[i].URL = u

		// Acquire in the parent: this bounds the number of goroutines, not
		// just the number of busy ones, and lets us stop launching early.
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
		}
		// select picks randomly when both cases are ready, so re-check:
		// never start a fetch once ctx is done.
		if err := ctx.Err(); err != nil {
			for j := i; j < len(urls); j++ {
				results[j] = Result{URL: urls[j], Err: err}
			}
			break
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }() // release
			body, err := fetch(ctx, u)
			results[i].Body, results[i].Err = body, err // each goroutine owns one element
		}()
	}
	wg.Wait()
	return results
}

// Errors combines the failures in results into one error, each wrapped with
// its URL, using errors.Join. It returns nil if every fetch succeeded.
func Errors(results []Result) error {
	var errs []error
	for _, r := range results {
		if r.Err != nil {
			errs = append(errs, fmt.Errorf("fetch %s: %w", r.URL, r.Err))
		}
	}
	return errors.Join(errs...) // nil when errs is empty
}

func main() {
	fetch := func(ctx context.Context, url string) (string, error) {
		if strings.Contains(url, "bad") {
			return "", errors.New("connection refused")
		}
		return "<html>" + url + "</html>", nil
	}
	results := FetchAll(context.Background(), []string{"a.example", "bad.example", "c.example"}, 2, fetch)
	for _, r := range results {
		fmt.Printf("%s: %q %v\n", r.URL, r.Body, r.Err)
	}
	fmt.Println(Errors(results)) // fetch bad.example: connection refused
}
```

**Why it's idiomatic**:
- Semaphore channel bounds in-flight work. Buffer capacity = max concurrent.
- Acquire in parent before spawning: bounds goroutines themselves, not just busy ones.
- Check `ctx.Err()` after attempting to acquire to avoid starting a fetch if already canceled.
- Capture loop vars by passing them as function arguments.
- `errors.Join` combines multiple errors with `%w` wrapping.

### Optimizations (fast.go)

No significant optimization. The semaphore pattern is efficient.

### Trade-off

- **Acquire in parent**: simple, clear, bounds goroutines.
- **Acquire in goroutine**: slightly less obvious but equivalent (both bound concurrency).

Acquire in parent is clearer.

### Grading Checklist (Fetcher)

- [ ] Semaphore channel has capacity = `max(limit, 1)`.
- [ ] Acquire token in parent before spawning goroutine (bounds goroutines).
- [ ] Check `ctx.Err()` after attempting acquire to avoid starting canceled fetches.
- [ ] Goroutine acquires then immediately releases on deferred `<-sem`.
- [ ] Results are in the same order as URLs (results[i] corresponds to urls[i]).
- [ ] A failed fetch does not stop others (all URLs are attempted).
- [ ] On context cancellation, remaining unstarted URLs get `Err == ctx.Err()`.
- [ ] `Errors()` wraps each error with its URL and combines with `errors.Join()`.
- [ ] Tests pass with `-race` and timeouts.

---

## Project: Duplicate File Finder

### Readable Solution (dupfind.go)

**What it does**: Walks a directory, groups files by size, hashes files in size collisions concurrently with a bounded worker pool, and reports duplicates.

**Key functions**:

1. **HashFile(ctx, path)**: compute SHA-256 hash.
   ```go
   func HashFile(ctx context.Context, path string) (string, error) {
       if err := ctx.Err(); err != nil {
           return "", err
       }
       f, err := os.Open(path)
       if err != nil {
           return "", err
       }
       defer f.Close()

       h := sha256.New()
       if _, err := io.Copy(h, f); err != nil {
           return "", fmt.Errorf("hash %s: %w", path, err)
       }
       return hex.EncodeToString(h.Sum(nil)), nil
   }

   func main() {
   	f, _ := os.CreateTemp("", "hash-*")
   	defer os.Remove(f.Name())
   	f.WriteString("hello\n")
   	f.Close()
   	fmt.Println(HashFile(context.Background(), f.Name())) // 5891b5b5... <nil>
   }
   ```

2. **filesBySize(ctx, root, minSize)**: walk and group by size.
   ```go
   func filesBySize(ctx context.Context, root string, minSize int64) (map[int64][]string, error) {
       bySize := make(map[int64][]string)
       err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
           if err != nil { return err }
           if err := ctx.Err(); err != nil { return err }
           if !d.Type().IsRegular() { return nil }
           info, err := d.Info()
           if err != nil { return err }
           if info.Size() >= minSize {
               bySize[info.Size()] = append(bySize[info.Size()], path)
           }
           return nil
       })
       if err != nil {
           return nil, fmt.Errorf("walk %s: %w", root, err)
       }
       return bySize, nil
   }

   func main() {
   	dir, _ := os.MkdirTemp("", "bysize")
   	defer os.RemoveAll(dir)
   	for name, body := range map[string]string{"a": "12345", "b": "abcde", "c": "x", "empty": ""} {
   		os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644)
   	}
   	bySize, err := filesBySize(context.Background(), dir, 1)
   	fmt.Println(len(bySize[5]), len(bySize[1]), len(bySize[0]), err) // 2 1 0 <nil>
   }
   ```

3. **hashAll(ctx, paths, opts)**: bounded worker pool hashing.
   ```go
   import "gocourse/solutions/07-concurrency/project/dupfind"

   type Options = dupfind.Options

   var HashFile = dupfind.HashFile

   // hashAll hashes paths with a bounded worker pool and returns the hashes in
   // the same order as paths. It stops at the first error.
   func hashAll(ctx context.Context, paths []string, opts Options) ([]string, error) {
   	hash := opts.Hash
   	if hash == nil {
   		hash = HashFile
   	}
   	workers := opts.Workers
   	if workers < 1 {
   		workers = runtime.GOMAXPROCS(0)
   	}

   	ctx, cancel := context.WithCancelCause(ctx)
   	defer cancel(nil)

   	hashes := make([]string, len(paths))
   	jobs := make(chan int)
   	var wg sync.WaitGroup
   	for range min(workers, len(paths)) {
   		wg.Add(1)
   		go func() {
   			defer wg.Done()
   			for i := range jobs {
   				h, err := hash(ctx, paths[i])
   				if err != nil {
   					cancel(fmt.Errorf("hash %s: %w", paths[i], err))
   					return
   				}
   				hashes[i] = h
   			}
   		}()
   	}

   feed:
   	for i := range paths {
   		select {
   		case jobs <- i:
   		case <-ctx.Done():
   			break feed
   		}
   	}
   	close(jobs)
   	wg.Wait()

   	if err := context.Cause(ctx); err != nil {
   		return nil, err
   	}
   	return hashes, nil
   }

   func main() {
   	dir, _ := os.MkdirTemp("", "hashall")
   	defer os.RemoveAll(dir)
   	var paths []string
   	for _, name := range []string{"a", "b", "c"} {
   		p := filepath.Join(dir, name)
   		os.WriteFile(p, []byte("same"), 0o644)
   		paths = append(paths, p)
   	}
   	hashes, err := hashAll(context.Background(), paths, Options{Workers: 2})
   	fmt.Println(len(hashes), hashes[0] == hashes[2], err) // 3 true <nil>
   }
   ```

4. **Find**: orchestrates the whole process.
   ```go
   import "gocourse/solutions/07-concurrency/project/dupfind"

   type (
   	Options = dupfind.Options
   	Group   = dupfind.Group
   )

   var HashFile = dupfind.HashFile

   // Find walks root and returns every group of duplicate regular files.
   // Symlinks and other non-regular files are ignored. Groups are sorted by
   // Size descending, then by their first path; paths within a group are
   // sorted. Find returns a nil slice (and nil error) when there are no
   // duplicates.
   //
   // If walking or hashing fails, or ctx is canceled, Find stops hashing,
   // waits for its goroutines to exit, and returns the error.
   func Find(ctx context.Context, root string, opts Options) ([]Group, error) {
   	bySize, err := filesBySize(ctx, root, max(opts.MinSize, 1))
   	if err != nil {
   		return nil, err
   	}

   	var candidates []string
   	sizeOf := make(map[string]int64)
   	for size, paths := range bySize {
   		if len(paths) < 2 {
   			continue // a unique size cannot have a duplicate: never read it
   		}
   		for _, p := range paths {
   			candidates = append(candidates, p)
   			sizeOf[p] = size
   		}
   	}

   	hashes, err := hashAll(ctx, candidates, opts)
   	if err != nil {
   		return nil, err
   	}

   	type key struct {
   		size int64
   		hash string
   	}
   	byKey := make(map[key][]string)
   	for i, p := range candidates {
   		k := key{sizeOf[p], hashes[i]}
   		byKey[k] = append(byKey[k], p)
   	}

   	var groups []Group
   	for k, paths := range byKey {
   		if len(paths) < 2 {
   			continue
   		}
   		slices.Sort(paths)
   		groups = append(groups, Group{Size: k.size, Hash: k.hash, Paths: paths})
   	}
   	slices.SortFunc(groups, func(a, b Group) int {
   		return cmp.Or(
   			cmp.Compare(b.Size, a.Size), // larger files first
   			cmp.Compare(a.Paths[0], b.Paths[0]),
   		)
   	})
   	return groups, nil
   }

   // filesBySize walks root and groups regular files of at least minSize bytes
   // by size.
   func filesBySize(ctx context.Context, root string, minSize int64) (map[int64][]string, error) {
   	bySize := make(map[int64][]string)
   	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
   		if err != nil {
   			return err
   		}
   		if err := ctx.Err(); err != nil {
   			return err
   		}
   		if !d.Type().IsRegular() {
   			return nil // directories, symlinks, devices, sockets...
   		}
   		info, err := d.Info()
   		if err != nil {
   			return err
   		}
   		if info.Size() >= minSize {
   			bySize[info.Size()] = append(bySize[info.Size()], path)
   		}
   		return nil
   	})
   	if err != nil {
   		return nil, fmt.Errorf("walk %s: %w", root, err)
   	}
   	return bySize, nil
   }

   // hashAll hashes paths with a bounded worker pool and returns the hashes in
   // the same order as paths. It stops at the first error.
   func hashAll(ctx context.Context, paths []string, opts Options) ([]string, error) {
   	hash := opts.Hash
   	if hash == nil {
   		hash = HashFile
   	}
   	workers := opts.Workers
   	if workers < 1 {
   		workers = runtime.GOMAXPROCS(0)
   	}

   	ctx, cancel := context.WithCancelCause(ctx)
   	defer cancel(nil)

   	hashes := make([]string, len(paths))
   	jobs := make(chan int)
   	var wg sync.WaitGroup
   	for range min(workers, len(paths)) {
   		wg.Add(1)
   		go func() {
   			defer wg.Done()
   			for i := range jobs {
   				h, err := hash(ctx, paths[i])
   				if err != nil {
   					cancel(fmt.Errorf("hash %s: %w", paths[i], err))
   					return
   				}
   				hashes[i] = h
   			}
   		}()
   	}

   feed:
   	for i := range paths {
   		select {
   		case jobs <- i:
   		case <-ctx.Done():
   			break feed
   		}
   	}
   	close(jobs)
   	wg.Wait()

   	if err := context.Cause(ctx); err != nil {
   		return nil, err
   	}
   	return hashes, nil
   }

   func main() {
   	dir, _ := os.MkdirTemp("", "dupfind")
   	defer os.RemoveAll(dir)
   	for name, body := range map[string]string{"a.txt": "hello", "b.txt": "hello", "c.txt": "world", "d.txt": "much longer"} {
   		os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644)
   	}
   	groups, err := Find(context.Background(), dir, Options{})
   	for _, g := range groups {
   		fmt.Println(g.Size, len(g.Paths), filepath.Base(g.Paths[0]), filepath.Base(g.Paths[1]))
   	}
   	fmt.Println(err) // one group: 5 2 a.txt b.txt, then <nil>
   }
   ```

**Why it's idiomatic**:
- Two-stage grouping: cheap (size) before expensive (hash).
- Worker pool for hashing with bounded concurrency.
- Error propagation via `context.WithCancelCause`.
- `sync.WaitGroup` ensures all workers finish before checking for errors.
- Each output element (hash at index i) is written by exactly one worker (no contention).

### Optimizations

1. **Parallel hashing of large files**: read large files in chunks and hash in parallel (complex; not required).
2. **Memory-mapped files**: avoid buffering entire files in RAM (Go's `io.Copy` already uses bufio internally).
3. **Avoiding redundant stats**: already done (one stat per file in `WalkDir`).

The solution is already well-optimized for typical use cases.

### Trade-off

- **Two-stage (size, then hash)**: avoids hashing files with unique sizes; simplest and best for most directories.
- **Hash all files**: simpler code, slower if many files have unique sizes (wasted hash work).

Two-stage is correct and faster; use it.

### Grading Checklist (Project)

**dupfind.go**:
- [ ] `HashFile(ctx, path)` returns the correct SHA-256 hash (verify against `sha256sum`).
- [ ] `HashFile(ctx, path)` checks `ctx.Err()` early to support cancellation.
- [ ] `filesBySize` skips non-regular files (directories, symlinks, etc.).
- [ ] `filesBySize` respects `minSize` (empty files are always skipped, per Options.MinSize = 1 default).
- [ ] `hashAll` creates a worker pool with `min(workers, len(paths))` goroutines.
- [ ] `hashAll` uses unbuffered `jobs` channel and feeds indices.
- [ ] `hashAll` checks `ctx.Done()` when feeding jobs (graceful cancellation).
- [ ] `hashAll` preserves result order: hashes[i] = hash of paths[i].
- [ ] `hashAll` stops hashing on first error and propagates it.
- [ ] `Find` groups files by size first (optimization: avoid hashing unique-size files).
- [ ] `Find` only hashes files in size collisions (at least 2 files per size).
- [ ] `Find` groups by (size, hash) and returns only groups with >= 2 files.
- [ ] `Find` sorts groups by size descending, then by first path.
- [ ] `Find` sorts paths within each group alphabetically.
- [ ] `WriteReport` outputs the correct format (group N, file count, hash, paths, summary).
- [ ] `Wasted()` correctly computes Size * (len(Paths) - 1) per group.
- [ ] Tests pass with `-race` (no data races).
- [ ] No goroutine leaks on error or cancellation (checked via `runtime.NumGoroutine()`).
- [ ] Tests include: empty dir, no duplicates, duplicates, symlinks (skipped), MinSize filter, custom Hash, concurrency bounds.

**main.go**:
- [ ] Parses flags correctly: `-workers`, `-min-size`, optional directory.
- [ ] Defaults: workers=0 (GOMAXPROCS), min-size=1, directory=".".
- [ ] Uses `signal.NotifyContext` for graceful Ctrl+C.
- [ ] Exit codes: 0 on success, 1 on file system errors, 2 on invalid flags.
- [ ] Calls `dupfind.Find` and `WriteReport` correctly.
- [ ] Error messages go to stderr; report goes to stdout.
- [ ] Tests: finds duplicates, lists paths, filters by MinSize, handles bad flags, handles missing dir.

---

## Summary: Concurrency Best Practices

1. **Synchronous API, concurrent implementation**: let the caller decide to spawn goroutines.
2. **Every goroutine needs an exit condition**: channel close, `ctx.Done()`, `wg.Done()`, or `return`.
3. **Every blocking operation needs a matching cancellation check**: `select { case <-ch:; case <-ctx.Done(): return }`.
4. **One owner per resource**: one goroutine sends and closes a channel; one locks a Mutex; one adds to a WaitGroup.
5. **Acquire semaphore tokens in the parent** (not spawned goroutine) to bound goroutines, not just busy ones.
6. **Test with `-race` and timeouts**: find races and deadlocks.
7. **Avoid subtle closure bugs**: capture loop variables by passing them as arguments to spawned functions.
8. **Use `errors.Join` to combine multiple errors**: cleaner than `[]error` with manual formatting.
9. **Prefer unbuffered channels**: simpler reasoning; buffer only when you have a specific reason.
10. **Keep critical sections small**: no I/O or channel sends while holding a Mutex.

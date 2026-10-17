# Module 07: Goroutines, Channels & Sync

## Goals

After this module, you will be able to:

- Spawn and coordinate goroutines using `sync.WaitGroup`.
- Design concurrent programs using channels (unbuffered, buffered, closing rules).
- Handle context cancellation in goroutines and gracefully stop goroutines on shutdown.
- Use `sync.Mutex` and `sync.RWMutex` to protect shared state.
- Build worker pools, pipelines, and fan-out/fan-in patterns.
- Avoid goroutine leaks and data races (test with `-race`).
- Use `sync/atomic` for single-value concurrency and `sync.Once` for one-time initialization.

## Concepts

### Goroutines: Lightweight Concurrency

A goroutine is a lightweight thread managed by the Go runtime. Unlike OS threads, goroutines are cheap to spawn (thousands or millions can exist in one program). They run concurrently within a single process and share memory.

```go
func printHello() { fmt.Println("Hello from printHello") }

// Spawn a goroutine. Nothing waits for it, so it may never get to run
// before main returns: run this a few times.
go printHello()

// The main goroutine must wait for others (here using sync.WaitGroup)
var wg sync.WaitGroup
wg.Add(1)
go func() {
	defer wg.Done()
	fmt.Println("Hello from a goroutine")
}()
wg.Wait()
```

Key points:
- `go` keyword spawns a new goroutine that runs concurrently.
- The spawning goroutine does *not* wait; it continues immediately.
- A goroutine that sends on a channel or calls `wg.Done()` must always reach that point, even on errors.
- A program exits when `main` returns, killing all remaining goroutines. Use `sync.WaitGroup` to wait for them.

### Channels: Goroutine Communication

A channel is a typed, synchronized queue that goroutines use to send and receive values. It's the primary way to coordinate goroutines and avoid data races on shared memory.

**Unbuffered channels** block the sender until a receiver is ready:

```go
ch := make(chan int)  // unbuffered
go func() {
	ch <- 42  // blocks until the receiver is ready
}()
v := <-ch  // blocks until the sender sends
fmt.Println(v)
```

**Buffered channels** allow a limited number of sends without a receiver ready:

```go expect=runtime-error
ch := make(chan int, 2) // buffer capacity 2
ch <- 1                 // does not block
ch <- 2                 // does not block
ch <- 3                 // blocks! buffer is full, and no one will ever receive
// fatal error: all goroutines are asleep - deadlock!
```

**Closing channels**: a closed channel broadcasts to all receivers:

```go
ch := make(chan int, 1)
ch <- 1
close(ch)
// Receiving from a closed channel never blocks: buffered values come out
// first, then the zero value with ok=false
v, ok := <-ch
fmt.Println(v, ok) // 1 true
v, ok = <-ch
fmt.Println(v, ok) // 0 false
```

Important rules:
- Only the *sender* (the goroutine that owns the channel) should close it.
- Closing a channel signals "no more values; stop waiting."
- Receiving from a closed channel is safe; sending to a closed channel panics.
- `range ch` stops when the channel is closed.

### select: Waiting on Multiple Channels

`select` lets a goroutine wait on multiple channel operations, picking one at random when ready:

```go
ch1, ch2 := make(chan int), make(chan int)
go func() {
	time.Sleep(10 * time.Millisecond)
	ch2 <- 2
}()

select {
case v := <-ch1:
	fmt.Println("ch1:", v)
case v := <-ch2:
	fmt.Println("ch2:", v) // this one: ch2 is ready first
case <-time.After(1 * time.Second):
	fmt.Println("timeout")
}
```

Common patterns:
- **Timeout**: `case <-time.After(d):` to enforce a deadline.
- **Cancellation**: `case <-ctx.Done():` to exit when context is canceled.
- **Graceful shutdown**: `close(done)` to signal all waiting goroutines.

### Synchronization: WaitGroup, Mutex, RWMutex

**sync.WaitGroup** waits for a collection of goroutines to finish:

```go
var wg sync.WaitGroup
for i := range 10 {
	wg.Add(1)
	go func(i int) {
		defer wg.Done()
		fmt.Println("Worker", i)
	}(i)  // pass i; don't capture it
}
wg.Wait()
```

Gotcha: `wg.Add(1)` must be called *before* spawning the goroutine (in the parent goroutine), never inside the spawned goroutine. Otherwise a race exists between `Wait` and the late `Add`.

**sync.Mutex** protects shared data:

```go
var mu sync.Mutex
var count int
var wg sync.WaitGroup

for range 1000 {
	wg.Add(1)
	go func() {
		defer wg.Done()
		mu.Lock()
		defer mu.Unlock()
		count++
	}()
}
wg.Wait()
fmt.Println(count) // always 1000. Remove the Lock/Unlock and it usually isn't.
```

Rules:
- Always use `defer mu.Unlock()` to ensure the lock is released, even on panic.
- Keep critical sections small; do not do I/O or send on channels while holding a lock.
- `func (c *Counter) Value()` should not call other methods that lock the mutex; avoid nested locks.

**sync.RWMutex** allows many readers or one writer:

```go
var mu sync.RWMutex
var data int

// Many goroutines can call RLock simultaneously
func read() int {
	mu.RLock()
	defer mu.RUnlock()
	return data
}

// Only one goroutine can call Lock
func write(v int) {
	mu.Lock()
	defer mu.Unlock()
	data = v
}

var wg sync.WaitGroup
for i := range 5 {
	wg.Add(2)
	go func() { defer wg.Done(); write(i) }()
	go func() { defer wg.Done(); _ = read() }()
}
wg.Wait()
fmt.Println(read()) // one of 0..4: whichever write ran last
```

### Context: Deadlines and Cancellation

`context.Context` lets you cancel goroutines and set deadlines:

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()
done := make(chan struct{})

go func() {
	defer close(done)
	select {
	case <-ctx.Done():
		fmt.Println("Canceled:", ctx.Err())
		return
	case <-time.After(10 * time.Second):
		fmt.Println("Timeout")
	}
}()

// Cancel after 1 second
time.Sleep(1 * time.Second)
cancel()
<-done // wait, or main may exit before the goroutine prints
```

Rules:
- Pass `ctx` as the first argument to functions that respect cancellation.
- Every blocking operation (channel receive, I/O, `time.Sleep`) must also check `ctx.Done()`.
- `context.WithCancelCause(ctx)` lets the caller set a specific error message.

### Atomics and sync.Once

**sync/atomic** provides atomic operations on single values (no goroutines racing):

```go
var count atomic.Int64

count.Add(1)
count.Store(10)
v := count.Load()
fmt.Println(v) // 10

// sync/atomic only for single values; use Mutex for multiple related fields
```

**sync.Once** runs a function exactly once, even if called by many goroutines:

```go
func expensiveComputation() int {
	fmt.Println("computing...")
	return 42
}

var once sync.Once
var value int

for range 3 {
	once.Do(func() {
		value = expensiveComputation() // runs only the first time
	})
}
fmt.Println(value) // computing... once, then 42
```

Use `sync.OnceValue` (Go 1.24) to cache the result:

```go
func expensiveComputation() int {
	fmt.Println("computing...")
	return 42
}

var getValue = sync.OnceValue(expensiveComputation)

v := getValue() // calls expensive once; subsequent calls return the cached value
v += getValue()
fmt.Println(v) // computing... once, then 84
```

### Common Patterns

**Worker Pool**: fixed number of goroutines process jobs from a channel:

```go
func process(job int) int { return job * job }

var wg sync.WaitGroup
var mu sync.Mutex
sum := 0

jobs := make(chan int)
for range 5 {
	wg.Add(1)
	go func() {
		defer wg.Done()
		for job := range jobs { // blocks until jobs is closed
			r := process(job)
			mu.Lock()
			sum += r
			mu.Unlock()
		}
	}()
}
// Feed jobs from another goroutine
go func() {
	for i := range 100 {
		jobs <- i
	}
	close(jobs) // signal workers to stop
}()
wg.Wait()
fmt.Println(sum) // 328350
```

**Bounded Concurrency via Semaphore**: a buffered channel of `struct{}` tokens:

```go
func work(i int) { time.Sleep(10 * time.Millisecond) }

var wg sync.WaitGroup
var running, peak atomic.Int32

sem := make(chan struct{}, 3) // max 3 concurrent operations
for i := range 20 {
	wg.Add(1)
	go func(i int) {
		defer wg.Done()
		sem <- struct{}{}        // acquire token; blocks if buffer full
		defer func() { <-sem }() // release token
		n := running.Add(1)
		for p := peak.Load(); n > p && !peak.CompareAndSwap(p, n); p = peak.Load() {
		}
		work(i)
		running.Add(-1)
	}(i)
}
wg.Wait()
fmt.Println("peak concurrency:", peak.Load()) // 3
```

**Pipeline**: each stage owns its output channel and closes it:

```go
func generate(nums ...int) <-chan int {
	ch := make(chan int)
	go func() {
		defer close(ch)
		for _, n := range nums {
			ch <- n
		}
	}()
	return ch
}

func square(in <-chan int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for n := range in {
			out <- n * n
		}
	}()
	return out
}

nums := generate(1, 2, 3)
squares := square(nums)
for v := range squares {
	fmt.Println(v)
}
```

**Fan-Out / Fan-In**: one source broadcasts to many workers; workers report back:

```go
// worker squares each job it receives.
func worker(jobs <-chan int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for j := range jobs {
			out <- j * j
		}
	}()
	return out
}

// Fan-out: distribute work to multiple workers
func distribute(jobs <-chan int, workers int) []<-chan int {
	out := make([]<-chan int, workers)
	for i := range workers {
		out[i] = worker(jobs)
	}
	return out
}

// Fan-in: merge outputs
func merge(cs ...<-chan int) <-chan int {
	out := make(chan int)
	var wg sync.WaitGroup
	for _, c := range cs {
		wg.Add(1)
		go func(c <-chan int) {
			defer wg.Done()
			for v := range c {
				out <- v
			}
		}(c)
	}
	go func() {
		wg.Wait()
		close(out) // close once all sources are done
	}()
	return out
}

func main() {
	jobs := make(chan int)
	go func() {
		for i := 1; i <= 10; i++ {
			jobs <- i
		}
		close(jobs)
	}()
	sum := 0
	for v := range merge(distribute(jobs, 3)...) {
		sum += v
	}
	fmt.Println(sum) // 385: every square, whichever worker computed it
}
```

### Race Detector

Enable the race detector to find data races at runtime:

```bash
go test -race ./...
go run -race main.go
```

A data race is any unsynchronized concurrent access to memory where at least one access is a write. The race detector is a powerful tool but cannot find all races (it only instruments code that runs).

Example race:

```go
// BAD: race condition. The WaitGroup makes main wait; it does NOT
// synchronize the two writes to count.
var count int
var wg sync.WaitGroup
wg.Add(2)
go func() { defer wg.Done(); count++ }()
go func() { defer wg.Done(); count++ }()
wg.Wait()
fmt.Println(count) // usually 2, sometimes 1; `go run -race` reports the DATA RACE
```

Fixed with a mutex:

```go
// GOOD: no race
var mu sync.Mutex
var count int
var wg sync.WaitGroup
wg.Add(2)
go func() { defer wg.Done(); mu.Lock(); count++; mu.Unlock() }()
go func() { defer wg.Done(); mu.Lock(); count++; mu.Unlock() }()
wg.Wait()
fmt.Println(count) // always 2
```

### Goroutine Leaks

A goroutine leak occurs when a goroutine never exits, wasting resources. Common causes:

1. **Blocked send**: a goroutine sends on a channel but no one receives.
   ```go
   ch := make(chan int)  // unbuffered
   go func() { ch <- 1 }()  // blocks forever; main exits before receiving

   time.Sleep(10 * time.Millisecond)
   fmt.Println("goroutines still alive:", runtime.NumGoroutine()) // 2: main and the stuck sender
   ```

2. **Blocked receive**: a goroutine waits on a channel that never sends.
   ```go
   ch := make(chan int)
   go func() { <-ch }()  // blocks forever

   time.Sleep(10 * time.Millisecond)
   fmt.Println("goroutines still alive:", runtime.NumGoroutine()) // 2: main and the stuck receiver
   ```

3. **Missing context check**: a goroutine loops forever ignoring `ctx.Done()`.
   ```go
   go func() {
       for {  // never exits on cancellation
           time.Sleep(1 * time.Second)
       }
   }()

   time.Sleep(10 * time.Millisecond)
   fmt.Println("goroutines still alive:", runtime.NumGoroutine()) // 2, and the loop never ends
   ```

Fix: every blocking operation needs a matching cancellation or completion signal.

## Idioms & Best Practices

- **Synchronous APIs, concurrent implementations**: prefer a synchronous function that the caller can wrap in `go` to add concurrency, rather than embedding goroutines in the function.
  ```go
  type Item int
  type Result int

  // GOOD: caller decides concurrency
  func Process(item Item) Result { return Result(item * 2) }

  // BAD: function spawns its own goroutines; caller has no control
  func ProcessAsync(item Item) <-chan Result {
  	out := make(chan Result, 1)
  	go func() { out <- Process(item) }()
  	return out
  }

  fmt.Println(Process(1)) // synchronous; the caller can still write `go Process(item)`
  var wg sync.WaitGroup
  for _, it := range []Item{2, 3} {
  	wg.Add(1)
  	go func() { defer wg.Done(); _ = Process(it) }() // the caller chose concurrency
  }
  wg.Wait()
  fmt.Println(<-ProcessAsync(4)) // works, but every call costs a goroutine and a channel
  ```

- **Goroutines are owned by their spawner**: the goroutine should have a clear exit condition (channel close, `ctx.Done()`, `wg.Done()`) and the spawner should wait for it.

- **Never do I/O or send on channels while holding a mutex**: keep critical sections short and lock-free.
  ```go
  type config struct{ URL string }

  var mu sync.Mutex
  cfg := &config{URL: "https://example.com"}
  fetchFromNetwork := func(url string) string {
  	time.Sleep(50 * time.Millisecond) // slow!
  	return "body of " + url
  }

  // BAD: every other goroutine that needs mu waits 50ms
  mu.Lock()
  result := fetchFromNetwork(cfg.URL)
  mu.Unlock()

  // GOOD
  mu.Lock()
  url := cfg.URL // fast
  mu.Unlock()
  result = fetchFromNetwork(url) // slow, but no lock held
  fmt.Println(result)
  ```

- **Unbuffered channels are simpler**: use them by default. Buffered channels are an optimization, not the norm.

- **Close only from the sender**: if multiple goroutines send on a channel, use a coordination channel or `sync.Once` to close.
  ```go
  items := []string{"a", "b", "c"}
  jobs := make(chan string)
  done := make(chan struct{})

  // One receiver
  go func() {
  	defer close(done)
  	for job := range jobs {
  		fmt.Println("processed", job)
  	}
  }()

  // GOOD: the one sender (main) owns jobs and closes it when done
  for _, it := range items {
  	jobs <- it
  }
  close(jobs)
  <-done
  ```

- **Prefer context.WithCancelCause for error propagation**: set a cause when canceling to give downstream goroutines a reason.
  ```go
  ctx, cancel := context.WithCancelCause(context.Background())
  err := errors.New("disk full")
  // ...
  cancel(fmt.Errorf("worker failed: %w", err))

  // Downstream can call context.Cause(ctx) to get the error
  <-ctx.Done()
  fmt.Println(ctx.Err())          // context canceled
  fmt.Println(context.Cause(ctx)) // worker failed: disk full
  ```

- **Test with `-race` and timeouts**: concurrency tests must both verify correctness and confirm no leaks.
  ```go
  func TestConcurrent(t *testing.T) {
  	// t.Context() is canceled when the test ends; add your own deadline on top
  	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
  	defer cancel()

  	results := make(chan int)
  	go func() {
  		select {
  		case results <- 42:
  		case <-ctx.Done(): // never leaks, even if the test stops listening
  		}
  	}()
  	select {
  	case v := <-results:
  		t.Logf("got %d", v)
  	case <-ctx.Done():
  		t.Fatal("timed out:", ctx.Err())
  	}
  }
  ```

## Common Pitfalls

- **Forgetting `defer wg.Done()`**: a goroutine that doesn't call `Done()` never unblocks `Wait()`.
  ```go expect=runtime-error
  doWork := func() { fmt.Println("working") }

  var wg sync.WaitGroup
  wg.Add(1)
  go func() {
  	// defer wg.Done()  // MISSING!
  	doWork()
  }()
  wg.Wait() // hangs forever: here the runtime notices and reports a deadlock
  ```

- **Calling `wg.Add` inside the goroutine**: races with `Wait()`.
  ```go
  doWork := func() { fmt.Println("work done") }

  var wg sync.WaitGroup
  go func() {
  	wg.Add(1) // BAD: races with Wait()
  	defer wg.Done()
  	doWork()
  }()
  wg.Wait() // usually returns at once, before the goroutine even starts
  fmt.Println("Wait returned") // "work done" usually never prints
  ```

- **Using `time.Sleep` for synchronization in tests**: flaky. Use channels or `sync.WaitGroup`.
  ```go
  worker := func() { time.Sleep(10 * time.Millisecond); fmt.Println("worker finished") }

  // BAD: flakey, may timeout on slow systems
  go worker()
  time.Sleep(100 * time.Millisecond)

  // GOOD: waits for work to actually finish
  var wg sync.WaitGroup
  wg.Add(1)
  go func() { defer wg.Done(); worker() }()
  wg.Wait()
  ```

- **Calling `t.Fatal` from a spawned goroutine**: not allowed (it stops only that goroutine, not the test). Use `t.Error` and send the error back on a channel.
  ```go
  func doWork() error { return nil }

  func TestWorker(t *testing.T) {
  	// BAD (go vet: call to (*T).Fatal from a non-test goroutine)
  	//
  	//	go func() { t.Fatal("error") }()

  	// GOOD
  	done := make(chan error, 1)
  	go func() { done <- doWork() }()
  	if err := <-done; err != nil {
  		t.Fatal(err)
  	}
  }
  ```

- **Copying a `sync.Mutex` or `sync.WaitGroup`**: the copy loses synchronization. Pass by pointer or embed in a struct.
  ```go
  type Counter struct {
  	mu sync.Mutex
  	n  int
  }

  // BAD
  c := &Counter{}
  c.mu.Lock()
  c2 := *c                     // copy loses sync (go vet: assignment copies lock value)
  fmt.Println(c2.mu.TryLock()) // false: the copy is a locked mutex nobody will ever unlock
  c.mu.Unlock()

  // GOOD: always use pointers
  c3 := c // pointer copy is fine: one shared mutex
  c3.mu.Lock()
  c3.n++
  c3.mu.Unlock()
  fmt.Println(c.n) // 1
  ```

- **Setting a channel variable to `nil` by accident**: this disables its `select` case, but is a mistake if unintended.
  ```go
  // The deliberate use: a nil channel blocks forever, so setting a closed
  // channel to nil disables its select case.
  a, b := make(chan int), make(chan int)
  go func() { a <- 1; close(a) }()
  go func() { b <- 2; b <- 3; close(b) }()

  for a != nil || b != nil {
  	select {
  	case v, ok := <-a:
  		if !ok {
  			a = nil // a is done: stop selecting on it
  			continue
  		}
  		fmt.Println("a", v)
  	case v, ok := <-b:
  		if !ok {
  			b = nil
  			continue
  		}
  		fmt.Println("b", v)
  	}
  }
  // By accident, the same thing makes a case silently never fire.
  ```

- **Not checking `ctx.Err()` in loops**: a goroutine may receive buffered values even after `ctx.Done()` is closed; check both.
  ```go
  func consume(ctx context.Context, in <-chan int) {
  	for {
  		select {
  		case v, ok := <-in:
  			if !ok {
  				return
  			}
  			fmt.Println("got", v)
  		case <-ctx.Done():
  			fmt.Println("stopping:", ctx.Err())
  			return // stop even if input is still open
  		}
  	}
  }

  ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
  defer cancel()
  in := make(chan int)
  go func() { in <- 1 }() // one value, and then in stays open forever
  consume(ctx, in)
  ```

## Exercises

### Exercise 1: Concurrent-Safe Counter (5-10 min)

**Exercise name**: `counter`

**Directory**: `modules/07-concurrency/exercises/counter/`

**Implement three types**:
1. `Counter`: guarded by a mutex, with `Inc()`, `Add(delta)`, `Value()`.
2. `AtomicCounter`: using `sync/atomic`, same interface.
3. `KeyedCounter`: counts per string key with `Inc(key)`, `Get(key)`, `Snapshot()`.

All must be safe for concurrent use. Test with `-race`.

Run: `go test -race ./modules/07-concurrency/exercises/counter/`

### Exercise 2: Parallel Sum (5-10 min)

**Exercise name**: `parallelsum`

**Directory**: `modules/07-concurrency/exercises/parallelsum/`

**Implement**:
1. `Chunks(n, parts)`: splits indices [0, n) into parts contiguous ranges.
2. `Sum(nums, workers)`: sums `nums` by splitting into chunks and summing each in a goroutine using `sync.WaitGroup`.

Run: `go test ./modules/07-concurrency/exercises/parallelsum/`

### Exercise 3: Worker Pool (10-15 min)

**Exercise name**: `workerpool`

**Directory**: `modules/07-concurrency/exercises/workerpool/`

**Implement** `Map[In, Out](ctx, in, workers, fn)`: applies `fn` to every element of `in` using a pool of `workers` goroutines, returning results in order. Cancels on error or context cancellation with no goroutine leaks.

Run: `go test -race ./modules/07-concurrency/exercises/workerpool/`

### Exercise 4: Pipeline with Cancellation (15-20 min)

**Exercise name**: `pipeline`

**Directory**: `modules/07-concurrency/exercises/pipeline/`

**Implement**:
1. `Generate(ctx, nums)`: yields values from `nums` on a channel.
2. `Square(ctx, in)`: reads from `in`, squares each value, yields on output.
3. `Merge(ctx, cs)`: fans in values from multiple channels.
4. `SumSquares(ctx, nums, workers)`: builds a pipeline (generate -> fan-out to workers squarers -> merge -> sum), canceling gracefully.

Each stage must check `ctx.Done()` on every blocking operation. No goroutine leaks.

Run: `go test -race ./modules/07-concurrency/exercises/pipeline/`

### Exercise 5: Bounded Fetcher (10-15 min)

**Exercise name**: `fetcher`

**Directory**: `modules/07-concurrency/exercises/fetcher/`

**Implement**:
1. `FetchAll(ctx, urls, limit, fetch)`: fetches each URL using a semaphore channel to bound concurrent fetches to `limit`.
2. `Errors(results)`: combines fetch errors into one error via `errors.Join`.

Results are in order; a failed fetch doesn't stop others. No leaks on cancellation.

Run: `go test -race ./modules/07-concurrency/exercises/fetcher/`

## Project

One project (~45-60 minutes): **Duplicate File Finder** (`dupfind`).

See `project/README.md` for details.

Run: `go test -race ./modules/07-concurrency/project/... -timeout 60s`

## Submitting for Review

When all exercises and the project pass their tests, including `-race -count=3 -timeout 60s`, ask your instructor (Claude) to review `modules/07-concurrency/`.

## Further Reading

- [Effective Go: Concurrency](https://go.dev/doc/effective_go#concurrency)
- [Go Code Review Comments: Goroutine Lifetimes](https://github.com/golang/go/wiki/CodeReviewComments#goroutine-lifetimes)
- [Go Memory Model](https://go.dev/ref/mem)
- [Go Concurrency Patterns](https://go.dev/blog/pipelines)
- [Channel Axioms](https://dave.cheney.net/2014/03/19/channel-axioms)
- [The Go Blog: Context](https://go.dev/blog/context)
- [Sync Package](https://pkg.go.dev/sync)
- [Sync/Atomic Package](https://pkg.go.dev/sync/atomic)
- [Race Detector](https://go.dev/doc/articles/race_detector)

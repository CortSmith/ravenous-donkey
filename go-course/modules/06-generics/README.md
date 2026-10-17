# Module 06: Generics & Iterators

Go went ten years without generics, and the language still leans hard on interfaces
and plain loops. When generics arrived in Go 1.18 they were deliberately small: type
parameters on functions and types, constraints written as interfaces, and not much
more. Go 1.23 added **range-over-func iterators**, which let you write lazy sequences
that work in an ordinary `for ... range` loop.

This module covers both features, and also teaches you when to leave them alone.

> **Version note.** This course pins Go 1.24, and all code here compiles with it. The
> current release (October 2026) is Go 1.27, which added some generics features (see
> [What changed after 1.24](#what-changed-after-124)). They are mentioned where
> relevant but never used.

---

## 1. Goals

After this module you will be able to:

- write generic functions and types with type parameters, and read signatures like
  `func Map[S ~[]E, E, R any](s S, f func(E) R) []R` fluently;
- choose the right constraint: `any`, `comparable`, `cmp.Ordered`, or your own union
  with `~`;
- predict when type inference works and when you have to write type arguments;
- recognise when an interface or a plain function is a better tool than a type
  parameter;
- write correct `iter.Seq` / `iter.Seq2` iterators that stop when the consumer stops,
  and compose them into lazy pipelines;
- use the iterator-aware standard library: `slices.Collect`, `slices.Sorted`,
  `slices.Values`, `maps.Keys`, `maps.Values` and `maps.All`.

---

## 2. Concepts

### 2.1 The problem generics solve

Before Go 1.18, writing "the same function for several types" meant choosing between
three options, all bad:

```go
// 1. Copy and paste.
func SumInts(xs []int) int           { ... }
func SumFloats(xs []float64) float64 { ... }

// 2. interface{} plus type assertions: no compile-time safety, allocations for boxing.
func Sum(xs []interface{}) interface{} { ... }

// 3. Reflection: slow, and errors appear at run time.
func Sum(xs interface{}) interface{} { v := reflect.ValueOf(xs); ... }
```

A **type parameter** lets you write the function once and have the compiler check
every use:

```go
func Sum[T int | float64](xs []T) T {
	var total T
	for _, x := range xs {
		total += x
	}
	return total
}

Sum([]int{1, 2, 3})      // 6, T = int
Sum([]float64{1.5, 2.5}) // 4, T = float64
Sum([]string{"a"})       // compile error: string does not satisfy int | float64
```

### 2.2 Anatomy of a type parameter list

```go
func Map[S ~[]E, E, R any](s S, f func(E) R) []R
//      └──────┬───────┘
//   type parameters, each with a constraint
```

- The list sits in **square brackets** between the name and the ordinary parameters.
- Each type parameter has a **constraint**, which is an interface type. Adjacent
  parameters can share one: `E, R any` means both `E` and `R` are constrained by `any`.
- Parameters can refer to each other. `S ~[]E` says "S is any type whose underlying
  type is a slice of E".

Calling a generic function **instantiates** it: the compiler substitutes concrete type
arguments, checks they satisfy the constraints, and then type-checks the body as if
you had written that version by hand. You can instantiate explicitly
(`Map[[]int, int, string](xs, strconv.Itoa)`), but you rarely need to.

### 2.3 Constraints are interfaces (with type sets)

The Go spec defines an interface by its **type set**: the set of types that implement
it. For an ordinary method interface like `io.Reader`, that's every type with a
`Read` method. Since 1.18, an interface can also list types directly:

```go
type Integer interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}

type Float interface{ ~float32 | ~float64 }

type Number interface{ Integer | Float } // the union of two type sets
```

The constraint does two jobs at once:

1. **It restricts callers.** Only types in the set can be type arguments.
2. **It gives the body permissions.** An operation is allowed on a `T` only if it's
   valid for *every* type in `T`'s type set. `+` works on every `Number`, so
   `total += x` compiles. `%` doesn't work on floats, so `x % 2` fails for a `Number`,
   but it would compile for an `Integer`.

Interfaces with type elements (`int | float64`, `~string`) are **constraints only**.
You can't declare a variable of such a type:

```go
var n Number // cannot use type Number outside a type constraint: interface contains type constraints
```

#### Built-in and standard constraints

| Constraint | Type set | Lets you |
|---|---|---|
| `any` | every type (alias for `interface{}`) | assign, pass around, take address, declare `var zero T`. **Not** `==`. |
| `comparable` | types that support `==` and `!=` | use `==`, use `T` as a map key |
| `cmp.Ordered` (Go 1.21) | all integer, float and string types (with `~`) | `<`, `<=`, `>`, `>=`, and `min`/`max` builtins |
| `fmt.Stringer` (or any method interface) | types with that method | call the method |

Try `==` under `any` and you get:

```go
func Eq[T any](a, b T) bool { return a == b }
// invalid operation: a == b (incomparable types in type set)
```

That error is correct: slices, maps and funcs are in `any`'s type set and can't be
compared. Use `comparable` instead.

> `golang.org/x/exp/constraints` (with `constraints.Ordered`, `constraints.Integer`) is
> not in the standard library. Use `cmp.Ordered`, or write a small union as shown
> above. Don't add a dependency for one line of code.

#### The tilde: `~T` means "underlying type T"

Go programs are full of named types: `type Celsius float64`,
`type Duration int64`, `type UserID string`. Without the `~`, a constraint matches
**only** the exact type:

```go
type Celsius float64

func Sum[T int | float64](xs ...T) T { ... }

Sum(Celsius(1), Celsius(2))
// Celsius does not satisfy int | float64 (possibly missing ~ for float64 in int | float64)
```

`~float64` means "any type whose *underlying type* is `float64`", which includes
`Celsius`. Write `~` on every basic type in your own unions unless you have a specific
reason not to. The standard library's `cmp.Ordered` does.

### 2.4 Type inference

You almost never write type arguments, because the compiler infers them from the
ordinary arguments:

```go
Map([]int{1, 2}, strconv.Itoa) // S=[]int, E=int (from S ~[]E), R=string (from f)
```

Inference fails when a type parameter appears **only in the result** or in nothing at
all:

```go
func Zero[T any]() T { var z T; return z }

Zero()      // in call to Zero, cannot infer T
Zero[int]() // fine
```

This is why constructors of generic containers need explicit type arguments:
`lru.New[string, int](100)`, `new(Set[int])`. It's also why `Of[int]()` (an empty
variadic call) needs one, while `Of(1, 2, 3)` doesn't.

You can supply a **prefix** of the type arguments and let inference fill in the rest:
`Convert[float64](xs)` for `func Convert[To, From Number](xs []From) []To`. Order your
type parameters so that the ones callers must spell out come first.

### 2.5 The `S ~[]E` pattern

Compare two signatures for `Filter`:

```go
func Filter1[E any](s []E, keep func(E) bool) []E
func Filter2[S ~[]E, E any](s S, keep func(E) bool) S
```

Both accept a `Names` (where `type Names []string`). Only `Filter2` **returns** a
`Names`. `Filter1` returns a plain `[]string`, so the caller loses the named type and
any methods declared on it:

```go
type Names []string
func (n Names) Join() string { return strings.Join(n, ", ") }

Filter1(names, keep).Join() // compile error: []string has no method Join
Filter2(names, keep).Join() // ok
```

Every function in the `slices` package uses this pattern for that reason. Use it
whenever you return "the same kind of slice you were given".

### 2.6 Generic types

Types can have type parameters too:

```go
type Stack[T any] struct {
	items []T
}

func (s *Stack[T]) Push(v T) { s.items = append(s.items, v) }

func (s *Stack[T]) Pop() (T, bool) {
	var zero T // the zero value of whatever T is
	if len(s.items) == 0 {
		return zero, false
	}
	last := len(s.items) - 1
	v := s.items[last]
	s.items[last] = zero // let the GC reclaim it if T holds pointers
	s.items = s.items[:last]
	return v, true
}

var s Stack[string] // instantiate when you name the type
s.Push("hi")
```

Things to notice:

- In the method receiver you write `Stack[T]`, which re-declares the receiver's type
  parameter names. You can't add constraints there; they come from the type
  declaration.
- **Methods cannot declare their own type parameters** in Go 1.24 (see
  §2.10). `func (s *Stack[T]) Map[U any](...)` is a compile error
  (`method must have no type parameters`). Write a top-level function instead:
  `func MapStack[T, U any](s *Stack[T], f func(T) U) *Stack[U]`.
- A method can't *tighten* the receiver's constraint either. A `Set[T comparable]`
  can't have a `Sorted()` method that needs `T cmp.Ordered`. That's why
  `set.Sorted[T cmp.Ordered](s *Set[T]) []T` in Exercise 2 is a function.
- `var zero T` is the idiomatic way to get a zero value. `*new(T)` also works but reads
  worse.

**Generic type aliases** have been fully supported since Go 1.24:
`type Set[T comparable] = map[T]struct{}`. An alias is the *same* type, so it can't
have methods of its own. Most of the time you want a defined type
(`type Set[T comparable] struct{...}` or `map[...]...`), which can.

### 2.7 How generics are compiled (and why you shouldn't assume they're fast)

Go doesn't create a separate copy of each generic function for every type argument,
as C++ templates do. It uses **GC-shape stenciling with dictionaries**: one copy of
the code per "GC shape" (all pointer types share one shape), plus a hidden dictionary
argument that describes the concrete types. Consequences:

- Code size stays reasonable.
- A generic function called with a pointer or interface type argument can be **slower**
  than the equivalent interface-based code, because method calls go through the
  dictionary and may not be inlined.
- For value types like `int` and `float64` you typically get code as good as
  handwritten code.

Use generics for **type safety and to avoid duplication**. Don't use them for speed.
If speed matters, measure (module 09).

### 2.8 When NOT to use generics

Ian Lance Taylor's rule ([When To Use Generics](https://go.dev/blog/when-generics)):
*"Write Go programs by writing code, not by defining types."* Start concrete. Reach for
type parameters when you notice you're writing the same code a second time and only
the types differ.

**Good fits:**

- functions over slices, maps and channels of any element type (`Map`, `Filter`,
  `Keys`, `Merge`);
- general-purpose containers: sets, stacks, queues, trees, caches, ring buffers;
- the case where every type would share the *identical* implementation.

**Poor fits:**

```go
// BAD: a type parameter whose only use is to call a method. An interface does the
// same thing, more simply, and the caller can pass any io.Writer at run time.
func WriteAll[W io.Writer](w W, data []byte) error { _, err := w.Write(data); return err }

// GOOD
func WriteAll(w io.Writer, data []byte) error { _, err := w.Write(data); return err }
```

- **You only call methods on the value.** Use an interface.
- **The implementation differs per type.** Use an interface with per-type methods. If
  you find yourself writing `switch any(v).(type)` inside a generic function, that's a
  sign generics were the wrong choice.
- **The operation is about the *structure* of arbitrary types** (encoding, deep
  equality). That's reflection's job; see `encoding/json`.
- **The standard library already has it.** Don't wrap `slices.Index`,
  `slices.Contains`, `slices.SortFunc`, `slices.Max`, `maps.Clone` or `min`/`max`.

### 2.9 Iterators: range over functions (Go 1.23)

#### The shape

An iterator is just a function that takes a `yield` callback:

```go
package iter

type Seq[V any] func(yield func(V) bool)
type Seq2[K, V any] func(yield func(K, V) bool)
```

The iterator calls `yield` once per element. `yield` returns `false` when the consumer
wants to stop (the loop hit `break`, `return`, or a panic). Go 1.23 lets you `range`
over such a function directly:

```go
func Countdown(n int) iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := n; i > 0; i-- {
			if !yield(i) {
				return // the consumer is done: stop immediately
			}
		}
	}
}

for v := range Countdown(10) {
	if v < 8 {
		break
	}
	fmt.Println(v) // 10, 9, 8
}
```

The compiler rewrites the loop body into the `yield` function. `break` becomes
`return false`, and `continue` becomes `return true`. A `return` from the enclosing
function also works: the compiler records it, makes `yield` return `false`, and
completes the return once the iterator has returned.

#### The one rule

> **When `yield` returns `false`, the iterator must return, and must never call
> `yield` again.**

Break that rule and the program panics at run time:

```go
func bad() iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := range 3 {
			yield(i) // result ignored
		}
	}
}

for v := range bad() {
	if v == 1 {
		break
	}
}
// panic: runtime error: range function continued iteration after function for loop body returned false
```

Every iterator you write should contain the `if !yield(v) { return }` line. If you
have nested loops, `return` (not `break`) is what you want.

#### Push iterators are lazy and composable

Nothing runs until someone ranges over the sequence, and the iterator only produces
as many values as the consumer asks for. That makes adapters cheap to stack:

```go
func Filter[T any](seq iter.Seq[T], keep func(T) bool) iter.Seq[T] {
	return func(yield func(T) bool) {
		for v := range seq {
			if keep(v) && !yield(v) {
				return
			}
		}
	}
}

// Even numbers from an infinite source: fine, because the consumer stops.
for v := range Filter(Naturals(), isEven) {
	if v > 10 {
		break
	}
}
```

When the outer consumer breaks, `yield` returns false in `Filter`. `Filter` returns
from its `range seq` loop, and that loop's own `yield` returns false into `Naturals`,
which returns too. The stop signal travels all the way back to the source.

#### Iterators should be reusable

An `iter.Seq` is a value; callers may range over it more than once. Keep per-iteration
state (counters, cursors) **inside** the returned function, not in the enclosing one:

```go
// BAD: the counter is shared by every range over the returned Seq.
func Take[T any](seq iter.Seq[T], n int) iter.Seq[T] {
	taken := 0
	return func(yield func(T) bool) { ... taken++ ... }
}

// GOOD: each range starts fresh.
func Take[T any](seq iter.Seq[T], n int) iter.Seq[T] {
	return func(yield func(T) bool) {
		taken := 0
		...
	}
}
```

Some sequences are inherently single-use, such as one that reads from a network
stream. Document that when it's the case.

#### Pull iterators

Sometimes you need to drive the iteration yourself, for example to walk two
sequences in lockstep. `iter.Pull` converts a push iterator into a `next` function:

```go
next, stop := iter.Pull(seq)
defer stop() // always: releases the iterator's resources

for {
	v, ok := next()
	if !ok {
		break
	}
	...
}
```

`Pull` runs the iterator in a separate coroutine, so it costs much more than ranging
over the sequence. Use it only when push style genuinely can't express what you need.

#### Iterators in the standard library

| Function | Returns | Since |
|---|---|---|
| `slices.All(s)` | `iter.Seq2[int, E]` (index, value) | 1.23 |
| `slices.Values(s)` | `iter.Seq[E]` | 1.23 |
| `slices.Backward(s)` | `iter.Seq2[int, E]`, from the end | 1.23 |
| `slices.Collect(seq)` | `[]E` (nil for an empty seq) | 1.23 |
| `slices.AppendSeq(s, seq)` | `[]E` | 1.23 |
| `slices.Sorted(seq)` / `SortedFunc` | sorted `[]E` | 1.23 |
| `slices.Chunk(s, n)` | `iter.Seq[[]E]` | 1.23 |
| `maps.Keys(m)` / `maps.Values(m)` | `iter.Seq[K]` / `iter.Seq[V]` (**not slices!**) | 1.23 |
| `maps.All(m)` | `iter.Seq2[K, V]` | 1.23 |
| `maps.Collect(seq2)` / `maps.Insert(m, seq2)` | build a map from a `Seq2` | 1.23 |
| `strings.Lines`, `SplitSeq`, `FieldsSeq` (and `bytes.*`) | `iter.Seq[string]` | 1.24 |

The most common use, sorted map keys, is now one line:

```go
m := map[string]int{"b": 2, "a": 1, "c": 3}
for _, k := range slices.Sorted(maps.Keys(m)) {
	fmt.Println(k, m[k]) // a 1, b 2, c 3
}
```

> `maps.Keys` returns an **iterator**, not a slice. Many blog posts from 2022 show
> `golang.org/x/exp/maps.Keys`, which returned a slice. That version never made it into
> the standard library, and the `maps` package in 1.21 had no `Keys` at all. If you see
> `cannot use maps.Keys(m) (value of type iter.Seq[string]) as []string`, wrap it in
> `slices.Collect` (or `slices.Sorted`).

**Naming conventions** (from the standard library): a method that iterates over
everything is called `All`. Variants are named after what they yield (`Keys`,
`Values`, `Backward`). A function that returns an iterator version of an existing
slice-returning function gets a `Seq` suffix (`strings.Split` → `strings.SplitSeq`).

#### Iterators versus slices

Iterators aren't free. Each element is a call through a function value, and the
closures usually escape to the heap. A plain `for` loop over a slice is still the
fastest thing in Go. In this module's solutions, a `Filter`→`Map` iterator pipeline
over 10,000 ints ran about **5x slower** than the equivalent hand-written loop (see
the `iterx` benchmark in the review notes). Use iterators when you need laziness
(infinite or huge sources, early exit), when you need to expose a container's
contents without exposing its internals, or when composing would otherwise mean
building intermediate slices. Don't use them to replace every loop.

### 2.10 What changed after 1.24

You'll meet these in newer codebases. Don't use them in this course.

- **Go 1.26:** self-referential generic constraints, e.g.
  `type Adder[A Adder[A]] interface{ Add(A) A }`.
- **Go 1.27:** **generic methods**. A method can now declare its own type parameters,
  so `func (l List[T]) Map[U any](f func(T) U) List[U]` compiles in Go 1.27+. (Interface
  methods still can't have type parameters, so a generic method can't satisfy an
  interface method.) Go 1.27 also generalised function type inference. With the
  course's Go 1.24 toolchain, the top-level-function workaround from §2.6 is still the
  only option, and you'll see it throughout older code.

---

## 3. Idioms & Best Practices

- **Start concrete, generalise on the second copy.**
  ```go
  // BAD: designed generic up front for one caller
  type Repository[T Entity, ID comparable, Q Query[T]] interface { ... }
  // GOOD: write UserStore. Make it generic if a second store turns out to be identical.
  ```

- **Prefer the standard library's generic helpers to your own.**
  ```go
  // BAD
  func Contains[T comparable](xs []T, v T) bool { for ... }
  // GOOD
  slices.Contains(xs, v)
  ```

- **Use `~` in your own unions.**
  ```go
  type Number interface{ int | float64 }   // BAD: rejects time.Duration, Celsius, ...
  type Number interface{ ~int | ~float64 } // GOOD
  ```

- **Use the weakest constraint that compiles.** `any` < `comparable` < `cmp.Ordered`.
  A function that only stores values shouldn't demand `comparable`.

- **Pass a function instead of requiring a method or a stronger constraint.** That's
  why `slices.SortFunc` exists alongside `slices.Sort`.
  ```go
  func MaxBy[T any](xs []T, less func(a, b T) bool) T // works for any T
  ```

- **Return the caller's slice type with `S ~[]E`.**
  ```go
  func Filter[E any](s []E, f func(E) bool) []E        // loses named types
  func Filter[S ~[]E, E any](s S, f func(E) bool) S    // GOOD
  ```

- **Make the zero value of generic containers useful.** Lazily `make` the map in `Add`
  so that `var s Set[string]` just works, as `bytes.Buffer` and `sync.Mutex` do.

- **Clear vacated slots in containers.**
  ```go
  s.items = s.items[:len(s.items)-1]                           // BAD: old value still referenced
  s.items[last] = zero; s.items = s.items[:last]               // GOOD
  ```

- **Iterators: check `yield`'s result every time.** `if !yield(v) { return }`.

- **Expose container contents with `All() iter.Seq[...]`** rather than returning the
  internal slice or map, which callers could mutate, or copying everything into a new
  slice.

- **Use short type parameter names.** `T`, `K`, `V`, `E` (element), `S` (slice), `R`
  (result). Use longer names only when a function has several unrelated parameters
  and one-letter names would confuse.

- **Don't put constraint interfaces where they'll be mistaken for ordinary
  interfaces.** Name them for what they describe (`Number`, `Integer`) and keep them
  near their uses.

---

## 4. Common Pitfalls

1. **Forgetting `~`.** `Celsius does not satisfy int | float64 (possibly missing ~ for
   float64 ...)`. The compiler error tells you what's wrong, but it's better to avoid
   it in the first place.

2. **Using `==` under `any`.** Switch the constraint to `comparable`.

3. **`comparable` is satisfied by interface types, and `==` on them can panic.** Since
   Go 1.20, `any` satisfies `comparable` ([blog](https://go.dev/blog/comparable)). So
   this compiles, and panics at run time:
   ```go
   func Eq[T comparable](a, b T) bool { return a == b }
   var x, y any = []int{1}, []int{1}
   Eq(x, y) // panic: runtime error: comparing uncomparable type []int
   ```
   The same holds for a `map[any]V` or a `Set[any]`. If you key a set by `any`, every
   element must have a comparable dynamic type.

4. **Trying to add type parameters to a method.** In Go 1.24: `method must have no
   type parameters`. Use a top-level function. (Go 1.27 lifted this, but this course
   targets 1.24.)

5. **Type-switching on a type parameter.** `switch v.(type)` with `v T` is rejected:
   `cannot use type switch on type parameter value v`. `switch any(v).(type)` compiles,
   but if you need it, an interface was probably the right tool.

6. **Expecting inference from return types.** `Zero()` → `cannot infer T`. Write
   `Zero[int]()`.

7. **Treating `maps.Keys` as returning a slice.** It returns `iter.Seq[K]`. Wrap it:
   `slices.Collect(maps.Keys(m))`, or `slices.Sorted(maps.Keys(m))` for deterministic
   output.

8. **Ignoring `yield`'s return value.** This triggers a runtime panic as soon as a
   consumer breaks early. Tests that always consume the whole sequence won't catch it,
   so always test an early `break`.

9. **Iterator state outside the closure.** The second `range` over the same `Seq`
   silently yields nothing (or continues from where the first left off).

10. **`Filter` in place by accident.** `out := s[:0]` reuses the caller's backing
    array and overwrites their data. That's fine when you own `s` (it's what
    `slices.DeleteFunc` does), but it's a bug in a function documented as "returns a
    new slice".

11. **Holding on to popped values.** Reslicing `s[:n-1]` or `s[1:]` leaves the old
    element reachable from the backing array, so the GC can't free whatever it points
    to.

12. **Overflow in generic arithmetic.** `Sum[uint8](200, 100)` is `44`. The type
    parameter doesn't widen anything. If you need a wider accumulator, convert
    explicitly (as `Mean` does with `float64(x)`).

13. **NaN with ordered comparisons.** `cmp.Ordered` includes floats. `<` with a NaN is
    always false, so hand-written `if a < b` comparisons give inconsistent orderings.
    `cmp.Compare` and `cmp.Less` treat NaN as less than everything, which gives a
    consistent order.

---

## 5. Exercises

Each exercise is a package under `modules/06-generics/exercises/`. The tests are
complete; make them pass without editing them. Run all of them at once with
`go test ./modules/06-generics/exercises/...`.

| # | Package | What you implement | Command |
|---|---|---|---|
| 1 | `funcs` | `Map`, `Filter`, `Reduce`, `GroupBy` over slices, using the `S ~[]E` pattern. `Filter` must not touch its input's backing array. | `go test ./modules/06-generics/exercises/funcs/` |
| 2 | `set` | `Set[T comparable]` with a usable zero value, nil-safe reads, `Add`/`Remove`/`Contains`/`Len`, `Union`/`Intersect`/`Difference`, an `All() iter.Seq[T]` iterator, and a `Sorted` function (and an answer to why it isn't a method). | `go test ./modules/06-generics/exercises/set/` |
| 3 | `stackqueue` | Generic `Stack[T]` and `Queue[T]` with `Push`/`Pop`/`Peek`/`Len`. Returning a zero value of `T`; not leaking popped values. | `go test ./modules/06-generics/exercises/stackqueue/` |
| 4 | `numeric` | `Sum`, `SumBy`, `Mean`, `Clamp` over `Integer`/`Float`/`Number` constraints with `~`, plus `cmp.Ordered`. Start with the experiment in the package doc. | `go test ./modules/06-generics/exercises/numeric/` |
| 5 | `iterx` | Lazy iterator adapters: `Naturals`, `Filter`, `Map`, `Take`, `Enumerate` (`iter.Seq2`). The tests check laziness, early stop and reusability. | `go test ./modules/06-generics/exercises/iterx/` |

Suggested order: 1 → 4 → 3 → 2 → 5. Exercise 2 uses an iterator; read §2.9 first.

When a test fails, run it verbose for the full message, for example
`go test -run TestTake -v ./modules/06-generics/exercises/iterx/`.

---

## 6. Project

**Generic LRU Cache** (`modules/06-generics/project/`). Build
`Cache[K comparable, V any]` with O(1) `Get`/`Put`/`Delete`, capacity-based eviction,
optional TTL with an injectable clock, and an `All()` iterator in recency order. The
spec, hints and stretch goals (including a zero-allocation version) are in
[`project/README.md`](project/README.md). Run the tests with
`go test ./modules/06-generics/project/`.

---

## 7. Submitting for Review

When tests pass, ask your instructor (Claude) to review `modules/06-generics/`.

Before you ask, run:

```sh
gofmt -l modules/06-generics        # should print nothing
go vet ./modules/06-generics/...
go test ./modules/06-generics/...
```

The review covers more than passing tests. It also looks at constraint choice,
zero-value handling, iterator correctness, doc comments, and whether each type
parameter earns its place.

---

## 8. Further Reading

- [Tutorial: Getting started with generics](https://go.dev/doc/tutorial/generics)
- [An Introduction To Generics](https://go.dev/blog/intro-generics), Robert Griesemer & Ian Lance Taylor
- [When To Use Generics](https://go.dev/blog/when-generics), Ian Lance Taylor
- [All your comparable types](https://go.dev/blog/comparable) covers why `any` satisfies `comparable` since 1.20
- [Deconstructing Type Parameters](https://go.dev/blog/deconstructing-type-parameters) explains the `S ~[]E` pattern in `slices`
- [Range Over Function Types](https://go.dev/blog/range-functions), Ian Lance Taylor
- [What's in an (Alias) Name?](https://go.dev/blog/alias-names) covers generic type aliases
- Go spec: [Type parameter declarations](https://go.dev/ref/spec#Type_parameter_declarations),
  [Interface types / type sets](https://go.dev/ref/spec#Interface_types),
  [For statements with range clause](https://go.dev/ref/spec#For_range)
- Package docs: [`iter`](https://pkg.go.dev/iter), [`slices`](https://pkg.go.dev/slices),
  [`maps`](https://pkg.go.dev/maps), [`cmp`](https://pkg.go.dev/cmp),
  [`container/list`](https://pkg.go.dev/container/list)
- [100 Go Mistakes](https://100go.co/), #9 "Being confused about when to use generics"
- [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)

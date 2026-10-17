# Module 01: Types, Slices, Maps, Strings

Most Go bugs that new Gophers write are not exotic. They come from four everyday
types (slices, maps, strings and constants) behaving differently from their
look-alikes in other languages. This module explains how each one actually works
underneath, so you can predict their behaviour instead of memorising rules.

> The course pins **Go 1.24**. Everything below compiles with go1.24.2. Where
> newer releases changed something relevant, the text says "in Go 1.25+".

## Goals

By the end of this module you will be able to:

- Explain **zero values** and design code that uses them instead of fighting them.
- Draw the **slice header** (pointer, length, capacity), predict when `append`
  reuses or reallocates a backing array, and use the **full slice expression**
  `s[lo:hi:max]` to prevent aliasing bugs.
- Use **maps** safely: nil-map behaviour, comma-ok lookups, `map[K]struct{}`
  sets, and producing **deterministic output** despite randomised iteration.
- Tell **bytes, runes and strings** apart, iterate UTF-8 correctly, and build
  strings efficiently with `strings.Builder`.
- Define typed constants and enumerations with **`iota`**.
- Reach for the standard **`slices`, `maps` and `cmp`** packages and the
  **`min`, `max`, `clear`** built-ins instead of hand-rolling loops.

---

## Concepts

### 1. Zero values: every variable starts out usable

In Go, every variable is initialised. A declaration without a value gets the
type's **zero value** ([spec: The zero value](https://go.dev/ref/spec#The_zero_value)):

| Type | Zero value |
|---|---|
| `bool` | `false` |
| numeric types | `0` |
| `string` | `""` |
| pointer, func, interface, slice, map, channel | `nil` |
| array, struct | every element / field set to *its* zero value |

```go
var n int          // 0
var s string       // ""
var p *int         // nil
var xs []int       // nil slice: len 0, cap 0, safe to append to and range over
var m map[string]int // nil map: safe to READ, panics on WRITE (see below)
var sb strings.Builder // ready to use, no constructor needed
var mu sync.Mutex      // unlocked mutex, ready to use
```

**Why it matters:** Go's standard library is designed so that the zero value is
useful ("Make the zero value useful" is one of the
[Go Proverbs](https://go-proverbs.github.io/)). `bytes.Buffer`, `strings.Builder`,
`sync.Mutex` and `sync.WaitGroup` all work straight away. When you design your
own types, aim for the same thing. It removes a whole class of "forgot to call
`New`" bugs.

`var x T` is the idiomatic way to say "I want the zero value". Use `x := T{...}`
when you are actually filling in fields.

### 2. Arrays are values; slices are views

An **array** has its length as part of its type (`[3]int` and `[4]int` are
different types). Arrays are *values*, so assigning or passing one copies every
element:

```go
arr := [3]int{1, 2, 3}
arr2 := arr   // full copy
arr2[0] = 100
fmt.Println(arr, arr2) // [1 2 3] [100 2 3]
```

You rarely use arrays directly. Their main job is to be the storage underneath
**slices**. A slice is a small struct, the **slice header**:

```
type sliceHeader struct {   // conceptually; see runtime/slice.go
    ptr *T  // points at some element of a backing array
    len int // number of elements you can index: s[0] .. s[len-1]
    cap int // number of elements from ptr to the end of the backing array
}
```

Copying a slice copies the *header* (three words), **not the elements**. Two
slices can therefore point into the same backing array, and that is both what
makes slices cheap and what makes them surprising.
Read [Go Slices: usage and internals](https://go.dev/blog/slices-intro) once;
it is short and pays for itself many times over.

#### Slicing shares memory

```go
a := []int{1, 2, 3, 4}
b := a[:2]                 // b = [1 2], len 2, cap 4: same backing array
fmt.Println(len(b), cap(b)) // 2 4
```

```
a: ptr ─┐ len 4 cap 4
b: ptr ─┤ len 2 cap 4
        ▼
      [ 1 | 2 | 3 | 4 ]   ← one backing array
```

#### `append` reuses capacity, which causes the classic aliasing bug

`append(s, v)` checks whether `len(s) < cap(s)`. If so, it writes `v` into the
**existing** backing array at index `len(s)` and returns a header with `len+1`.
Only when capacity runs out does it allocate a new, larger array and copy.

```go
a := []int{1, 2, 3, 4}
b := a[:2]
b = append(b, 99)  // len(b)=2 < cap(b)=4, so it writes into a[2]!
fmt.Println(a, b)  // [1 2 99 4] [1 2 99]
```

`a` changed even though we never touched it. This is mistake #25 in
[100 Go Mistakes](https://100go.co/#unexpected-side-effects-using-slice-append-25).

#### The full slice expression caps capacity

`s[lo:hi:max]` creates a slice with `len = hi-lo` and **`cap = max-lo`**
([spec: Full slice expressions](https://go.dev/ref/spec#Slice_expressions)).
Setting `max == hi` means the result has no spare capacity, so the next `append`
*must* allocate and can't clobber anyone else's data:

```go
a := []int{1, 2, 3, 4}
c := a[:2:2]       // len 2, cap 2
c = append(c, 42)  // no room, so a new array is allocated
fmt.Println(a, c)  // [1 2 3 4] [1 2 42]   a is untouched
```

Use it whenever you hand out sub-slices of a buffer you still own. The
`chunk` exercise is built around it. If you need an independent copy instead
of a capped view, use `slices.Clone(s)`.

#### Always use the result of `append`

```go
append(s, x)       // compile error: the result is not used
s2 := append(s, x) // compiles, but now s and s2 may or may not share memory
s = append(s, x)   // the idiom
```

Because `append` may or may not reallocate, the only safe mental model is
"`append` returns *the* slice; the old header is stale".

#### Growth and preallocation

When `append` reallocates, it grows capacity geometrically (roughly doubling for
small slices, more gently for large ones; the exact policy is an
implementation detail, see `runtime.growslice`). Watch it happen:

```go
var s []int
for i := range 10 {
    s = append(s, i)
    fmt.Print(cap(s), " ")
}
// 1 2 4 4 8 8 8 8 16 16
```

Each reallocation copies every element and leaves garbage behind. If you know
(or can bound) the final size, **preallocate** with `make([]T, 0, n)`:

```go
// BAD: up to log2(n) reallocations
var names []string
for _, u := range users {
    names = append(names, u.Name)
}

// GOOD: one allocation
names := make([]string, 0, len(users))
for _, u := range users {
    names = append(names, u.Name)
}
```

Watch out for the most common preallocation bug. `make([]T, n)` makes a slice of
**length** n (n zero values). Appending to it adds *after* those zeros:

```go
x := make([]int, 3)
x = append(x, 1)
fmt.Println(x) // [0 0 0 1]   oops
```

Use either `make([]T, n)` + index assignment `x[i] = v`, **or**
`make([]T, 0, n)` + `append`. Never mix the two.

#### `copy` copies `min(len(dst), len(src))`

```go
src := []int{1, 2, 3}
dst := make([]int, 0, 3)
n := copy(dst, src) // n == 0! dst has length 0
dst = make([]int, len(src))
copy(dst, src)      // now [1 2 3]
```

#### nil vs empty slices

`var s []int` is **nil**; `s := []int{}` is **empty but non-nil**. Both have
`len 0`, both can be ranged over and appended to. So:

- Test emptiness with `len(s) == 0`, never `s == nil`.
- Prefer `var s []T` when declaring an empty slice
  ([CodeReviewComments: Declaring Empty Slices](https://go.dev/wiki/CodeReviewComments#declaring-empty-slices)).
- The one observable difference: `encoding/json` marshals nil as `null` and
  empty as `[]`. Choose deliberately when a slice is going out over an API.

#### Ranging copies elements

```go
type point struct{ x, y int }
pts := []point{{1, 2}, {3, 4}}
for _, p := range pts {
    p.x = 0 // modifies the copy `p`, not pts
}
for i := range pts {
    pts[i].x = 0 // modifies the element in place
}
```

#### Slices can keep big arrays alive

A small slice of a huge array keeps the *whole* array reachable, so the garbage
collector can't free it. If you keep a few elements from a large buffer for a
long time, copy them out with `slices.Clone(big[:10])`. Similarly, when you
shrink a `[]*T`, the pointers past the new length are still referenced by the
backing array. Zero them (`clear(s[newLen:])`) before reslicing.

### 3. Maps

A map is a hash table: a reference to a runtime structure, so copying a map
variable copies the *reference*, and both copies see the same data.
(Go 1.24 switched the implementation to [Swiss tables](https://go.dev/blog/swisstable);
the semantics did not change.)

#### Reading a nil map is fine; writing panics

```go
var m map[string]int     // nil
fmt.Println(m["x"], len(m)) // 0 0   reads return the zero value
for k, v := range m {}   // zero iterations
m["x"] = 1               // panic: assignment to entry in nil map
```

Always initialise before writing: `m := make(map[string]int)` or a literal
`m := map[string]int{}`. A nil map **field** inside a struct is the common
version of this bug:

```go
type Index struct{ words map[string]int }
var idx Index
idx.words["go"]++ // panic, the field's zero value is nil
```

Give such types a constructor, or initialise the map lazily on first write.

#### Comma-ok distinguishes "missing" from "zero"

```go
v := m["z"]       // 0 if missing. Was it missing, or stored as 0?
v, ok := m["z"]   // ok == false means missing
```

The zero-value default is *useful*: counting needs no existence check, because
a missing key reads as 0:

```go
counts := make(map[string]int)
for _, w := range words {
    counts[w]++ // works for new and existing keys
}
```

#### Iteration order is random, on purpose

The spec says map iteration order is unspecified, and the runtime **randomises
it on every `range`** so that nobody can depend on it by accident
([Go maps in action](https://go.dev/blog/maps)). Any output built by ranging
over a map must be sorted if it needs to be stable (tests, reports, diffs,
cache keys...):

```go
m := map[string]int{"b": 2, "a": 1, "c": 3}
for _, k := range slices.Sorted(maps.Keys(m)) { // Go 1.23+
    fmt.Println(k, m[k])
}
// a 1
// b 2
// c 3
```

`maps.Keys` returns an iterator (`iter.Seq[K]`) in Go 1.23+, not a slice;
`slices.Sorted` collects and sorts it. (Iterators get their own module later.)

#### Sets: `map[K]struct{}`

Go has no set type. The idiom is a map whose value is the empty struct, which
occupies zero bytes:

```go
seen := make(map[string]struct{})
seen["go"] = struct{}{}
if _, ok := seen["go"]; ok { /* present */ }
```

`map[K]bool` is also fine and reads nicely (`if seen[k]`). Prefer `struct{}` when
the set is large or when "false" would be a confusing third state.

#### Other map facts

- `make(map[K]V, n)` takes a **size hint**. A correct hint avoids incremental
  growth. A wildly *oversized* hint wastes memory: the `wordfreq` review shows a
  real case where it made code slower.
- Map values are not addressable: `m[k].field = v` does not compile for struct
  values. Read, modify and write back, or store pointers.
- Keys must be **comparable** (`==` defined): numbers, strings, pointers,
  arrays, structs of comparable fields... but not slices, maps or funcs.
- Maps are **not safe for concurrent use**. A concurrent write is a fatal
  runtime error (not a recoverable panic). Module 07 covers locking.
- `delete(m, k)` on a missing key (or nil map) is a no-op. `clear(m)` removes
  everything (Go 1.21+).
- Deleting entries during `range` is allowed. Entries *added* during `range`
  may or may not be visited.

### 4. Strings, bytes and runes

This is the area where intuitions from other languages fail most often. Read
[Strings, bytes, runes and characters in Go](https://go.dev/blog/strings).

- A `string` is an **immutable sequence of bytes**. It is *conventionally* UTF-8,
  but it can hold any bytes.
- A `byte` is an alias for `uint8`.
- A `rune` is an alias for `int32` and holds a Unicode **code point**.

```go
s := "héllo, 世界"
fmt.Println(len(s))                    // 14   BYTES, not characters
fmt.Println(utf8.RuneCountInString(s)) // 9    code points
fmt.Println(s[1])                      // 195  a byte (first half of 'é')
fmt.Printf("%q\n", s[1:2])             // "\xc3" slicing is by byte too
```

`é` is two bytes in UTF-8 (`0xC3 0xA9`) and `世` is three, so `s[1]` is half a
character. Indexing and slicing a string work on **bytes**.

#### `range` over a string decodes runes

```go
for i, r := range "aé世" {
    fmt.Print(i, ":", string(r), " ")
}
// 0:a 1:é 3:世
```

The index `i` is the **byte offset** where the rune starts (note the jump from 1
to 3), and `r` is the decoded rune. Invalid UTF-8 decodes to `U+FFFD` (the
replacement character `�`), one byte at a time, and never panics.

#### Conversions

| Conversion | Cost | Notes |
|---|---|---|
| `[]byte(s)` | copies | strings are immutable, byte slices aren't |
| `string(b)` | copies | (the compiler avoids the copy in a few cases, e.g. `m[string(b)]` lookups) |
| `[]rune(s)` | allocates 4 bytes per rune | decodes UTF-8; good for random access by rune |
| `string(runes)` | allocates | encodes back to UTF-8 |
| `string(r)` for a rune `r` | | the UTF-8 encoding of that code point |
| `string(65)` | | `"A"`, **not** `"65"`. `go vet` flags this; use `strconv.Itoa` |

#### A rune is not a "character"

What a user sees as one character (a *grapheme cluster*) may be several runes:
`"é"` can be one precomposed rune `U+00E9` *or* `e` + combining acute `U+0301`;
`"👍🏽"` is a thumbs-up plus a skin-tone modifier. Reversing such strings
rune-by-rune separates the parts. The standard library has no grapheme
segmentation and no Unicode normalisation (both live in
`golang.org/x/text`). The `reverse` and `anagram` exercises make you document
this limit rather than pretend it isn't there.

#### Useful `strings` functions (and their traps)

```go
strings.Fields("  a  b\tc ")             // ["a" "b" "c"]  splits on any whitespace run
strings.FieldsFunc(s, f)                 // splits where f(rune) is true
strings.Split("a,b,", ",")               // ["a" "b" ""]   keeps empties
strings.Join([]string{"a", "b"}, "-")    // "a-b"
strings.EqualFold("Go", "GO")            // true. Case-insensitive compare without allocating
strings.Cut("key=value", "=")            // "key", "value", true
strings.TrimSpace("  x \n")              // "x"

strings.TrimRight("abba", "ba")  // ""     removes any trailing chars in the SET {b,a}
strings.TrimSuffix("abba", "ba") // "ab"   removes ONE exact suffix
```

`TrimLeft`/`TrimRight` taking a *cutset* rather than a prefix/suffix is a
classic surprise (100 Go Mistakes #38).

Prefer `strings.EqualFold(a, b)` over `strings.ToLower(a) == strings.ToLower(b)`:
it is correct for simple case folding and allocates nothing.

#### Building strings: `strings.Builder`

Strings are immutable, so `s += x` allocates a new string and copies everything
so far. In a loop that is **O(n²)**:

```go
// BAD: quadratic copying
s := ""
for _, w := range words {
    s += w + ","
}

// GOOD: amortised linear
var sb strings.Builder
for i, w := range words {
    if i > 0 {
        sb.WriteByte(',')
    }
    sb.WriteString(w)
}
result := sb.String() // no copy: Builder hands over its buffer
```

- If you already have a `[]string`, `strings.Join` is simplest and pre-sizes for you.
- `sb.Grow(n)` preallocates when you can estimate the size.
- `*strings.Builder` is an `io.Writer`, so `fmt.Fprintf(&sb, "%d items", n)` works.
- The zero value is ready to use. **Don't copy a non-zero Builder** (pass a pointer);
  the copy panics on its next write.

### 5. Constants and `iota`

Constants are evaluated at compile time. **Untyped** constants (`const big = 1 << 100`)
have arbitrary precision and only take on a type when used, which is why
`time.Second * 2` and `math.Pi / 2` just work
([go.dev/blog/constants](https://go.dev/blog/constants)).

`iota` is the index of the current line inside a `const (...)` block. An
omitted expression repeats the previous one, with `iota` incremented:

```go
type Weekday int

const (
    Sunday Weekday = iota // 0
    Monday                // 1
    Tuesday               // 2
)

type ByteSize int64

const (
    _           = iota             // skip 0
    KB ByteSize = 1 << (10 * iota) // 1 << 10 = 1024
    MB                             // 1 << 20
    GB                             // 1 << 30
)
```

Enumeration advice:

- Give the enum its **own named type** (`Weekday`, not bare `int`) so the
  compiler stops you mixing it up with other integers.
- Consider making the zero value mean "unknown/unset" (`StatusUnknown Status = iota`)
  so an uninitialised field isn't silently a real state
  ([Uber style guide](https://github.com/uber-go/guide/blob/master/style.md#start-enums-at-one)).
- Add a `String()` method so it prints nicely. The stdlib has no generator for
  this (`stringer` lives in `golang.org/x/tools`), so write a `switch` or an array lookup.

Two numeric literal traps: `010` is **octal 8** (write `0o10` to make that
explicit), and integer overflow wraps silently (`int8(127) + 1 == -128`).

### 6. The `slices`, `maps` and `cmp` packages; `min`, `max`, `clear`

Go 1.21 added generic helper packages and built-ins that replace most
hand-written loops:

```go
xs := []int{3, 1, 2}
slices.Sort(xs)                     // [1 2 3]
slices.Contains(xs, 2)              // true
slices.Index(xs, 3)                 // 2
i, found := slices.BinarySearch(xs, 2) // 1 true (on sorted input)
slices.Reverse(xs)                  // in place
slices.Equal(a, b)                  // element-wise ==
ys := slices.Clone(xs)              // independent copy
xs = slices.Compact(xs)             // drop consecutive duplicates, in place
slices.Concat(a, b, c)              // Go 1.22

m2 := maps.Clone(m)
keys := slices.Sorted(maps.Keys(m)) // Go 1.23 (Keys returns an iterator)

min(3, 1, 2)       // 1    built-in, any ordered type, any number of args
max(2.5, 1)        // 2.5
clear(m)           // map: delete every entry
clear(xs)          // slice: zero every element; len unchanged!
```

**Multi-key sorting** combines `slices.SortFunc` with `cmp.Compare` and
`cmp.Or` (Go 1.22). `cmp.Or` returns its first non-zero argument, so it reads
as "compare by this, then by that":

```go
type WordCount struct {
    Word  string
    Count int
}

slices.SortFunc(wcs, func(a, b WordCount) int {
    return cmp.Or(
        cmp.Compare(b.Count, a.Count),   // count descending (note b, a)
        strings.Compare(a.Word, b.Word), // then word ascending
    )
})
```

The comparison function returns negative / zero / positive, like C's
`strcmp`. `slices.SortFunc` is **not stable**. If two elements compare equal
their relative order is unspecified, so either make the comparison total (as
above, by adding a tie-breaker) or use `slices.SortStableFunc`.

> You will see `sort.Slice(x, func(i, j int) bool {...})` in older code. Prefer
> `slices.SortFunc`: it is type-safe (no indices into a captured slice) and faster.

### 7. Loops, including range-over-int

Go has one loop keyword, `for`:

```go
for i := 0; i < n; i++ {}    // classic
for i := range n {}          // Go 1.22+: i = 0..n-1
for cond {}                  // while
for {}                       // forever
for i, v := range slice {}   // index, copy of element
for k, v := range m {}       // random order
for i, r := range str {}     // byte offset, rune
for range 3 {}               // just repeat 3 times
```

Since Go 1.22, **each iteration has its own loop variable** (when `go.mod` says
`go 1.22` or later, as ours does), so closures and goroutines that capture
`i` or `v` no longer all see the last value
([go.dev/blog/loopvar-preview](https://go.dev/blog/loopvar-preview)). You will
still see `v := v` lines in older code; they are now unnecessary.

---

## Idioms & Best Practices

- **Use the zero value.**
  ```go
  var sb strings.Builder // GOOD
  sb := strings.Builder{} // fine, but noisier
  sb := new(strings.Builder) // only if you need a pointer
  ```
- **Preallocate when the size is known.**
  ```go
  out := make([]T, 0, len(in))   // GOOD
  var out []T                    // OK when the size is unknown or usually tiny
  out := make([]T, len(in))      // BAD if you then append
  ```
- **Check emptiness with `len`.**
  ```go
  if len(s) == 0 { ... }  // GOOD: true for nil and empty
  if s == nil { ... }     // BAD: misses []T{}
  ```
- **Cap sub-slices you hand out.**
  ```go
  return buf[:n:n]  // GOOD: callers can't append into the rest of buf
  return buf[:n]    // RISKY: append(result, x) overwrites buf[n]
  ```
- **Sets are `map[K]struct{}`.**
  ```go
  seen := make(map[string]struct{}, len(xs)) // GOOD
  seen := make(map[string]int)               // BAD: the value means nothing
  ```
- **Sort before printing anything that came from a map.**
  ```go
  for _, k := range slices.Sorted(maps.Keys(m)) { ... } // GOOD: deterministic
  for k := range m { fmt.Println(k) }                   // BAD in tests/reports
  ```
- **Make comparisons total.**
  ```go
  cmp.Or(cmp.Compare(b.N, a.N), strings.Compare(a.S, b.S)) // GOOD: ties broken
  cmp.Compare(b.N, a.N)                                     // BAD: tie order random
  ```
- **Iterate strings with `range`, measure them with `utf8`.**
  ```go
  for _, r := range s { ... }             // GOOD: runes
  for i := 0; i < len(s); i++ { s[i] }    // only when you truly want bytes
  n := utf8.RuneCountInString(s)          // GOOD: characters (code points)
  n := len(s)                             // bytes!
  ```
- **Build strings with `strings.Builder` or `strings.Join`.**
  ```go
  strings.Join(parts, ", ")  // GOOD
  for ... { s += p + ", " }  // BAD: O(n²)
  ```
- **Compare case-insensitively with `strings.EqualFold`.**
  ```go
  strings.EqualFold(a, b)                    // GOOD: no allocation
  strings.ToLower(a) == strings.ToLower(b)   // BAD: two allocations
  ```
- **Use the standard helpers.**
  ```go
  slices.Contains(xs, x)                // GOOD
  for _, v := range xs { if v == x ... } // BAD: noise, easy to get wrong
  m := min(a, b)                         // GOOD (Go 1.21+)
  ```
- **Give enums a type, and consider making the zero value "unknown".**
  ```go
  type Color int
  const (
      ColorUnknown Color = iota
      ColorRed
  )
  ```

## Common Pitfalls

1. **`append` writing into someone else's data.** `b := a[:2]; b = append(b, x)`
   overwrites `a[2]`. Fix: `a[:2:2]` or `slices.Clone`.
2. **`make([]T, n)` followed by `append`.** Leaves n zero values at the front.
3. **Ignoring `append`'s result**, or keeping using the old slice afterwards.
4. **`copy` into a zero-length slice** copies nothing.
5. **Writing to a nil map** (especially a struct field) panics.
6. **Depending on map iteration order.** Tests pass locally "most of the time"
   and then fail in CI. Go randomises iteration so this fails *fast*. Always sort.
7. **`len(s)` on a string is bytes.** `len("世界") == 6`. Use `utf8.RuneCountInString`.
8. **Indexing/slicing strings by "character".** `s[:5]` can cut a UTF-8
   sequence in half and produce invalid text.
9. **`for i := range s` gives byte offsets** for strings, which skip numbers
   across multi-byte runes. Don't use `i` as a rune counter.
10. **`string(65) == "A"`**, not `"65"`. Use `strconv.Itoa`.
11. **`TrimLeft`/`TrimRight` take a cutset**, not a prefix/suffix.
12. **Unstable sort + incomplete comparator** gives nondeterministic order for ties.
13. **Comparing lengths as a shortcut.** E.g. "anagrams must have equal
    `len`": false once case-folding, punctuation-skipping or multi-byte runes
    are involved (`"É"` is 2 bytes, `"é"` is also 2, but `"K"` (Kelvin sign
    U+212A) is 3 bytes and lowercases to the 1-byte `"k"`).
14. **`clear(slice)` doesn't shorten it**; it zeroes the elements. Use `s = s[:0]` to empty.
15. **Octal literals**: `0644` is fine for file modes but `010 == 8` surprises people. Prefer `0o644`.
16. **A tiny sub-slice or substring pinning a huge parent** in memory. Clone what you keep.

---

## Exercises

Each exercise lives in `exercises/<name>/` with a README, a starter file whose
functions compile but return wrong results, and a complete test file. Make the
tests pass **without editing the test file**. Run commands from the course root.

| # | Exercise | What you implement | Command |
|---|---|---|---|
| 1 | [`reverse`](exercises/reverse/) | `Reverse(s)` by rune (UTF-8 safe) and `ReverseWords(s)` | `go test ./modules/01-basics/exercises/reverse/` |
| 2 | [`dedupe`](exercises/dedupe/) | `Dedupe([]string)` preserving first-occurrence order without aliasing the input; `CompactInPlace([]int)` with zero allocations | `go test ./modules/01-basics/exercises/dedupe/` |
| 3 | [`wordfreq`](exercises/wordfreq/) | `Words(text)` and `TopN(text, n)` with deterministic tie-breaking | `go test ./modules/01-basics/exercises/wordfreq/` |
| 4 | [`anagram`](exercises/anagram/) | `IsAnagram(a, b)`: Unicode-aware, case-insensitive, ignores punctuation | `go test ./modules/01-basics/exercises/anagram/` |
| 5 | [`chunk`](exercises/chunk/) | `Chunk(s, size)` as capped views; `RotateLeft(s, k)` in place | `go test ./modules/01-basics/exercises/chunk/` |

Run all exercises at once with `go test ./modules/01-basics/exercises/...`.
Add `-v` to see each subtest, or `-run 'TestTopN/ties'` to focus on one case.

## Project

**Text Analyzer** (`textstats`, about 45-60 minutes). Build a package that
tokenizes text (with a careful apostrophe rule), counts words, finds the most
frequent non-stopwords, the longest word and the average word length, and
formats a deterministic report. Then wrap it in a small, testable CLI. The spec
and acceptance criteria are in [project/README.md](project/README.md).

## Submitting for Review

When tests pass, ask your instructor (Claude) to review `modules/01-basics/`.

Before you do, run:

```sh
gofmt -l modules/01-basics          # should print nothing
go vet ./modules/01-basics/...
go test ./modules/01-basics/...
```

## Further Reading

- The Go spec: [Slice types](https://go.dev/ref/spec#Slice_types),
  [Slice expressions](https://go.dev/ref/spec#Slice_expressions),
  [Map types](https://go.dev/ref/spec#Map_types),
  [String types](https://go.dev/ref/spec#String_types),
  [Iota](https://go.dev/ref/spec#Iota),
  [Appending and copying slices](https://go.dev/ref/spec#Appending_and_copying_slices)
- [Go Slices: usage and internals](https://go.dev/blog/slices-intro)
- [Arrays, slices (and strings): The mechanics of 'append'](https://go.dev/blog/slices)
- [Go maps in action](https://go.dev/blog/maps) and [Swiss tables in Go 1.24](https://go.dev/blog/swisstable)
- [Strings, bytes, runes and characters in Go](https://go.dev/blog/strings)
- [Constants](https://go.dev/blog/constants)
- [Fixing for loops in Go 1.22](https://go.dev/blog/loopvar-preview)
- [Effective Go](https://go.dev/doc/effective_go): sections on Allocation, Slices, Maps and Constants
- [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
- [100 Go Mistakes](https://100go.co/): #17-#41 cover this module's topics
- Package docs: [slices](https://pkg.go.dev/slices), [maps](https://pkg.go.dev/maps),
  [cmp](https://pkg.go.dev/cmp), [strings](https://pkg.go.dev/strings),
  [unicode/utf8](https://pkg.go.dev/unicode/utf8), [unicode](https://pkg.go.dev/unicode)

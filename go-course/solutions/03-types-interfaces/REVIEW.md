# Review Guide: Structs, Methods & Interfaces

This document contains reference solutions, optimization notes, and grading checklists for each exercise and the project.

---

## Exercise 1: Shapes

### Learning Goals
- Implement the `Shape` interface with multiple types.
- Use the `fmt.Stringer` interface for custom formatting.
- Understand compile-time interface assertions.
- Sort with `slices.SortFunc` and `cmp.Compare`.

### Readable Solution

```go
// All three types (Circle, Rect, Triangle) implement Shape with value receivers
// because they are immutable and small.

type Circle struct {
	Radius float64
}

func (c Circle) Area() float64 { return math.Pi * c.Radius * c.Radius }
func (c Circle) Perimeter() float64 { return 2 * math.Pi * c.Radius }
func (c Circle) String() string { return "Circle(r=" + ftoa(c.Radius) + ")" }

type Rect struct {
	Width, Height float64
}

func (r Rect) Area() float64 { return r.Width * r.Height }
func (r Rect) Perimeter() float64 { return 2 * (r.Width + r.Height) }
func (r Rect) String() string { return "Rect(" + ftoa(r.Width) + "x" + ftoa(r.Height) + ")" }

type Triangle struct {
	A, B, C float64
}

func NewTriangle(a, b, c float64) (Triangle, error) {
	if a <= 0 || b <= 0 || c <= 0 {
		return Triangle{}, fmt.Errorf("sides must be positive: %w", ErrInvalidTriangle)
	}
	if a+b <= c || a+c <= b || b+c <= a {
		return Triangle{}, fmt.Errorf("triangle inequality violation: %w", ErrInvalidTriangle)
	}
	return Triangle{A: a, B: b, C: c}, nil
}

func (t Triangle) Area() float64 {
	s := t.Perimeter() / 2
	return math.Sqrt(s * (s - t.A) * (s - t.B) * (s - t.C))
}

func (t Triangle) Perimeter() float64 { return t.A + t.B + t.C }
func (t Triangle) String() string {
	return "Triangle(" + ftoa(t.A) + ", " + ftoa(t.B) + ", " + ftoa(t.C) + ")"
}

// Helper to format floats like %g (shortest round-trip form)
func ftoa(f float64) string {
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// TotalArea returns the sum of the areas of shapes.
func TotalArea(shapes []Shape) float64 {
	var total float64
	for _, s := range shapes {
		total += s.Area()
	}
	return total
}

// Largest returns the shape with the greatest area.
func Largest(shapes []Shape) (Shape, bool) {
	if len(shapes) == 0 {
		return nil, false
	}
	largest := shapes[0]
	for _, s := range shapes {
		if s.Area() > largest.Area() {
			largest = s
		}
	}
	return largest, true
}

// SortByArea sorts shapes by area (stable sort preserves ties by SKU/input order).
func SortByArea(shapes []Shape) {
	slices.SortStableFunc(shapes, func(a, b Shape) int {
		return cmp.Compare(a.Area(), b.Area())
	})
}

// Compile-time assertions: prove every type satisfies both interfaces.
var (
	_ Shape        = Circle{}
	_ Shape        = Rect{}
	_ Shape        = Triangle{}
	_ fmt.Stringer = Circle{}
	_ fmt.Stringer = Rect{}
	_ fmt.Stringer = Triangle{}
)
```

### Optimized Solution

The readable solution is already well-optimized. The only micro-optimization would be avoiding the string concatenation in `String()` methods by using `fmt.Sprintf`, but the concatenation version is more efficient in terms of allocations.

A minor optimization for `TotalArea` and `Largest` would be inlining the iteration, but the code is clear and the compiler likely inlines the loops anyway.

### Trade-off

The readable solution is the optimal one here. Simplicity, clarity, and performance are aligned. String formatting with concatenation is faster than `fmt.Sprintf` for constant templates.

### Review Checklist

**Correctness:**
- [ ] All three types satisfy `Shape` and `fmt.Stringer`.
- [ ] Compile-time assertions are present and correct.
- [ ] `NewTriangle` validates the triangle inequality strictly (≤ fails, not <).
- [ ] Area calculations are correct (using Heron's formula for triangles).
- [ ] `String()` outputs match the expected format exactly (e.g., "Circle(r=2)").
- [ ] `TotalArea` returns 0 for nil or empty slices.
- [ ] `Largest` returns the first shape when all areas are equal (not nil).

**Receiver Types:**
- [ ] All methods use value receivers (Circle, Rect, Triangle are immutable).
- [ ] No pointer receivers where they shouldn't be.

**Error Handling:**
- [ ] `NewTriangle` errors wrap `ErrInvalidTriangle` with `%w`.
- [ ] Error messages mention the offending values.

**Sorting:**
- [ ] Uses `slices.SortStableFunc` (stable sort preserves input order for ties).
- [ ] Comparison function uses `cmp.Compare`.
- [ ] Sorting is in-place.

**Code Style:**
- [ ] `ftoa` helper is defined and used consistently.
- [ ] No magic numbers in string formatting.
- [ ] No infinite recursion in `String()` (no `fmt.Sprintf("%v", c)`).

---

## Exercise 2: Stack

### Learning Goals
- Understand the difference between value and pointer receivers.
- Design mutable types with consistent receiver types.
- Implement `Clone` and return copies to avoid leaking internal state.
- Implement `fmt.Stringer` for custom printing.

### Readable Solution

```go
// Stack uses all pointer receivers because it mutates.
// The zero value is a ready-to-use empty stack.

type Stack struct {
	items []int
}

func (s *Stack) Push(v int) {
	s.items = append(s.items, v)
}

func (s *Stack) Pop() (v int, ok bool) {
	if len(s.items) == 0 {
		return 0, false
	}
	last := len(s.items) - 1
	v = s.items[last]
	s.items = s.items[:last]
	return v, true
}

func (s *Stack) Peek() (v int, ok bool) {
	if len(s.items) == 0 {
		return 0, false
	}
	return s.items[len(s.items)-1], true
}

func (s *Stack) Len() int {
	return len(s.items)
}

// Values returns a copy, not the internal slice.
func (s *Stack) Values() []int {
	return slices.Clone(s.items)
}

// Clone returns an independent copy of the stack.
func (s *Stack) Clone() *Stack {
	return &Stack{items: slices.Clone(s.items)}
}

// String formats the stack.
func (s *Stack) String() string {
	return fmt.Sprintf("Stack%v", s.items)
}
```

### Optimized Solution

The readable solution is already very efficient. One minor alternative for `String()` uses a strings.Builder to avoid the allocation from `fmt.Sprintf`, but for a small slice it's unlikely to matter:

```go
// Micro-optimization: avoid fmt.Sprintf allocation
func (s *Stack) String() string {
	var sb strings.Builder
	sb.WriteString("Stack[")
	for i, v := range s.items {
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(strconv.Itoa(v))
	}
	sb.WriteByte(']')
	return sb.String()
}
```

But this is more complex and the readable version is almost certainly faster in practice (fmt is optimized, and the slice is usually small). The benchmark difference would be negligible.

### Trade-off

**Readable:** `fmt.Sprintf("Stack%v", s.items)` is clear and concise.

**Optimized:** Manual formatting with `strings.Builder` avoids formatting overhead.

**Recommendation:** Use the readable version. The overhead is minimal, and clarity is more important than micro-optimizations here.

### Review Checklist

**Receiver Types:**
- [ ] All methods use pointer receivers (Stack is mutable).
- [ ] Consistent use of `*Stack`, no value receivers.

**Correctness:**
- [ ] `Push` adds an element and increases the stack.
- [ ] `Pop` removes and returns the top element.
- [ ] `Pop` returns (0, false) when empty, not a panic.
- [ ] `Peek` returns the top element without removing it.
- [ ] `Len` returns the correct length.
- [ ] `Values()` returns a *copy*, not a reference to the internal slice. Mutating the result does not affect the stack.
- [ ] `Clone()` returns an independent copy. Modifying one does not affect the other.
- [ ] The zero value `var s Stack` is ready to use (no need for `NewStack`).

**String Formatting:**
- [ ] `String()` output matches "Stack[1 2 3]" format (spaces between elements).
- [ ] Empty stack prints as "Stack[]".
- [ ] Using `fmt.Sprintf` is acceptable; no need for manual formatting.

**Memory Safety:**
- [ ] `Values()` and `Clone()` use `slices.Clone` to avoid slice aliasing.
- [ ] Internal `items` slice is never leaked to callers.

---

## Exercise 3: Describe

### Learning Goals
- Implement a comprehensive type switch to inspect `interface{}` values.
- Handle many types with proper priority (error before Stringer).
- Understand the "nil interface vs nil pointer" gotcha.
- Implement recursive formatting for slices and maps.

### Readable Solution

```go
func Describe(v any) string {
	switch v := v.(type) {
	case nil:
		return "nil"
	case error:
		// Error must be checked BEFORE Stringer, since a type can implement both.
		return "error: " + v.Error()
	case fmt.Stringer:
		return "Stringer: " + v.String()
	case bool:
		return "bool " + strconv.FormatBool(v)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		// Multi-type case: v keeps type any, so let fmt %T figure out the actual type.
		return fmt.Sprintf("%T %d", v, v)
	case float32:
		return "float32 " + strconv.FormatFloat(float64(v), 'g', -1, 32)
	case float64:
		return "float64 " + strconv.FormatFloat(v, 'g', -1, 64)
	case string:
		return "string " + strconv.Quote(v)
	case []byte:
		return "[]byte len=" + strconv.Itoa(len(v))
	case []any:
		parts := make([]string, len(v))
		for i, elem := range v {
			parts[i] = Describe(elem)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]any:
		// Sort keys for deterministic output.
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = k + ": " + Describe(v[k])
		}
		return "map[" + strings.Join(parts, ", ") + "]"
	default:
		return fmt.Sprintf("unknown %T", v)
	}
}

// ValidationError carries structured error information.
type ValidationError struct {
	Value  int
	Reason string
}

func (e *ValidationError) Error() string {
	return "invalid value " + strconv.Itoa(e.Value) + ": " + e.Reason
}

// Validate returns nil (untyped) on success, or a *ValidationError on failure.
// Key: return the untyped nil literal, not a nil pointer.
func Validate(n int) error {
	switch {
	case n < 0:
		return &ValidationError{Value: n, Reason: "negative"}
	case n > 100:
		return &ValidationError{Value: n, Reason: "greater than 100"}
	}
	return nil // untyped nil, not (*ValidationError)(nil)
}
```

### Optimized Solution

The readable solution is already well-optimized. The only potential micro-optimization would be:
- Pre-allocate the `parts` slice with `make([]string, len(v))` instead of building it incrementally, which this solution already does.
- Cache sorted keys if maps are frequently described, but that's premature.

No optimization is needed for correctness or typical performance.

### Trade-off

**Readable:** The solution above is clear, correct, and handles all cases with proper priority.

**Optimized:** Caching sorted keys or precomputing parts would only help in a tight loop over many maps.

**Recommendation:** The readable version is optimal here.

### Review Checklist

**Type Switch Order:**
- [ ] `nil` case handled first.
- [ ] `error` case checked *before* `fmt.Stringer` (priority matters when a type implements both).
- [ ] `fmt.Stringer` case before primitives.
- [ ] Default case last.

**Output Format:**
- [ ] `nil`: `"nil"`.
- [ ] `error`: `"error: " + message`.
- [ ] `Stringer`: `"Stringer: " + String()` output.
- [ ] `bool`: `"bool true"` or `"bool false"`.
- [ ] Integer types: `"<type> <value>"` with correct type name (e.g., `"int8 -3"`).
- [ ] `float32`/`float64`: shortest representation (e.g., `"float64 2.5"`).
- [ ] `string`: uses `strconv.Quote` for proper escaping (e.g., `` string "a\"b" ``).
- [ ] `[]byte`: `"[]byte len=N"`.
- [ ] `[]any`: `"[elem1, elem2, ...]"` with recursive description.
- [ ] `map[string]any`: `"map[k1: desc, k2: desc]"` with sorted keys and recursive values.
- [ ] Unknown types: `"unknown <type>"` where `<type>` is from `%T`.

**Recursion:**
- [ ] `[]any` elements are recursively described.
- [ ] `map[string]any` values are recursively described.
- [ ] Nested slices/maps format correctly, e.g., `"[map[k: int 1]]"`.

**The Nil Gotcha:**
- [ ] `Validate` returns the untyped `nil` literal, not a nil pointer.
- [ ] A nil `*ValidationError` would return a non-nil error interface (both cases are tested).

**Edge Cases:**
- [ ] Empty slices: `[]any{}` → `"[]"`.
- [ ] Empty maps: `map[string]any{}` → `"map[]"`.
- [ ] `[]byte(nil)` and `[]byte{}` are distinct (both should work).
- [ ] Negative numbers format correctly.
- [ ] Very large integers (int64.MinValue) format correctly.

---

## Exercise 4: Logging

### Learning Goals
- Understand method promotion through embedding.
- Design composition vs inheritance patterns.
- Implement interfaces that compose embedded types.

### Readable Solution

```go
// Logger is an unexported type that logs messages.
type Logger struct {
	prefix string
}

func (l Logger) Log(msg string) {
	fmt.Println(l.prefix + msg)
}

// Animal embeds a Logger and uses its methods.
type Animal struct {
	Logger // embedded: Log is promoted
	Name   string
	Sound  string
}

// Speak uses the promoted Log method.
func (a Animal) Speak() string {
	msg := a.Name + " says " + a.Sound
	a.Log(msg) // promoted method
	return msg
}
```

### Optimized Solution

The readable solution is optimal. Embedding is the idiomatic Go pattern here.

### Trade-off

Embedding is cleaner than composition for this use case because `Log` is promoted. If you wanted to hide `Log`, you'd use a named field instead.

### Review Checklist

**Embedding:**
- [ ] Logger is unexported (lowercase `type Logger`).
- [ ] Animal embeds Logger (not a named field).
- [ ] Log method is promoted to Animal.
- [ ] Animal can call `a.Log(...)` without `a.Logger.Log(...)`.

**Implementation:**
- [ ] `Speak()` returns the correct string.
- [ ] `Speak()` calls `Log()` to record the action.
- [ ] Log output includes the correct prefix.

**Design:**
- [ ] No mixing of embedded fields with other logic.
- [ ] Methods are placed correctly (on the type that owns them).

---

## Exercise 5: Server

### Learning Goals
- Implement the functional options pattern.
- Validate configuration options and report errors.
- Design immutable configuration types.
- Use constructors to enforce invariants.

### Readable Solution

```go
type Server struct {
	host         string
	port         int
	readTimeout  time.Duration
	writeTimeout time.Duration
	maxConns     int
	certFile     string
	keyFile      string
}

type Option func(*Server) error

func WithPort(port int) Option {
	return func(s *Server) error {
		if port < 1 || port > 65535 {
			return fmt.Errorf("port %d: %w", port, ErrInvalidOption)
		}
		s.port = port
		return nil
	}
}

func WithTimeouts(read, write time.Duration) Option {
	return func(s *Server) error {
		if read <= 0 || write <= 0 {
			return fmt.Errorf("timeout must be positive: %w", ErrInvalidOption)
		}
		s.readTimeout = read
		s.writeTimeout = write
		return nil
	}
}

func WithMaxConns(n int) Option {
	return func(s *Server) error {
		if n < 1 {
			return fmt.Errorf("max conns %d must be >= 1: %w", n, ErrInvalidOption)
		}
		s.maxConns = n
		return nil
	}
}

func WithTLS(certFile, keyFile string) Option {
	return func(s *Server) error {
		if certFile == "" || keyFile == "" {
			return fmt.Errorf("TLS cert and key must be non-empty: %w", ErrInvalidOption)
		}
		s.certFile = certFile
		s.keyFile = keyFile
		return nil
	}
}

func New(host string, opts ...Option) (*Server, error) {
	s := &Server{
		host:         host,
		port:         DefaultPort,
		readTimeout:  DefaultReadTimeout,
		writeTimeout: DefaultWriteTimeout,
		maxConns:     DefaultMaxConns,
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(s); err != nil {
			return nil, fmt.Errorf("server: %w", err)
		}
	}
	return s, nil
}

func (s *Server) Addr() string {
	return net.JoinHostPort(s.host, fmt.Sprint(s.port))
}

func (s *Server) ReadTimeout() time.Duration { return s.readTimeout }
func (s *Server) WriteTimeout() time.Duration { return s.writeTimeout }
func (s *Server) MaxConns() int { return s.maxConns }
func (s *Server) TLS() bool { return s.certFile != "" }

func (s *Server) String() string {
	scheme := "http"
	if s.TLS() {
		scheme = "https"
	}
	addr := s.Addr()
	return fmt.Sprintf("%s://%s (read %v, write %v, max %d conns)", scheme, addr, s.readTimeout, s.writeTimeout, s.maxConns)
}
```

### Optimized Solution

The readable solution is already optimized. No changes needed.

### Trade-off

The functional options pattern trades a small amount of complexity for a very clean, extensible API. It's the right choice here.

### Review Checklist

**Functional Options Pattern:**
- [ ] `Option` is a function type that takes `*Server` and returns `error`.
- [ ] Each option function is defined (WithPort, WithTimeouts, etc.).
- [ ] Options are applied in order.
- [ ] Nil options are skipped.
- [ ] The first failing option stops construction.

**Validation:**
- [ ] Port range: 1–65535.
- [ ] Timeouts: both must be positive.
- [ ] MaxConns: must be >= 1.
- [ ] TLS: both cert and key must be non-empty.

**Error Handling:**
- [ ] Errors wrap `ErrInvalidOption`.
- [ ] Error messages mention the offending field or value.
- [ ] `New` wraps option errors with "server: " prefix.

**Getters:**
- [ ] No `Get` prefix (e.g., `ReadTimeout()`, not `GetReadTimeout()`).
- [ ] All getters return the configured value or the default.

**String Formatting:**
- [ ] Uses "http" or "https" based on TLS.
- [ ] Includes host:port (uses `net.JoinHostPort` for IPv6 bracketing).
- [ ] Includes timeouts and max conns in a readable format.

**Type Safety:**
- [ ] Server fields are unexported (lowercase).
- [ ] Can only be created through `New`.

---

## Project: Inventory Manager

### Learning Goals
- Design multiple types that work together (Product, Store, Inventory).
- Implement an interface with a concrete implementation.
- Use embedding and composition appropriately.
- Implement sorting and formatting for reports.
- Practice concurrency safety with mutexes.
- Implement error handling with custom types and wrapping.

### Readable Solution

See the solution files for the complete implementation. Below are key design points:

#### Product Type (Readable)

```go
type Product struct {
	SKU          string
	Name         string
	Category     string
	PriceCents   int64
	Quantity     int
	ReorderLevel int
}

func (p Product) Validate() error {
	if p.SKU == "" {
		return fmt.Errorf("SKU: %w", ErrInvalid)
	}
	if p.Name == "" {
		return fmt.Errorf("name: %w", ErrInvalid)
	}
	if p.PriceCents < 0 {
		return fmt.Errorf("price: %w", ErrInvalid)
	}
	if p.Quantity < 0 {
		return fmt.Errorf("quantity: %w", ErrInvalid)
	}
	if p.ReorderLevel < 0 {
		return fmt.Errorf("reorder: %w", ErrInvalid)
	}
	return nil
}

func (p Product) Value() int64 { return p.PriceCents * int64(p.Quantity) }
func (p Product) LowStock() bool { return p.Quantity <= p.ReorderLevel }

func (p Product) String() string {
	return fmt.Sprintf("%s %q qty=%d @ %s", p.SKU, p.Name, p.Quantity, FormatCents(p.PriceCents))
}

func FormatCents(cents int64) string {
	if cents < 0 {
		return "-" + FormatCents(-cents)
	}
	dollars := cents / 100
	fraction := cents % 100
	return fmt.Sprintf("$%d.%02d", dollars, fraction)
}
```

#### Store Interface and MemoryStore (Readable)

```go
type Store interface {
	Add(p Product) error
	Get(sku string) (Product, error)
	Update(p Product) error
	Delete(sku string) error
	List() []Product
}

type MemoryStore struct {
	mu       sync.Mutex
	products map[string]Product
}

var _ Store = (*MemoryStore)(nil) // compile-time assertion

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{products: make(map[string]Product)}
}

func (m *MemoryStore) Add(p Product) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.products[p.SKU]; exists {
		return fmt.Errorf("sku %s: %w", p.SKU, ErrDuplicate)
	}
	m.products[p.SKU] = p
	return nil
}

func (m *MemoryStore) Get(sku string) (Product, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, exists := m.products[sku]
	if !exists {
		return Product{}, fmt.Errorf("sku %s: %w", sku, ErrNotFound)
	}
	return p, nil
}

func (m *MemoryStore) Update(p Product) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.products[p.SKU]; !exists {
		return fmt.Errorf("sku %s: %w", p.SKU, ErrNotFound)
	}
	m.products[p.SKU] = p
	return nil
}

func (m *MemoryStore) Delete(sku string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.products[sku]; !exists {
		return fmt.Errorf("sku %s: %w", sku, ErrNotFound)
	}
	delete(m.products, sku)
	return nil
}

func (m *MemoryStore) List() []Product {
	m.mu.Lock()
	defer m.mu.Unlock()
	skus := slices.Collect(maps.Keys(m.products))
	slices.Sort(skus)
	result := make([]Product, len(skus))
	for i, sku := range skus {
		result[i] = m.products[sku]
	}
	return result
}
```

#### Inventory Type (Readable)

```go
type Inventory struct {
	mu      sync.Mutex // guards the sequence of Get-Update operations
	store   Store
	alerter Alerter
}

func New(store Store, alerter Alerter) *Inventory {
	return &Inventory{store: store, alerter: alerter}
}

func (inv *Inventory) Sell(sku string, qty int) error {
	if qty <= 0 {
		return fmt.Errorf("quantity: %w", ErrInvalid)
	}
	inv.mu.Lock()
	defer inv.mu.Unlock()
	
	p, err := inv.store.Get(sku)
	if err != nil {
		return err
	}
	if p.Quantity < qty {
		return fmt.Errorf("sku %s: %w", sku, ErrInsufficientStock)
	}
	
	wasLowStock := p.LowStock()
	p.Quantity -= qty
	
	if err := inv.store.Update(p); err != nil {
		return err
	}
	
	if !wasLowStock && p.LowStock() && inv.alerter != nil {
		inv.alerter.LowStock(p)
	}
	return nil
}
```

#### Sorting and Reporting (Readable)

```go
func (inv *Inventory) Sorted(key SortKey) ([]Product, error) {
	products := inv.store.List()
	switch key {
	case BySKU:
		// already sorted by List()
	case ByName:
		slices.SortStableFunc(products, func(a, b Product) int {
			if cmp := cmp.Compare(a.Name, b.Name); cmp != 0 {
				return cmp
			}
			return cmp.Compare(a.SKU, b.SKU)
		})
	case ByValue:
		slices.SortStableFunc(products, func(a, b Product) int {
			// descending value
			if cmp := -cmp.Compare(a.Value(), b.Value()); cmp != 0 {
				return cmp
			}
			return cmp.Compare(a.SKU, b.SKU)
		})
	case ByQuantity:
		slices.SortStableFunc(products, func(a, b Product) int {
			if cmp := cmp.Compare(a.Quantity, b.Quantity); cmp != 0 {
				return cmp
			}
			return cmp.Compare(a.SKU, b.SKU)
		})
	default:
		return nil, fmt.Errorf("sort key: %w", ErrInvalid)
	}
	return products, nil
}

func (inv *Inventory) WriteReport(w io.Writer, key SortKey) error {
	products, err := inv.Sorted(key)
	if err != nil {
		return err
	}
	
	totalQty := 0
	totalValue := int64(0)
	
	fmt.Fprintf(w, "%-8s %-20s %5s %10s %12s\n", "SKU", "NAME", "QTY", "PRICE", "VALUE")
	for _, p := range products {
		fmt.Fprintf(w, "%-8s %-20s %5d %10s %12s\n",
			p.SKU, p.Name, p.Quantity, FormatCents(p.PriceCents), FormatCents(p.Value()))
		totalQty += p.Quantity
		totalValue += p.Value()
	}
	
	fmt.Fprintf(w, "%-8s %-20s %5d %10s %12s\n",
		"TOTAL", "", totalQty, "", FormatCents(totalValue))
	
	lowStockSKUs := inv.LowStock()
	if len(lowStockSKUs) == 0 {
		fmt.Fprintf(w, "low stock: none\n")
	} else {
		skus := make([]string, len(lowStockSKUs))
		for i, p := range lowStockSKUs {
			skus[i] = p.SKU
		}
		fmt.Fprintf(w, "low stock: %s\n", strings.Join(skus, ", "))
	}
	
	return nil
}
```

### Optimized Solution

A performance-optimized version might:

1. **Avoid allocations in List()** by using a preallocated slice:
   ```go
   func (m *MemoryStore) List() []Product {
       m.mu.Lock()
       defer m.mu.Unlock()
       result := make([]Product, 0, len(m.products))
       for _, sku := range slices.Collect(maps.Keys(m.products)) {
           result = append(result, m.products[sku])
       }
       slices.Sort(result) // would need custom less func or define a type
       return result
   }
   ```
   But since we can't sort Products directly, this doesn't help.

2. **Cache sorted keys in MemoryStore** to avoid repeated sorting:
   ```go
   type MemoryStore struct {
       mu         sync.Mutex
       products   map[string]Product
       sortedSKUs []string // cached sorted list of keys
   }
   ```
   This adds complexity and only helps if `List()` is called repeatedly.

3. **Use sync.RWMutex instead of sync.Mutex** if reads heavily outnumber writes:
   ```go
   type MemoryStore struct {
       mu       sync.RWMutex
       products map[string]Product
   }
   ```
   But the added complexity may not be worth it unless profiling shows contention.

### Trade-off

**Readable:** The solution above is clear, correct, and maintainable. The performance is adequate for most inventory use cases.

**Optimized:** Cache sorted keys, use RWMutex. These add complexity without major benefit unless profiling shows real bottlenecks.

**Recommendation:** Start with the readable version. Optimize only if profiling shows the need.

### Review Checklist

**Product Type:**
- [ ] Validates all fields in the correct order (SKU, Name, Price, Quantity, Reorder).
- [ ] Error messages mention the offending field.
- [ ] Errors wrap `ErrInvalid`.
- [ ] `Value()` correctly computes PriceCents × Quantity.
- [ ] `LowStock()` returns true if Quantity <= ReorderLevel.
- [ ] `String()` formats correctly: `` `SKU "Name" qty=N @ $X.YY` ``.
- [ ] `FormatCents` handles all int64 values (including negative and min/max).

**Store Interface & MemoryStore:**
- [ ] All methods are implemented.
- [ ] Store operations are concurrent-safe (use `sync.Mutex`).
- [ ] `List()` returns products sorted by SKU in a newly allocated slice.
- [ ] `Get()` and other methods return copies, never references to internal state.
- [ ] Errors wrap the appropriate sentinel error (`ErrDuplicate`, `ErrNotFound`).
- [ ] Compile-time assertion proves `*MemoryStore` implements `Store`.

**Alerter & AlerterFunc:**
- [ ] `AlerterFunc` is a function type.
- [ ] `LowStock` method on AlerterFunc calls the underlying function.
- [ ] Alerter may be nil (checked with `if inv.alerter != nil`).

**Inventory Service:**
- [ ] All methods are implemented.
- [ ] `Sell` and `Receive` are atomic (use `sync.Mutex`).
- [ ] `Sell` validates qty > 0.
- [ ] `Sell` checks for insufficient stock before modifying.
- [ ] `Sell` alerts only on transition from not-low to low stock (not always, not never).
- [ ] `Receive` validates qty > 0.
- [ ] `Receive` never triggers an alert.
- [ ] `Filter` returns products sorted by SKU.
- [ ] `LowStock` is implemented as a one-liner using `Filter`.
- [ ] `TotalValue` sums the value of all products.
- [ ] `Sorted` produces deterministic output (ties broken by SKU).
- [ ] `Sorted` returns an error for unknown sort keys.
- [ ] `WriteReport` produces fixed-width, aligned output.
- [ ] `WriteReport` includes the TOTAL line.
- [ ] `WriteReport` includes the "low stock" line.

**Error Handling:**
- [ ] All errors wrap sentinel errors with `%w`.
- [ ] Error messages mention the context (e.g., SKU, field name).
- [ ] No double-logging (errors are returned, not logged and returned).

**Testing:**
- [ ] All tests pass.
- [ ] Tests are run with `-race` (no data races).
- [ ] Tests cover edge cases (empty store, single product, ties in sorting).

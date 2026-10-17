# Structs, Methods & Interfaces

## Goals

By the end of this module you'll understand:
- How to design struct types and methods that make sense together.
- The difference between value and pointer receivers, and why it matters.
- Go's implicit interfaces: what they are, how they're used, and why they're powerful.
- How to embed types and avoid the pitfalls of promotion.
- The pattern of compile-time assertions to catch interface violations early.
- How to use interfaces as contracts in your API and how to keep them small.
- Type assertions and type switches for inspecting interface values at runtime.
- Common gotchas like nil receivers and nil interface values.
- How to design with constructors and functional options for clean APIs.

## Concepts

### Structs and Fields

A struct is a sequence of named fields with their types. You create one like this:

```go
type Person struct {
	Name string
	Age  int
	City string
}

p := Person{Name: "Alice", Age: 30, City: "NYC"}

q := p // a copy: structs are values
q.Age = 31
fmt.Println(p.Age, q.Age) // 30 31
```

Structs are values. When you assign a struct to a variable or pass it to a function, Go copies all its fields. If the struct is large (e.g., a thousand-element array), that copy can be expensive—a sign that you might want a pointer instead.

Struct fields can be exported (capitalized, visible outside the package) or unexported (lowercase, private to the package). A common pattern is to hide the implementation details (use unexported fields) and expose only what's needed through methods (see below).

```go
// Good: hides implementation, exposes behavior
type Stack struct {
	items []int // unexported
}

func (s *Stack) Push(v int) { s.items = append(s.items, v) }
func (s *Stack) Len() int   { return len(s.items) }

var s Stack
s.Push(1)
s.Push(2)
fmt.Println(s.Len()) // 2: callers use the methods, never s.items directly
```

### Methods and Receivers

A method is a function attached to a type. It has a *receiver*—a special parameter between `func` and the method name:

```go
type Stack struct{ items []int }

func (s *Stack) Push(v int) {
	s.items = append(s.items, v)
}

var s Stack
s.Push(7) // Go takes &s for you because s is addressable
fmt.Println(s.items) // [7]
```

The receiver can be any named type (struct, int, string), not just pointers. The question is: should it be a value or a pointer?

#### Value Receivers vs Pointer Receivers

**Value receiver**: the method receives a copy of the value. Changes inside the method don't affect the original.

```go
type Point struct {
	X, Y float64
}

// Value receiver: changes are lost
func (p Point) Add(dx, dy float64) {
	p.X += dx // only affects the copy
	p.Y += dy
}

point := Point{1, 2}
point.Add(3, 4)
fmt.Println(point) // still {1 2}
```

**Pointer receiver**: the method receives a pointer to the original value. Changes affect the original.

```go
type Point struct {
	X, Y float64
}

// Pointer receiver: changes persist
func (p *Point) Add(dx, dy float64) {
	p.X += dx // affects the original
	p.Y += dy
}

point := Point{1, 2}
point.Add(3, 4)
fmt.Println(point) // {4 6}
```

**When to use each** (from [Effective Go](https://go.dev/doc/effective_go#pointers_vs_values)):

- **Pointer receiver** when the method mutates the receiver (including building state through repeated calls).
- **Pointer receiver** when the struct contains a `sync.Mutex` or another type with no-copy semantics (e.g., `sync.WaitGroup`).
- **Pointer receiver** when the struct is large (e.g., thousands of fields or a large array); copying is expensive.
- **Value receiver** for small, immutable types: `time.Time`, slices, maps, functions, channels, and built-in numeric types.
- **Don't mix**: if any method of a type needs a pointer receiver, give *all* of them pointer receivers. Consistency matters, and it avoids confusion about which methods modify the value.

Violating the "don't mix" rule creates subtle bugs. A common pitfall:

```go
type Stack struct {
	items []int
}

// BUG: value receiver on a mutating method
func (s Stack) Push(v int) {
	s.items = append(s.items, v) // mutation lost
}

stack := Stack{}
stack.Push(1)
fmt.Println(stack.items) // empty—push didn't work!
```

The fix is simple: use a pointer receiver.

```go
type Stack struct {
	items []int
}

func (s *Stack) Push(v int) {
	s.items = append(s.items, v)
}

stack := Stack{}
stack.Push(1)
fmt.Println(stack.items) // [1]
```

#### Method Sets

The method set of a type `T` is the set of methods with a *value* receiver. The method set of `*T` is the set of *all* methods: both those with value receivers and those with pointer receivers.

```go
type Circle struct {
	Radius float64
}

// value receiver
func (c Circle) Area() float64 { return math.Pi * c.Radius * c.Radius }

// pointer receiver
func (c *Circle) SetRadius(r float64) { c.Radius = r }

c := Circle{Radius: 1}
c.SetRadius(2) // c is addressable, so Go calls (&c).SetRadius(2)
fmt.Printf("%.2f\n", c.Area()) // 12.57
```

The method set of `Circle` contains only `Area()`. The method set of `*Circle` contains both `Area()` and `SetRadius()`.

This matters for interfaces (see below). If an interface requires both `Area()` and `SetRadius()`, then:

- A `*Circle` can satisfy it (it has all methods).
- A `Circle` value cannot (it doesn't have `SetRadius()`), though `&Circle` could.

This is why compile-time assertions (coming up) are valuable: they catch mismatches early.

### Interfaces

An interface defines a set of methods. Any type that implements those methods implicitly satisfies the interface—no explicit "implements" keyword needed.

```go
// Reader is an interface; anyone with a Read method implements it.
type Reader interface {
	Read(p []byte) (n int, err error)
}

// *os.File has a Read method, so *os.File is a Reader.
// *bytes.Buffer has a Read method, so *bytes.Buffer is a Reader.

var _ Reader = (*os.File)(nil)
var _ Reader = (*bytes.Buffer)(nil)

var r Reader = strings.NewReader("so is *strings.Reader")
b, _ := io.ReadAll(r)
fmt.Println(string(b))
```

Because interfaces are implicit, you can define them *where they are used*, not next to the implementation. This decouples your code and makes it more flexible.

```go
// Bad: interface defined with the implementation (package userdb)
type User struct{ Name string }

type UserStorer interface {
	GetUser(id int) (*User, error)
}

type PostgresDB struct{ users map[int]*User }

func (p *PostgresDB) GetUser(id int) (*User, error) {
	u, ok := p.users[id]
	if !ok {
		return nil, fmt.Errorf("user %d not found", id)
	}
	return u, nil
}

// The producer now dictates the abstraction to every consumer.
var store UserStorer = &PostgresDB{users: map[int]*User{1: {"Ada"}}}
u, _ := store.GetUser(1)
fmt.Println(u.Name)
```

```go
// Good: interface defined by the consumer (package service)
type User struct{ Name string }

type UserStorer interface {
	GetUser(id int) (*User, error)
}

type Service struct{}

func (s *Service) FindUser(id int, store UserStorer) (*User, error) {
	return store.GetUser(id)
}

// A test fake satisfies it without importing anything from service.
type fakeStore struct{}

func (fakeStore) GetUser(id int) (*User, error) { return &User{Name: "Test User"}, nil }

u, _ := (&Service{}).FindUser(1, fakeStore{})
fmt.Println(u.Name) // Test User
```

The second approach means a new implementation (e.g., a mock for testing) doesn't need to import the service package or even know it exists. It just implements the methods it cares about.

**Keep interfaces small.** The Go philosophy: "The bigger the interface, the weaker the abstraction." Small interfaces are easier to implement, compose, and test.

```go
// Good: focused interfaces (the same shape as io.Reader and io.Writer)
type Reader interface {
	Read(p []byte) (n int, err error)
}

type Writer interface {
	Write(p []byte) (n int, err error)
}

// Each parameter asks for only the method it uses.
func Copy(dst Writer, src Reader) error {
	_, err := io.Copy(dst, src)
	return err
}

err := Copy(os.Stdout, strings.NewReader("hello from a Reader\n"))
fmt.Println(err) // <nil>
```

Avoid huge interfaces, especially ones with dozens of methods. If your interface has 10+ methods, consider breaking it into smaller pieces.

#### Implicit Satisfaction

Unlike languages with explicit `implements` keywords, Go types satisfy interfaces automatically if they have all the required methods. This is powerful but can be surprising:

```go
type Greeter interface {
	Greet() string
}

type Robot struct{}

func (r Robot) Greet() string { return "beep boop" }

var g Greeter = Robot{} // OK: Robot implements Greeter

fmt.Println(g.Greet()) // beep boop
```

There's no "Greeter" declaration in the Robot type; Go checks at compile time whether the method set covers the interface. This makes it easy to retrofit code with interfaces.

#### The `Stringer` Interface

Many standard library functions (like `fmt.Println`, `fmt.Sprintf`) use the `fmt.Stringer` interface:

```go
type Stringer interface {
	String() string
}

type Celsius float64

func (c Celsius) String() string { return fmt.Sprintf("%.1f°C", float64(c)) }

var s Stringer = Celsius(21.5)
fmt.Println(s.String(), Celsius(30)) // 21.5°C 30.0°C: fmt finds the same method
```

If your type has a `String()` method, `fmt.Println` calls it automatically. This is how you customize printing:

```go
type Color int

const (
	Red Color = iota
	Green
	Blue
)

func (c Color) String() string {
	switch c {
	case Red:
		return "red"
	case Green:
		return "green"
	case Blue:
		return "blue"
	default:
		return "unknown"
	}
}

fmt.Println(Red) // Output: red
```

**A gotcha**: if `String()` has a pointer receiver, then printing a *value* won't call it (a value's method set has no pointer-receiver methods):

```go
type Color int

const Red Color = 0

func (c *Color) String() string { return "red" }

c := Red
fmt.Println(c)  // 0: a Color value has no String method; only *Color does
fmt.Println(&c) // red: the pointer has it
```

**Another gotcha**: infinite recursion. This pattern causes a stack overflow:

```go expect=runtime-error
type Color int

func (c Color) String() string {
	return fmt.Sprintf("%v", c) // calls String again! (go vet warns about this)
}

// Fix: convert to the underlying type
//
//	func (c Color) String() string {
//		return fmt.Sprintf("%v", int(c))
//	}

fmt.Println(Color(1)) // fatal error: stack overflow
```

### Embedding and Composition

Go doesn't have inheritance, but it does have *embedding*: a struct can include another struct (or interface) as a field without a name. The embedded type's methods are *promoted* to the embedding type.

```go
type Animal struct {
	Name string
}

func (a Animal) Speak() string { return a.Name + " makes a sound" }

type Dog struct {
	Animal          // embedded: no field name
	Breed  string
}

dog := Dog{
	Animal: Animal{Name: "Buddy"},
	Breed:  "Golden Retriever",
}

fmt.Println(dog.Speak()) // "Buddy makes a sound"
```

The `Speak()` method is promoted to `Dog`, so you can call it directly. But the receiver of the promoted method is the embedded value, not the outer struct. There is **no virtual dispatch back to the outer type**:

```go
type Animal struct {
	Name string
}

func (a Animal) Speak() string     { return a.Name + " makes a sound" }
func (a Animal) Introduce() string { return "I'm " + a.Name + ". " + a.Speak() }

type Dog struct {
	Animal
	Breed string
}

func (d Dog) Speak() string { return d.Name + " barks" }

dog := Dog{Animal: Animal{Name: "Buddy"}}
fmt.Println(dog.Speak()) // Buddy barks (Dog.Speak hides Animal.Speak)

// Introduce is promoted from Animal, and inside it the receiver is the
// Animal, so a.Speak() is Animal.Speak, never Dog.Speak:
fmt.Println(dog.Introduce()) // I'm Buddy. Buddy makes a sound
```

This isn't inheritance; it's just method promotion. The embedded type's receiver is always the embedded value.

**Common pitfall: exported struct embedding**. Embedding an unexported type in an exported struct leaks that type's API. This couples your code to the embedded type and makes it harder to refactor:

```go
type DB struct{} // stands in for *sql.DB

func (*DB) Query(q string) string { return "rows for " + q }
func (*DB) Close() error          { return nil }

// Bad: client code can now access DB's fields and methods directly
type LeakyService struct {
	*DB
}

// Good: explicit composition
type DatabaseService struct {
	db *DB
}

func (s *DatabaseService) Query(query string) string { return s.db.Query(query) }

leaky := LeakyService{&DB{}}
leaky.Close() // anyone can close your connection pool
good := DatabaseService{&DB{}}
fmt.Println(good.Query("users")) // only the methods you chose
```

Another pitfall: **embedding a Mutex in an exported struct**. The Mutex's `Lock()` and `Unlock()` methods are promoted to the struct, which defeats the purpose of hiding synchronization:

```go
// Bad: Lock and Unlock are exposed
type LeakyCache struct {
	sync.Mutex
	items map[string]string
}

leaky := LeakyCache{}
leaky.Lock() // public API—not what you want
leaky.Unlock()

// Good: unexported named field
type Cache struct {
	mu    sync.Mutex
	items map[string]string
}

func (c *Cache) Get(key string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.items[key]
}

cache := &Cache{items: map[string]string{"k": "v"}}
fmt.Println(cache.Get("k")) // v
```

**Embedding interfaces** in other interfaces is idiomatic and useful:

```go
type ReadCloser interface {
	io.Reader
	io.Closer
}

// equivalent to:
type ReadCloserSpelledOut interface {
	Read(p []byte) (n int, err error)
	Close() error
}

var rc ReadCloser = io.NopCloser(strings.NewReader("x"))
var rc2 ReadCloserSpelledOut = rc // same method set, so this assignment compiles
fmt.Println(rc2.Close())          // <nil>
```

### Type Assertions and Type Switches

At runtime, an interface value holds both a dynamic type and a dynamic value. You can inspect and extract them with **type assertions**:

```go
var r io.Reader = os.Stdin

// Assert that r is a *os.File
f, ok := r.(*os.File)
if ok {
	fmt.Println("It's a file:", f.Name())
} else {
	fmt.Println("Not a file")
}
```

Always use the comma-ok form (`v, ok := expr.(T)`); a bare assertion panics if the type doesn't match:

```go expect=runtime-error
var r io.Reader = strings.NewReader("not a file")
f := r.(*os.File) // panics: interface conversion: io.Reader is *strings.Reader, not *os.File
fmt.Println(f)
```

For multiple possible types, use a **type switch**:

```go
func describe(v interface{}) {
	switch v := v.(type) {
	case string:
		fmt.Printf("string: %q\n", v)
	case int:
		fmt.Printf("int: %d\n", v)
	case fmt.Stringer:
		fmt.Printf("Stringer: %s\n", v.String())
	default:
		fmt.Printf("unknown: %T\n", v)
	}
}

describe("go")        // string: "go"
describe(42)          // int: 42
describe(time.Second) // Stringer: 1s
describe(3.14)        // unknown: float64
```

The type switch unwraps the dynamic type for each case. This is cleaner than repeated assertions.

**Important: wrapped errors.** If an error is wrapped with `%w`, the dynamic type of the error interface is the outer wrapper, not the underlying error. Type switches on the outer error won't catch the inner type:

```go
type MyError struct{ msg string }
func (e *MyError) Error() string { return e.msg }

err := fmt.Errorf("operation failed: %w", &MyError{"inner"})

// Type switch checks the outer *fmt.wrapError, not *MyError:
switch err.(type) {
case *MyError:
	fmt.Println("caught inner error")  // this won't run
default:
	fmt.Println("outer error")  // this runs
}

// Use errors.Is or errors.As to inspect wrapped errors:
if errors.Is(err, &MyError{}) {
	// doesn't work—Is uses == comparison
}

var me *MyError
if errors.As(err, &me) {
	fmt.Println("unwrapped:", me.msg)  // this works
}
```

### The Nil Interface Value Gotcha

An interface value is nil only if both its dynamic type and dynamic value are nil. A common mistake:

```go
type MyError struct{ msg string }

func (e *MyError) Error() string { return e.msg }

func doSomething() error {
	var e *MyError // nil pointer
	return e       // returns interface holding (type=*MyError, value=nil)
}

err := doSomething()
if err != nil { // true! The interface is not nil.
	fmt.Printf("err != nil, and it holds %T\n", err) // err != nil, and it holds *main.MyError
}
```

The interface value is *not* nil because it holds a type (`*MyError`). This trips up beginners constantly. The fix is to return `nil` explicitly:

```go
func doSomething(ok bool) error {
	if ok {
		return nil // explicit nil
	}
	return fmt.Errorf("something went wrong")
}

fmt.Println(doSomething(true) == nil) // true
```

Or, design your error types so the zero value is never valid:

```go
type MyError struct {
	Code    int
	Message string
}

func (e *MyError) Error() string {
	return fmt.Sprintf("code %d: %s", e.Code, e.Message)
}

// Zero value MyError{} is not meaningful, so it's unlikely to escape.

var err error = &MyError{Code: 404, Message: "not found"}
fmt.Println(err) // code 404: not found
```

### Compile-Time Assertions

You can assert at compile time that a type implements an interface using a blank identifier:

```go
type MyType struct{}

func (*MyType) Read(p []byte) (int, error) { return 0, io.EOF }

type MyValue struct{}

func (MyValue) String() string { return "my value" }

var _ io.Reader = (*MyType)(nil)
var _ fmt.Stringer = MyValue{}

fmt.Println(MyValue{}) // compiles, so both assertions hold: my value
```

The compiler checks whether `*MyType` has all the methods of `io.Reader`. If not, the assignment fails and the code won't compile. This is a cheap way to catch interface violations early. Here `ConsoleLogger` forgot `Logf`; run it to read the compiler's error:

```go expect=compile-error
type Logger interface {
	Log(msg string)
	Logf(format string, args ...any)
}

type ConsoleLogger struct{}

func (l *ConsoleLogger) Log(msg string) { fmt.Println(msg) }

// Missing Logf: this line fails to compile, naming the missing method
var _ Logger = (*ConsoleLogger)(nil)
```

Add the missing method and the assertion compiles. Note that the assertion uses `(*ConsoleLogger)(nil)`: the methods have pointer receivers, so only `*ConsoleLogger` is a `Logger`. `var _ Logger = ConsoleLogger{}` would still fail.

```go
type Logger interface {
	Log(msg string)
	Logf(format string, args ...any)
}

type ConsoleLogger struct{}

func (l *ConsoleLogger) Log(msg string)                 { fmt.Println(msg) }
func (l *ConsoleLogger) Logf(format string, args ...any) { fmt.Printf(format+"\n", args...) }

// Now this succeeds
var _ Logger = (*ConsoleLogger)(nil)

var log Logger = &ConsoleLogger{}
log.Logf("%d methods, all present", 2)
```

### Constructors and the "New" Convention

For types with unexported fields or complex setup, define a constructor function:

```go
type Server struct {
	host string
	port int
}

func NewServer(host string, port int) *Server {
	return &Server{host: host, port: port}
}

s := NewServer("localhost", 8080)
fmt.Printf("%+v\n", *s) // {host:localhost port:8080}
```

The constructor is typically named `New<TypeName>`. It should validate inputs and return a properly initialized value. For errors, return a tuple:

```go
type Server struct {
	host string
	port int
}

func NewServer(host string, port int) (*Server, error) {
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("invalid port %d", port)
	}
	return &Server{host: host, port: port}, nil
}

_, err := NewServer("localhost", 70000)
fmt.Println(err) // invalid port 70000
```

### Functional Options Pattern

When a type has many optional parameters, the functional options pattern is idiomatic:

```go
type Server struct {
	host         string
	port         int
	readTimeout  time.Duration
	writeTimeout time.Duration
	maxConns     int
}

// Option is a function that modifies a Server
type Option func(*Server) error

func WithPort(port int) Option {
	return func(s *Server) error {
		if port < 1 || port > 65535 {
			return fmt.Errorf("invalid port %d", port)
		}
		s.port = port
		return nil
	}
}

func WithReadTimeout(d time.Duration) Option {
	return func(s *Server) error {
		if d <= 0 {
			return errors.New("timeout must be positive")
		}
		s.readTimeout = d
		return nil
	}
}

// New applies defaults and then the options in order
func New(host string, opts ...Option) (*Server, error) {
	s := &Server{
		host:         host,
		port:         8080,
		readTimeout:  5 * time.Second,
		writeTimeout: 10 * time.Second,
		maxConns:     100,
	}
	for _, opt := range opts {
		if opt == nil { // skip nil options
			continue
		}
		if err := opt(s); err != nil {
			return nil, fmt.Errorf("invalid option: %w", err)
		}
	}
	return s, nil
}

// Usage
srv, err := New("localhost", WithPort(9000), WithReadTimeout(3*time.Second))

fmt.Println(srv.port, srv.readTimeout, srv.writeTimeout, srv.maxConns, err) // 9000 3s 10s 100 <nil>: options applied, defaults kept

_, err = New("localhost", WithPort(70000))
fmt.Println(err) // invalid option: invalid port 70000
```

This pattern gives you:
- Fluent, readable APIs: `New(host, WithPort(...), WithTimeout(...))`.
- Extensibility: add new options without breaking existing code (no function signature changes).
- Validation: each option can report errors.
- Optional parameters: skip any options you don't need (no positional `nil` arguments).

### Sorting: `slices.SortFunc` vs `sort.Slice`

In Go 1.21+, use `slices.SortFunc` (from the `slices` package) to sort with a custom comparison function:

```go
type Product struct {
	Name  string
	Price int
}

products := []Product{{"pen", 3}, {"book", 12}, {"mug", 8}}

// Sort by price, ascending
slices.SortFunc(products, func(a, b Product) int {
	return cmp.Compare(a.Price, b.Price)
})
fmt.Println(products) // [{pen 3} {mug 8} {book 12}]
```

The comparison function takes two elements and returns:
- A negative number if the first should come before the second.
- 0 if they're equal.
- A positive number if the first should come after the second.

The `cmp.Compare` function (from package `cmp`, Go 1.21+) implements this for ordered types.

For descending order, negate the result:

```go
type Product struct {
	Name  string
	Price int
}

products := []Product{{"pen", 3}, {"book", 12}, {"mug", 8}}
slices.SortFunc(products, func(a, b Product) int {
	return -cmp.Compare(a.Price, b.Price) // or cmp.Compare(b.Price, a.Price)
})
fmt.Println(products) // [{book 12} {mug 8} {pen 3}]
```

For stable sorting (preserving the relative order of equal elements), use `slices.SortStableFunc`.

The older `sort.Slice` takes a `less(i, j int) bool` func over indices and works through reflection, and `sort.Sort` needs a type with `Len`, `Less`, and `Swap` methods. Both are more verbose and slower. Prefer `slices.SortFunc`.

## Idioms & Best Practices

- **Accept interfaces, return concrete types:** functions should accept interface parameters (for flexibility) but return concrete types (so callers can call more methods without a type assertion). The exception is `error`.
  ```go
  // Good: accepts any io.Reader, returns a concrete []byte
  func ReadAll(r io.Reader) ([]byte, error) { return io.ReadAll(r) }

  // Bad: takes a filename (can't pass a buffer in a test) and returns an
  // interface (callers can only call Read).
  func OpenFile(filename string) io.Reader {
  	f, _ := os.Open(filename)
  	return f
  }

  b, _ := ReadAll(strings.NewReader("abc"))
  fmt.Println(b[0], len(b)) // 97 3: callers can index it
  _ = OpenFile
  ```

- **Avoid interface{} (any):** use `any` (a synonym for `interface{}`) sparingly. It says nothing and often hides bugs. Prefer concrete types or concrete interfaces.
  ```go
  // Bad: anything goes; mistakes surface at run time
  func ProcessAny(v any) {
  	if r, ok := v.(io.Reader); ok {
  		io.Copy(io.Discard, r)
  	}
  }

  // Better: the compiler checks callers
  func Process(r io.Reader) {
  	n, _ := io.Copy(io.Discard, r)
  	fmt.Println("processed", n, "bytes")
  }

  ProcessAny(42) // compiles, silently does nothing
  Process(strings.NewReader("hello"))
  ```

- **Don't use pointers to interfaces:** it's almost never the right thing. Interfaces are already reference types; a pointer to an interface is usually a mistake.
  ```go
  // Bad: a pointer to an interface
  func SetupPtr(w *io.Writer) { fmt.Fprintln(*w, "setup (ptr)") }

  // Good: the interface value already refers to the writer
  func Setup(w io.Writer) { fmt.Fprintln(w, "setup") }

  var w io.Writer = os.Stdout
  SetupPtr(&w) // awkward at every call site
  Setup(os.Stdout)
  ```

- **Consistent receiver types:** if any method of a type uses a pointer receiver, all should. This prevents confusion and matches Go conventions.
  ```go
  type Cache struct {
  	mu    sync.Mutex
  	items map[string]string
  }

  // All pointer receivers
  func (c *Cache) Get(key string) string {
  	c.mu.Lock()
  	defer c.mu.Unlock()
  	return c.items[key]
  }

  func (c *Cache) Set(key, value string) {
  	c.mu.Lock()
  	defer c.mu.Unlock()
  	if c.items == nil {
  		c.items = make(map[string]string)
  	}
  	c.items[key] = value
  }

  var c Cache
  c.Set("lang", "go")
  fmt.Println(c.Get("lang")) // go
  ```

- **Embed unexported types with care:** if you expose a struct, think twice before embedding another struct. It couples you to that struct's API. Often a named field is cleaner.
  ```go
  type DB struct{} // stands in for *sql.DB

  func (*DB) Exec(q string) {}

  // Bad: exposes DB's methods
  type LeakyUserDB struct{ *DB }

  // Good: hides the implementation
  type UserDB struct{ db *DB }

  func (u *UserDB) GetUser(id int) string { return fmt.Sprint("user ", id) }

  LeakyUserDB{&DB{}}.Exec("DROP TABLE users") // allowed!
  fmt.Println((&UserDB{&DB{}}).GetUser(1))
  ```

- **Use the zero value:** design types so their zero value is useful. `var mu sync.Mutex` works; `var sb strings.Builder` works. If the zero value is broken, the type is hard to use.

- **Validate at the boundary:** constructors and public methods should validate inputs. Unexported types and methods can assume they're called correctly.

- **Copy mutable arguments and results:** if a method takes or returns a slice or map, decide whether callers own it or you do. If you own it, make a copy or document the ownership rules clearly.
  ```go
  type Cache struct{ keys []string }

  // Good: caller is guaranteed not to affect internal state
  func (c *Cache) Keys() []string {
  	return slices.Clone(c.keys) // copy
  }

  c := &Cache{keys: []string{"a", "b"}}
  k := c.Keys()
  k[0] = "changed"
  fmt.Println(c.keys) // [a b]
  ```

## Common Pitfalls

- **Mixing receiver types:** inconsistent receiver types (some value, some pointer) confuse callers and lead to subtle bugs. Choose one style and stick to it.

- **Value receivers on mutating methods:** a value receiver mutates a copy, not the original. If the method should mutate, use a pointer.

- **Returning nil pointers through interface types:** `var e *MyError; return e` returns a non-nil interface because the type is `*MyError`. Always return `nil` explicitly when successful.

- **Forgetting the method set:** `T` and `*T` have different method sets. An interface requiring a pointer-receiver method cannot be satisfied by a value.

- **Embedding types you don't control:** embedding `sql.DB` or `http.Server` exposes their entire API to your users. Use composition with a named field instead.

- **Infinite recursion in `String()`:** the pattern `fmt.Sprintf("%v", m)` inside `String()` recurses forever. Convert to the underlying type: `fmt.Sprintf("%v", int(m))`.

- **Interfaces with a single implementation:** if an interface has only one implementation and is created just for testing, you probably don't need it. Interfaces are for flexibility, not just mocking.

- **Large interfaces:** a 20-method interface is a sign that your design needs rethinking. Break it into smaller pieces.

## Exercises

Each exercise is in `exercises/<pkg>/` with starter code and tests. The tests fail until you implement the functions.

**Run each exercise with:** `go test ./modules/03-types-interfaces/exercises/<pkg>/`

### Exercise 1: shapes

Implement a `Shape` interface with `Area()` and `Perimeter()` methods. Create three types: `Circle`, `Rect` and `Triangle`. Add a `String()` method to each (the `fmt.Stringer` interface) so they print nicely. Implement helper functions `TotalArea`, `Largest`, and `SortByArea`. Add compile-time assertions proving each type implements both `Shape` and `fmt.Stringer`.

**Test command:** `go test ./modules/03-types-interfaces/exercises/shapes/`

### Exercise 2: stack

The starter code defines a `Stack` type with value receivers on mutating methods. This is a bug. Fix the receivers, then implement the missing methods (`Peek`, `Values`, `Clone`, `String`). The `Values()` method should return a copy, not leak the internal slice. The `Clone()` method should return an independent copy. The `String()` method should format the stack as "Stack[1 2 3]".

**Test command:** `go test ./modules/03-types-interfaces/exercises/stack/`

### Exercise 3: describe

Implement a `Describe` function that takes an interface{} value and returns a formatted string describing it using a type switch. Handle many types in priority order:

1. `nil` (untyped) → `"nil"`
2. `error` → `"error: " + v.Error()`  (checked before Stringer!)
3. `fmt.Stringer` → `"Stringer: " + v.String()`
4. `bool` → `"bool true"` or `"bool false"`
5. Integer types (`int`, `int8`, `int16`, `int32`, `int64`, `uint`, `uint8`, `uint16`, `uint32`, `uint64`) → `"<type> <value>"`
6. `float32` and `float64` → `"<type> <value>"` using the shortest representation
7. `string` → `` `string "hello"` `` (using `strconv.Quote` for escaping)
8. `[]byte` → `"[]byte len=N"`
9. `[]any` → `"[elem1, elem2, ...]"` (recursive, comma-separated)
10. `map[string]any` → `"map[key1: val1, key2: val2]"` (sorted by key, recursive)
11. Everything else → `"unknown <type>"` where `<type>` is from `%T`

Also implement a `Validate` function that returns an error with the "nil interface" gotcha in mind: return the untyped `nil` literal, not a nil pointer.

**Test command:** `go test ./modules/03-types-interfaces/exercises/describe/`

### Exercise 4: logging

Design a type that combines (embeds) an unexported `Logger` type. The `Logger` has a `Log` method. Write an `Animal` type that embeds the logger and implements a `Speak()` method that logs its output. Demonstrate method promotion and the receiver rules.

**Test command:** `go test ./modules/03-types-interfaces/exercises/logging/`

### Exercise 5: server

Use the functional options pattern to implement a `Server` configuration type with optional parameters: port, read/write timeouts, and max connections. Implement option functions `WithPort`, `WithTimeouts`, `WithMaxConns`, and `WithTLS`. The `New` function should apply defaults, validate options in order, and return an error if any option is invalid.

**Test command:** `go test ./modules/03-types-interfaces/exercises/server/`

## Project

The **Inventory Manager** project ties everything together. See `project/README.md` for the full spec.

In brief: implement a `Product` type, a `Store` interface with an in-memory implementation, and an `Inventory` service layer that sells, restocks, filters, sorts and reports. Use methods with appropriate receivers, implement the `Stringer` interface, and practice embedding and composition.

**Test command:** `go test ./modules/03-types-interfaces/project/`

## Submitting for Review

When your tests pass, ask your instructor (Claude) to review `modules/03-types-interfaces/`. They'll check:
- Code style (gofmt, naming, doc comments).
- Receiver consistency and correctness.
- Interface design (small, well-scoped).
- Idiomatic Go patterns and error handling.
- Test coverage and clarity.

## Further Reading

- [Effective Go: Interfaces and types](https://go.dev/doc/effective_go#interfaces_and_types)
- [Go Code Review Comments: Interfaces](https://go.dev/wiki/CodeReviewComments#interfaces)
- [Go Code Review Comments: Receiver type](https://go.dev/wiki/CodeReviewComments#receiver-type)
- [Go Code Review Comments: Getters](https://go.dev/wiki/CodeReviewComments#getters-and-setters)
- [Google: Accept interfaces, return concrete types](https://google.github.io/styleguide/go/decisions#accept-interface-return-concrete)
- [Dave Cheney: Functional options for friendly APIs](https://dave.cheney.net/2014/10/17/functional-options-for-friendly-apis)
- [The Go Blog: Methods, Interfaces and Embedded Types](https://go.dev/blog/methods)
- [io package documentation](https://pkg.go.dev/io)
- [fmt.Stringer documentation](https://pkg.go.dev/fmt#Stringer)
- [100 Go Mistakes](https://100go.co) — sections on receivers (#42), interfaces (#5–#8), and embedding (#10).

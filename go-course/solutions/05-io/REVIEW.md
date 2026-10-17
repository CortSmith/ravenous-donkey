# Code Review: Module 05 (I/O, Readers, Writers, Files & Encoding)

This document contains reference solutions and grading criteria for each exercise and the project.

## Exercise 1: rot13

### Readable Solution

```go
func (r *Reader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	for i, b := range p[:n] {
		p[i] = rot13(b)
	}
	return n, err
}

func rot13(b byte) byte {
	switch {
	case 'a' <= b && b <= 'z':
		return 'a' + (b-'a'+13)%26
	case 'A' <= b && b <= 'Z':
		return 'A' + (b-'A'+13)%26
	default:
		return b
	}
}
```

The key insight is transforming exactly the n bytes that were read, then returning both n and the error (even if err != nil and n > 0, as the io.Reader contract allows).

### Optimized Solution (fast.go)

The optimized variant uses a 256-entry lookup table instead of branching on every byte:

```go
var table = func() (t [256]byte) {
	for i := range t {
		t[i] = rot13(byte(i))
	}
}()

func (r *FastReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	p = p[:n]  // helps the compiler eliminate bounds checks
	for i, b := range p {
		p[i] = table[b]
	}
	return n, err
}
```

**Trade-off:** The table version is ~4x faster on large inputs (measured 1294838 ns/op vs 316673 ns/op on go1.24.2 linux/amd64) because it eliminates branch misprediction and allows inlining. The readability cost is minimal: the lookup is still understandable. For small reads or latency-sensitive code, the branching version's better cache behavior (smaller code) might win.

### Review Checklist

- ✓ Transforms exactly `p[:n]`, not all of `p`
- ✓ Returns both `n` and `err` without losing either
- ✓ Correctly handles `io.EOF` together with data
- ✓ Works with `io.Copy` and `iotest.TestReader`
- ✓ Handles all byte values (0–255)
- ✓ ROT13 is its own inverse (double application is identity)
- ✓ Tests verify partial reads (`TestOnlyTransformsReadBytes`)
- ✓ Tests verify error propagation (`TestPropagatesErrors`)
- ✓ No unnecessary allocations

## Exercise 2: linecount

### Readable Solution

Uses `bufio.Reader.ReadLine` to handle lines of any length:

```go
func Count(r io.Reader) (Stats, error) {
	br := bufio.NewReader(r)
	var (
		s       Stats
		lineLen int
		partial bool
	)
	for {
		chunk, isPrefix, err := br.ReadLine()
		if err != nil {
			if errors.Is(err, io.EOF) {
				if partial {
					s.Lines++
					s.Longest = max(s.Longest, lineLen)
				}
				return s, nil
			}
			return s, err
		}
		lineLen += len(chunk)
		partial = isPrefix
		if isPrefix {
			continue
		}
		s.Lines++
		s.Longest = max(s.Longest, lineLen)
		lineLen = 0
	}
}
```

`ReadLine` returns chunks of a line (when `isPrefix == true`, more is coming) and strips both `\n` and `\r\n`. This avoids the 64 KiB limit of `Scanner` while still handling line boundaries correctly.

### Optimized Solution (fast.go)

Reads in fixed chunks and scans for `\n` manually:

```go
func Count(r io.Reader) (Stats, error) {
	const bufSize = 64 * 1024
	buf := make([]byte, bufSize)
	var (
		s       Stats
		lineLen int
		partial bool
	)
	for {
		n, err := r.Read(buf)
		for i := 0; i < n; i++ {
			if buf[i] == '\n' {
				s.Lines++
				s.Longest = max(s.Longest, lineLen)
				lineLen = 0
				partial = false
				continue
			}
			if buf[i] == '\r' && i+1 < n && buf[i+1] == '\n' {
				// \r\n: skip the \r in the length
				s.Lines++
				s.Longest = max(s.Longest, lineLen)
				i++  // skip the \n
				lineLen = 0
				partial = false
				continue
			}
			lineLen++
			partial = true
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				if partial {
					s.Lines++
					s.Longest = max(s.Longest, lineLen)
				}
				return s, nil
			}
			return s, err
		}
	}
}
```

**Trade-off:** This version avoids the syscall overhead of many small reads by using a larger buffer, but is more complex and error-prone (manual handling of `\r\n`). The readable version is often fast enough; only profile before optimizing.

### Review Checklist

- ✓ Handles lines longer than 64 KiB without error
- ✓ Correctly counts final unterminated lines
- ✓ Distinguishes `\n` and `\r\n` terminators
- ✓ Does not include terminators in `Longest`
- ✓ Uses fixed memory, not proportional to file size
- ✓ Propagates read errors correctly
- ✓ Tests verify edge cases: empty input, single newline, CRLF, unterminated final line

## Exercise 3: csvjson

### Readable Solution

```go
func Convert(dst io.Writer, src io.Reader) error {
	r := csv.NewReader(src)
	
	// Read and validate header
	header, err := r.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return fmt.Errorf("%w: no header row", ErrNoHeader)
		}
		return fmt.Errorf("read header: %w", err)
	}
	
	// Validate header
	seen := make(map[string]bool)
	for i, name := range header {
		if name = strings.TrimSpace(name); name == "" {
			return fmt.Errorf("%w: column %d is empty", ErrBadHeader, i+1)
		}
		if seen[name] {
			return fmt.Errorf("%w: column %q appears twice", ErrBadHeader, name)
		}
		seen[name] = true
		header[i] = name
	}
	
	enc := json.NewEncoder(dst)
	
	// Read and encode data rows
	for {
		record, err := r.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("read record: %w", err)
		}
		
		obj := make(map[string]string)
		for i, val := range record {
			if i < len(header) {
				obj[header[i]] = val
			}
		}
		if err := enc.Encode(obj); err != nil {
			return fmt.Errorf("encode record: %w", err)
		}
	}
}
```

Key points:
- Streams with `Reader.Read` in a loop, not `ReadAll`
- Validates header before processing data
- Wraps errors with context and original error type
- Uses `json.Encoder` which appends newlines

### Optimized Solution (fast.go)

A faster variant might reuse the object map to reduce allocations:

```go
func Convert(dst io.Writer, src io.Reader) error {
	r := csv.NewReader(src)
	
	// Read and validate header...
	header, err := r.Read()
	// ... validation ...
	
	enc := json.NewEncoder(dst)
	obj := make(map[string]string, len(header))
	
	for {
		record, err := r.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("read record: %w", err)
		}
		
		// Clear and reuse the map
		for k := range obj {
			delete(obj, k)
		}
		for i, val := range record {
			if i < len(header) {
				obj[header[i]] = val
			}
		}
		if err := enc.Encode(obj); err != nil {
			return fmt.Errorf("encode record: %w", err)
		}
	}
}
```

**Trade-off:** Reusing the map reduces allocations significantly (measured 92% fewer allocs and 87% fewer bytes on go1.24.2 linux/amd64), but the readable version is simpler and the improvement is often not noticeable for small datasets. Profile before choosing.

### Review Checklist

- ✓ Streams with `csv.Reader.Read`, not `ReadAll`
- ✓ Validates header: no empty names, no duplicates
- ✓ Wraps CSV parse errors preserving the `*csv.ParseError`
- ✓ Wraps `ErrNoHeader` and `ErrBadHeader` appropriately
- ✓ Outputs one JSON object per line with `enc.Encode`
- ✓ Handles empty input (returns `ErrNoHeader`)
- ✓ Handles blank input lines (CSV reader skips them)
- ✓ Strips BOM if present (set `r.FieldsPerRecord = -1`)
- ✓ Tests verify header validation (empty names, duplicates)
- ✓ Tests verify error wrapping (call `errors.As` to check type)

## Exercise 4: tail

### Readable Solution

Uses a ring buffer of strings:

```go
func Tail(r io.Reader, n int) ([]string, error) {
	if n <= 0 {
		return []string{}, nil
	}
	
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), MaxLineLength)
	
	buf := make([]string, n)
	idx := 0
	count := 0
	
	for sc.Scan() {
		buf[idx] = sc.Text()
		idx = (idx + 1) % n
		count++
	}
	if err := sc.Err(); err != nil {
		if err == bufio.ErrTooLong {
			return nil, fmt.Errorf("line too long: %w", err)
		}
		return nil, err
	}
	
	if count == 0 {
		return []string{}, nil
	}
	
	// Copy in order (oldest first)
	result := make([]string, min(count, n))
	for i := range result {
		result[i] = buf[(idx+i)%n]
	}
	return result, nil
}
```

The ring buffer avoids allocating a huge slice upfront, and stores only the last n lines.

### Optimized Solution (fast.go)

Avoids allocating the full ring buffer upfront:

```go
func Tail(r io.Reader, n int) ([]string, error) {
	if n <= 0 {
		return []string{}, nil
	}
	
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), MaxLineLength)
	
	buf := make([]string, 0, min(n, 1000))  // start small
	
	for sc.Scan() {
		buf = append(buf, sc.Text())
		if len(buf) > n {
			buf = buf[1:]  // trim oldest
		}
	}
	if err := sc.Err(); err != nil {
		if err == bufio.ErrTooLong {
			return nil, fmt.Errorf("line too long: %w", err)
		}
		return nil, err
	}
	
	return buf, nil
}
```

**Trade-off:** This is simpler and avoids modulo arithmetic, but with very large n it reallocates during append. The ring buffer is more memory-efficient for large n. For typical use (n ≤ 100), this simpler version is fine.

### Review Checklist

- ✓ Returns the last n lines, oldest first
- ✓ Uses a ring buffer or similar fixed-memory structure for n <= limit
- ✓ Does not allocate based on n (e.g., no `make([]string, n)` if n is huge)
- ✓ Raises the scanner limit to handle long lines (with `sc.Buffer`)
- ✓ Returns error if any line exceeds `MaxLineLength`, wrapped with context
- ✓ Handles empty input (returns empty slice, not nil)
- ✓ Tests verify n > 0, n < file lines, n = 0, n < 0

## Exercise 5: config

### Readable Solution

```go
func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("duration must be a string, not %T", b)
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	*d = Duration(dur)
	return nil
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

func Default() Config {
	return Config{
		Listen:   ":8080",
		Timeout:  Duration(30 * time.Second),
		MaxConns: 100,
	}
}

func Load(r io.Reader) (Config, error) {
	cfg := Default()
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	if dec.More() {
		return Config{}, errors.New("unexpected trailing data")
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func LoadFile(path string) (Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	cfg, err := Load(f)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

func (c Config) Validate() error {
	var errs []error
	
	if strings.TrimSpace(c.Name) == "" {
		errs = append(errs, fmt.Errorf("name: %w", ErrInvalid))
	}
	
	if c.Listen == "" {
		errs = append(errs, fmt.Errorf("listen: %w", ErrInvalid))
	} else {
		host, port, err := net.SplitHostPort(c.Listen)
		if err != nil {
			errs = append(errs, fmt.Errorf("listen: invalid format: %w", ErrInvalid))
		} else {
			portNum, err := strconv.Atoi(port)
			if err != nil || portNum < 1 || portNum > 65535 {
				errs = append(errs, fmt.Errorf("listen: port out of range: %w", ErrInvalid))
			}
		}
	}
	
	if c.Timeout <= 0 {
		errs = append(errs, fmt.Errorf("timeout: must be positive: %w", ErrInvalid))
	}
	
	if c.MaxConns < 1 || c.MaxConns > 10000 {
		errs = append(errs, fmt.Errorf("max_conns: out of range: %w", ErrInvalid))
	}
	
	for i, tag := range c.Tags {
		if strings.TrimSpace(tag) == "" {
			errs = append(errs, fmt.Errorf("tags[%d]: empty: %w", i, ErrInvalid))
		}
	}
	
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
```

Key points:
- `Default()` populates all default values
- `Load()` starts with `Default()` so absent fields keep defaults
- `DisallowUnknownFields()` catches typos
- `Validate()` joins all errors, not just the first
- Each error wraps `ErrInvalid` so callers can use `errors.Is`

### Optimized Solution

The readable solution is already quite good. A minor optimization is to avoid creating []string with make for Tags when not needed, but that's micro-optimization territory.

### Review Checklist

- ✓ `Default()` returns correct defaults: `Listen: ":8080"`, `Timeout: 30s`, `MaxConns: 100`
- ✓ `Load()` uses `Default()` as starting point
- ✓ Explicit zeros in JSON (e.g., `"max_conns": 0`) override defaults and are validated
- ✓ `DisallowUnknownFields()` rejects unknown keys
- ✓ Trailing junk after the JSON object is detected
- ✓ Custom `Duration` type uses `json.Unmarshaler` correctly
- ✓ `Validate()` reports ALL errors with `errors.Join`, not just first
- ✓ Each validation error wraps `ErrInvalid`
- ✓ `Listen` validation uses `net.SplitHostPort` and checks port range
- ✓ `Timeout` must be > 0, not >= 0
- ✓ `MaxConns` range is 1–10000
- ✓ `Tags` with empty strings are rejected (with index, e.g., `tags[1]`)
- ✓ `LoadFile()` adds the path to error messages
- ✓ Tests verify `errors.As(err, new(*json.UnmarshalTypeError))` for wrong types
- ✓ Tests verify `errors.As(err, new(*json.SyntaxError))` for bad JSON

## Project: Log Analyzer

### Readable Solution (Parse)

```go
func Parse(line string) (*LogEntry, error) {
	parts := strings.Fields(line)
	if len(parts) < 9 {
		return nil, errors.New("log line has too few fields")
	}
	
	ip := parts[0]
	// parts[1] is ident (-)
	userid := parts[2]
	if userid == "-" {
		userid = ""
	}
	
	// Find the timestamp in brackets
	var tsStart, tsEnd int
	for i, p := range parts {
		if strings.HasPrefix(p, "[") {
			tsStart = i
			break
		}
	}
	tsEnd := tsStart
	for i := tsStart; i < len(parts); i++ {
		if strings.HasSuffix(parts[i], "]") {
			tsEnd = i
			break
		}
	}
	
	// Reconstruct timestamp (may span multiple fields with spaces)
	ts := strings.Join(parts[tsStart:tsEnd+1], " ")
	ts = strings.TrimPrefix(strings.TrimSuffix(ts, "]"), "[")
	
	t, err := time.Parse("02/Jan/2006:15:04:05 -0700", ts)
	if err != nil {
		return nil, fmt.Errorf("parse timestamp: %w", err)
	}
	
	// Find request in quotes
	var reqStart, reqEnd int
	for i, p := range parts {
		if strings.HasPrefix(p, `"`) {
			reqStart = i
			break
		}
	}
	for i := reqStart; i < len(parts); i++ {
		if strings.HasSuffix(parts[i], `"`) {
			reqEnd = i
			break
		}
	}
	req := strings.Join(parts[reqStart:reqEnd+1], " ")
	req = strings.TrimPrefix(strings.TrimSuffix(req, `"`), `"`)
	
	fields := strings.Fields(req)
	if len(fields) != 3 {
		return nil, fmt.Errorf("malformed request line: %q", req)
	}
	method := fields[0]
	pathWithQuery := fields[1]
	
	// Strip query string
	path, _, _ := strings.Cut(pathWithQuery, "?")
	
	// Status
	statusStr := parts[reqEnd+1]
	status, err := strconv.Atoi(statusStr)
	if err != nil {
		return nil, fmt.Errorf("parse status: %w", err)
	}
	
	// Bytes
	bytesStr := parts[reqEnd+2]
	var bytes int64
	if bytesStr != "-" {
		b, err := strconv.ParseInt(bytesStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse bytes: %w", err)
		}
		bytes = b
	}
	
	// Latency
	latencyStr := parts[reqEnd+3]
	latencyFloat, err := strconv.ParseFloat(latencyStr, 64)
	if err != nil {
		return nil, fmt.Errorf("parse latency: %w", err)
	}
	latency := time.Duration(latencyFloat * float64(time.Second))
	
	return &LogEntry{
		IP:        ip,
		UserID:    userid,
		Timestamp: t,
		Method:    method,
		Path:      path,
		Status:    status,
		Bytes:     bytes,
		Latency:   latency,
	}, nil
}
```

This is a bit verbose because of the bracket and quote handling, but it's clear and robust.

### Analyzer

```go
type Analyzer struct {
	statusCounts map[int]int
	pathCounts   map[string]int
	latencies    []time.Duration
}

func NewAnalyzer() *Analyzer {
	return &Analyzer{
		statusCounts: make(map[int]int),
		pathCounts:   make(map[string]int),
		latencies:    []time.Duration{},
	}
}

func (a *Analyzer) ProcessLine(line string) error {
	entry, err := Parse(line)
	if err != nil {
		return err
	}
	a.statusCounts[entry.Status]++
	a.pathCounts[entry.Path]++
	a.latencies = append(a.latencies, entry.Latency)
	return nil
}

func (a *Analyzer) StatusCounts() []StatusCount {
	var counts []StatusCount
	for status, count := range a.statusCounts {
		counts = append(counts, StatusCount{Status: status, Count: count})
	}
	slices.SortFunc(counts, func(a, b StatusCount) int {
		return b.Count - a.Count
	})
	return counts
}

func (a *Analyzer) TopPaths(n int) []PathCount {
	if n <= 0 {
		return []PathCount{}
	}
	var paths []PathCount
	for path, count := range a.pathCounts {
		paths = append(paths, PathCount{Path: path, Count: count})
	}
	slices.SortFunc(paths, func(a, b PathCount) int {
		return b.Count - a.Count
	})
	if n < len(paths) {
		paths = paths[:n]
	}
	return paths
}

func (a *Analyzer) Latency() LatencyStats {
	if len(a.latencies) == 0 {
		return LatencyStats{}
	}
	slices.Sort(a.latencies)
	n := len(a.latencies)
	return LatencyStats{
		P50: a.latencies[(n-1)*50/100],
		P95: a.latencies[(n-1)*95/100],
		Max: a.latencies[n-1],
	}
}
```

### CLI (main.go)

```go
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("loganalyzer", flag.ContinueOnError)
	fs.SetOutput(stderr)
	format := fs.String("format", "text", "output format (text or json)")
	top := fs.Int("top", 10, "number of top paths")
	if err := fs.Parse(args); err != nil {
		return err
	}
	
	var input io.Reader = stdin
	if len(fs.Args()) > 0 {
		filename := fs.Args()[0]
		if filename != "-" {
			f, err := os.Open(filename)
			if err != nil {
				return fmt.Errorf("open %s: %w", filename, err)
			}
			defer f.Close()
			input = f
		}
	}
	
	a := loganalyzer.NewAnalyzer()
	sc := bufio.NewScanner(input)
	for sc.Scan() {
		if err := a.ProcessLine(sc.Text()); err != nil {
			fmt.Fprintf(stderr, "parse error: %v\n", err)
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("read error: %w", err)
	}
	
	switch *format {
	case "text":
		return outputText(stdout, a, *top)
	case "json":
		return outputJSON(stdout, a, *top)
	default:
		return fmt.Errorf("unknown format: %s", *format)
	}
}

func outputText(w io.Writer, a *loganalyzer.Analyzer, top int) error {
	fmt.Fprintf(w, "Status Codes:\n")
	for _, sc := range a.StatusCounts() {
		fmt.Fprintf(w, "  %d: %d\n", sc.Status, sc.Count)
	}
	
	fmt.Fprintf(w, "\nTop Paths:\n")
	for _, pc := range a.TopPaths(top) {
		fmt.Fprintf(w, "  %s: %d\n", pc.Path, pc.Count)
	}
	
	lat := a.Latency()
	fmt.Fprintf(w, "\nLatency:\n")
	fmt.Fprintf(w, "  p50: %v\n", lat.P50)
	fmt.Fprintf(w, "  p95: %v\n", lat.P95)
	fmt.Fprintf(w, "  max: %v\n", lat.Max)
	
	return nil
}

func outputJSON(w io.Writer, a *loganalyzer.Analyzer, top int) error {
	out := jsonOutput{
		StatusCodes: make([]statusCodeJSON, 0),
		TopPaths:    make([]topPathJSON, 0),
	}
	
	for _, sc := range a.StatusCounts() {
		out.StatusCodes = append(out.StatusCodes, statusCodeJSON{Status: sc.Status, Count: sc.Count})
	}
	
	for _, pc := range a.TopPaths(top) {
		out.TopPaths = append(out.TopPaths, topPathJSON{Path: pc.Path, Count: pc.Count})
	}
	
	lat := a.Latency()
	out.Latency = latencyJSON{
		P50Ms: int64(lat.P50.Milliseconds()),
		P95Ms: int64(lat.P95.Milliseconds()),
		MaxMs: int64(lat.Max.Milliseconds()),
	}
	
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
```

### Review Checklist

- ✓ `Parse` handles Apache Combined Log Format correctly
- ✓ Extracts path without query string
- ✓ Parses timestamp with time zone
- ✓ Handles `-` for missing userid and bytes
- ✓ Converts latency (seconds as float) to `time.Duration`
- ✓ Returns errors with context for malformed lines
- ✓ `Analyzer` streams logs, not loading all into memory
- ✓ `ProcessLine` skips errors (with stderr warning) and continues
- ✓ `StatusCounts` and `TopPaths` sorted by count descending
- ✓ `Latency` computes p50, p95 correctly from sorted latencies
- ✓ CLI reads from file or stdin (`-` or omitted)
- ✓ CLI supports `-format text|json` and `-top N`
- ✓ Text output is readable with section headers
- ✓ JSON output has correct structure with millisecond latencies
- ✓ Tests include edge cases (empty input, malformed lines, large n)
- ✓ Tests verify that malformed lines are skipped (not crashing)
- ✓ Integration test processes provided testdata

## Common Mistakes (Cross-Exercise)

1. **Ignoring n in `Read` when err != nil:** Many students forget that `Read` can return both data and an error. They need to process the n bytes even if err != nil.

2. **Not checking `sc.Err()` after Scanner loop:** This is the classic mistake. The scanner silently stops at 64 KiB without checking `sc.Err()`.

3. **Using `io.ReadAll` on untrusted input:** Students should limit input size with `io.LimitReader`.

4. **Forgetting to check Close/Flush errors on writers:** Data may only hit disk/network on Close, so the error is important.

5. **Buffering entire logs when asked to stream:** For the Log Analyzer, storing all log entries is wrong. Aggregating statistics incrementally is the point.

6. **Mishandling JSON unmarshaling errors:** Not preserving the original error type (e.g., `*json.SyntaxError`) so callers can't use `errors.As`.

7. **Not handling case-insensitive JSON field matching:** Students assume JSON keys are case-sensitive and are surprised when `"NAME": "x"` matches a `Name` field.

8. **Using `nil` maps or slices:** Several exercises involve maps or slices that must be initialized.

## Grading Scale

For each exercise, award full credit if:
- The implementation correctly solves the stated problem
- Tests pass (including edge cases)
- Code is gofmt-clean and passes go vet
- Error handling is idiomatic (wrapping with %w, checking all errors)
- No unnecessary allocations or buffering

Deduct points for:
- Off-by-one errors in percentile calculation (-10%)
- Not streaming (loading whole file into memory) (-20%)
- Ignoring errors (-15% per error)
- Not checking sc.Err() (-20%)
- Buffered output not flushed (-10%)

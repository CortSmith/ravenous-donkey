# Project: Text Analyzer

**Time budget:** 45 to 60 minutes.

You will build `textstats`, a package that tokenizes text and computes a few
simple, deterministic statistics (word count, vocabulary size, the most
frequent non-stopwords, the longest word, and the average word length), plus a
small CLI (`cmd/textstats`) that reads files or stdin and prints the report.

This project is deliberately an extension of the module's exercises: tokenizing
is a UTF-8-aware rune walk like `reverse`, counting and ranking words reuses
the `wordfreq` pattern, and the whole package must produce the same output
every time it runs on the same input, which means sorting anything that came
out of a map.

## Getting started

Everything lives in `modules/01-basics/project/`:

| File | What it is |
|---|---|
| `textstats/textstats.go` | Starter code. Types and doc comments are done. Fill in `Tokenize`, `Analyze` and `Report.String`, each marked `TODO`. |
| `textstats/textstats_test.go` | The acceptance tests for the package. Don't edit them. |
| `cmd/textstats/main.go` | Starter code for the CLI. Fill in `run`, marked `TODO`. |
| `cmd/textstats/main_test.go` | The acceptance tests for the CLI. Don't edit them. |

Run the package tests with:

```sh
go test ./modules/01-basics/project/textstats/
```

Run the CLI tests with:

```sh
go test ./modules/01-basics/project/cmd/textstats/
```

Or both together:

```sh
go test ./modules/01-basics/project/...
```

Run a single test while you work:

```sh
go test -run TestAnalyze -v ./modules/01-basics/project/textstats/
```

## Specification

### `Tokenize(text string) []string`

Splits `text` into lowercase words. A word is a maximal run of Unicode letters
and digits (`unicode.IsLetter` / `unicode.IsDigit`). An apostrophe (`'`
U+0027 or `’` U+2019) is part of a word only when it sits directly between two
letters/digits, as in `"don't"` or `"rock'n'roll"`; wherever it is kept, it is
normalized to the plain ASCII `'` in the output. Every other rune — spaces,
punctuation, newlines — separates words and is dropped.

```go
// Runs YOUR Tokenize from modules/01-basics/project/textstats.
import "gocourse/modules/01-basics/project/textstats"

fmt.Printf("%q\n", textstats.Tokenize("The fox's 2nd jump—don't blink!"))
// want: ["the" "fox's" "2nd" "jump" "don't" "blink"]
```

`strings.FieldsFunc` cannot implement the apostrophe rule by itself because
its split function sees one rune at a time and can't look at the *next* one to
decide whether an apostrophe is "inside" a word. Walk the runes yourself
(`[]rune(text)` or two-pass `range`) and accumulate the current word in a
`strings.Builder`.

### `Analyze(text string, opts Options) Report`

Computes the full report:

| Field | Meaning |
|---|---|
| `Words` | total number of words (`len(Tokenize(text))`) |
| `Unique` | number of distinct words |
| `Top` | the `opts.TopN` most frequent words that are **not** in `opts.Stopwords`, ordered by count descending, then alphabetically ascending to break ties. `opts.TopN <= 0` means `nil`. |
| `Longest` | the longest word, counted in **runes**, not bytes; ties go to the alphabetically first |
| `AverageLength` | mean word length in runes; `0` when there are no words |

Stopword matching is case-insensitive (the input words are already lowercase
from `Tokenize`, but `opts.Stopwords` itself may not be). Stopwords still count
towards `Words`, `Unique`, `Longest` and `AverageLength` — they are excluded
**only** from `Top`. `Analyze` must not mutate `opts.Stopwords`.

`Analyze`'s output must depend only on its inputs. Because map iteration order
is randomized, any value you build by ranging over a map (here, `Top`) must go
through a deterministic sort before you return it, exactly like the `wordfreq`
exercise.

### `Report.String() string`

Formats a report as human-readable text:

```
words: 9
unique: 8
longest: jumps
average length: 3.89
top words:
  1. fox (2)
  2. brown (1)
```

When `Top` is empty, the last line reads `top words: none` instead of a
header with nothing under it. Build the text with a `strings.Builder`;
`fmt.Fprintf(&b, ...)` works directly on it because `*strings.Builder`
implements `io.Writer`.

### CLI: `cmd/textstats`

```
textstats [-n N] [-all] [file ...]
```

- `-n int` (default `10`): how many top words to show. Passed straight through
  as `Options.TopN`. A **negative** value is a usage error (exit code `2`);
  the flag package already rejects a non-integer value the same way.
- `-all` (default `false`): include stopwords in `Top` by passing a `nil`
  stopword list to `Analyze` instead of `textstats.DefaultStopwords`.
- Remaining positional arguments are file names, read and joined with a `"\n"`
  between each (so the last word of one file can never merge with the first
  word of the next). With **no** file arguments, all of stdin is read.
- On success, `Report.String()` is written to stdout and `run` returns `0`.
- Any error (a file that can't be read, bad flags) is written to stderr only
  — stdout stays empty — prefixed with `"textstats: "` where the message
  doesn't already name the problem. Return `1` for an input/IO error, `2` for
  a usage error (bad or unknown flag, negative `-n`).

Use `flag.NewFlagSet("textstats", flag.ContinueOnError)` with
`fs.SetOutput(stderr)` so flag-parsing errors go to the right stream and `main`
never calls `os.Exit` directly from inside `run` (that would make `run`
untestable — see `main_test.go`, which calls `run` with in-memory buffers).

## Hints

<details>
<summary>Hint 1: the apostrophe rule</summary>

Convert the text to `[]rune` once up front so you can look one rune ahead.
Track whether the *current* builder already has content and whether the
*next* rune is a letter/digit before deciding an apostrophe belongs to the
current word:

```go
func tokenize(text string) []string {
	var words []string
	var cur strings.Builder
	runes := []rune(text)
	for i, r := range runes {
		switch {
		case isWordRune(r):
			cur.WriteRune(unicode.ToLower(r))
		case isApostrophe(r) && cur.Len() > 0 && i+1 < len(runes) && isWordRune(runes[i+1]):
			cur.WriteByte('\'')
		case cur.Len() > 0:
			words = append(words, cur.String())
			cur.Reset()
		}
	}
	if cur.Len() > 0 {
		words = append(words, cur.String())
	}
	return words
}

func isWordRune(r rune) bool   { return unicode.IsLetter(r) || unicode.IsDigit(r) }
func isApostrophe(r rune) bool { return r == '\'' || r == '’' }

fmt.Printf("%q\n", tokenize("Rock'n'roll isn't 'quoted'"))
// ["rock'n'roll" "isn't" "quoted"]
```

</details>

<details>
<summary>Hint 2: measuring the longest word</summary>

Compare with `utf8.RuneCountInString`, not `len`, or a word with multi-byte
runes will look shorter than it is. You can find `Longest` in the same loop
that builds the frequency map — no need for a second pass.

</details>

<details>
<summary>Hint 3: structuring Analyze</summary>

Split the ranking step into its own helper, e.g.
`topWords(counts map[string]int, stop map[string]struct{}, n int) []WordCount`,
mirroring `TopN` from the `wordfreq` exercise. It keeps `Analyze` itself
readable as "tokenize, count, measure, rank."

</details>

<details>
<summary>Hint 4: wiring the CLI</summary>

```go
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("textstats", flag.ContinueOnError)
	fs.SetOutput(stderr)
	n := fs.Int("n", 10, "number of most frequent words to show")
	all := fs.Bool("all", false, "include stopwords in the top words")
	if err := fs.Parse(args); err != nil {
		return 2 // flag already printed the error and usage to stderr
	}
	fmt.Fprintf(stdout, "n=%d all=%v\n", *n, *all)
	return 0
}

fmt.Println(run([]string{"-n", "3", "-all"}, os.Stdout, os.Stderr)) // n=3 all=true, then 0
fmt.Println(run([]string{"-n", "x"}, os.Stdout, os.Stdout))         // the error and usage, then 2
```

For files, read each with `os.ReadFile` (its error, a `*fs.PathError`, already
names the file — no need to wrap it) and join with `strings.Builder`, writing
a `'\n'` after each file's contents.

</details>

## Acceptance criteria

- [ ] `go test ./modules/01-basics/project/...` passes.
- [ ] `go vet ./modules/01-basics/project/...` is clean and the code is `gofmt`ed.
- [ ] `Tokenize` and `Analyze` never panic on empty input (`""`) or input with
      no word runes at all (e.g. `"... !!! ---"`).
- [ ] `Analyze`'s output is the same across repeated calls on the same input
      (no map-order leakage into `Top`).
- [ ] `Analyze` does not mutate `opts.Stopwords`.
- [ ] `run` never writes to stdout when it returns a non-zero exit code.
- [ ] Every exported identifier has a doc comment that starts with its name.
- [ ] No exported identifiers beyond those already in the starter files.

## Stretch goals

Do these only once the tests pass. Add your own tests for each.

1. **`-format json` flag**: add a flag to the CLI that marshals `Report` with
   `encoding/json` instead of using `String()`. Think about which struct tags
   you need and whether `Top` should be `null` or `[]` when empty.
2. **Line-based stats**: add `Report.Lines` (count of `"\n"`-separated lines)
   without double-counting a trailing newline.
3. **Allocation budget**: write a benchmark for `Analyze` over a few KB of
   text (`testing.AllocsPerRun` or `go test -bench=. -benchmem`) and try to
   cut allocations by reusing a single `[]rune` buffer or presizing the
   frequency map with `make(map[string]int, estimatedUnique)`.
4. **Custom stopword files**: add a `-stopwords file` flag that reads one
   stopword per line and overrides `DefaultStopwords`.

## Submitting

When the tests pass, ask your instructor (Claude) to review `modules/01-basics/`.
